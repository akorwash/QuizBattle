package websockets

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/akorwash/QuizBattle/resources"
	"github.com/gorilla/websocket"
)

type Registry struct {
	mu              sync.Mutex
	gameHubs        map[int64]*Hub
	hubGames        map[*Hub]int64
	hubConnections  map[*Hub]int
	eventsHub       *Hub
	worldChatHub    *Hub
	chatLimiter     *chatRateLimiter
	chatMessages    ChatMessageStore
	voiceLimiter    *chatRateLimiter
	connectionQuota *connectionQuota
	revokedMembers  map[int64]map[int64]time.Time
	revokedSessions map[string]time.Time
	sessionHubs     map[string]map[*Hub]int
	closedGames     map[int64]time.Time
	allowedOrigins  map[string]struct{}
	upgrader        websocket.Upgrader
	lobbyChanges    chan struct{}
	registryDone    chan struct{}
	closeOnce       sync.Once
	lobbyVersion    atomic.Uint64
	validateSession func(string) bool
}

const maxConnectionsPerIdentity = 8

func NewRegistry(allowedOrigins []string, validators ...func(string) bool) *Registry {
	registry := &Registry{
		gameHubs:        make(map[int64]*Hub),
		hubGames:        make(map[*Hub]int64),
		hubConnections:  make(map[*Hub]int),
		eventsHub:       NewHub(),
		worldChatHub:    NewHub(),
		chatLimiter:     newChatRateLimiter(5, time.Second),
		voiceLimiter:    newChatRateLimiter(40, 5*time.Second),
		connectionQuota: newConnectionQuota(maxConnectionsPerIdentity),
		revokedMembers:  make(map[int64]map[int64]time.Time),
		revokedSessions: make(map[string]time.Time),
		sessionHubs:     make(map[string]map[*Hub]int),
		closedGames:     make(map[int64]time.Time),
		allowedOrigins:  make(map[string]struct{}, len(allowedOrigins)),
		lobbyChanges:    make(chan struct{}, 1),
		registryDone:    make(chan struct{}),
	}
	if len(validators) > 0 && validators[0] != nil {
		registry.validateSession = validators[0]
	} else {
		registry.validateSession = func(string) bool { return true }
	}
	for _, origin := range allowedOrigins {
		registry.allowedOrigins[strings.TrimSuffix(origin, "/")] = struct{}{}
	}
	registry.upgrader = websocket.Upgrader{
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
		HandshakeTimeout: 5 * time.Second,
		CheckOrigin:      registry.checkOrigin,
	}
	go registry.eventsHub.Run()
	go registry.worldChatHub.Run()
	go registry.runLobbyNotifier()
	go registry.runTombstoneJanitor()
	return registry
}

func (registry *Registry) PublishGameEvent(event resources.GameEvent) {
	payload, err := json.Marshal(event)
	if err != nil {
		if event.Type == "closed" {
			registry.closeGameHub(event.GameID, nil)
		}
		return
	}
	message := Message{Type: websocket.TextMessage, Data: payload}
	registry.notifyLobbyChange()
	if event.Type == "closed" {
		registry.closeGameHub(event.GameID, &message)
		return
	}
	registry.mu.Lock()
	gameHub := registry.gameHubs[event.GameID]
	registry.mu.Unlock()
	if gameHub != nil {
		gameHub.Broadcast(message)
	}
}

func (registry *Registry) ServeEvents(userID int64, tokenID, fullName string, expiresAt time.Time, w http.ResponseWriter, r *http.Request) {
	if !registry.sessionAllowed(tokenID) {
		http.Error(w, "authentication expired", http.StatusUnauthorized)
		return
	}
	serveClient(
		registry.eventsHub,
		registry.connectionQuota,
		&registry.upgrader,
		userID,
		tokenID,
		fullName,
		expiresAt,
		maxTextMessage,
		maxConnectionsPerUser,
		nil,
		registry.clientRegistrar(registry.eventsHub, 0),
		func() { registry.connectionClosed(tokenID, registry.eventsHub) },
		nil,
		func() bool { return registry.validateSession(tokenID) },
		w,
		r,
	)
}

func (registry *Registry) ServeWorldChat(userID int64, tokenID, username, fullName string, expiresAt time.Time, w http.ResponseWriter, r *http.Request) {
	if !registry.sessionAllowed(tokenID) {
		http.Error(w, "authentication expired", http.StatusUnauthorized)
		return
	}
	registry.mu.Lock()
	chatMessages := registry.chatMessages
	registry.mu.Unlock()
	serveClient(
		registry.worldChatHub,
		registry.connectionQuota,
		&registry.upgrader,
		userID,
		tokenID,
		fullName,
		expiresAt,
		maxTextMessage,
		maxConnectionsPerUser,
		chatHandler(userID, username, fullName, registry.chatLimiter, chatMessages),
		registry.clientRegistrar(registry.worldChatHub, 0),
		func() { registry.connectionClosed(tokenID, registry.worldChatHub) },
		nil,
		func() bool { return registry.validateSession(tokenID) },
		w,
		r,
	)
}

// SetChatMessageStore wires durable chat persistence before the HTTP server
// starts accepting websocket upgrades.
func (registry *Registry) SetChatMessageStore(store ChatMessageStore) {
	registry.mu.Lock()
	registry.chatMessages = store
	registry.mu.Unlock()
}

func (registry *Registry) ServeBattle(gameID, userID int64, tokenID, fullName string, expiresAt time.Time, w http.ResponseWriter, r *http.Request) {
	hub, allowed := registry.gameHubForUser(gameID, userID, tokenID)
	if !allowed {
		http.Error(w, "battle access denied", http.StatusForbidden)
		return
	}
	// Battle state remains server-authoritative. Client-authored frames are
	// restricted to validated, ephemeral WebRTC signaling and never mutate play.
	serveClient(
		hub,
		registry.connectionQuota,
		&registry.upgrader,
		userID,
		tokenID,
		fullName,
		expiresAt,
		maxVoiceMessage,
		1,
		voiceSignalHandler(userID, registry.voiceLimiter),
		registry.clientRegistrar(hub, gameID),
		func() { registry.connectionClosed(tokenID, hub) },
		func() { registry.discardUnusedGameHub(gameID, hub) },
		func() bool { return registry.validateSession(tokenID) },
		w,
		r,
	)
}

func (registry *Registry) DisconnectSession(_ int64, tokenID string, expiresAt time.Time) {
	if tokenID == "" {
		return
	}
	registry.mu.Lock()
	now := time.Now()
	if len(registry.revokedSessions) >= 1000 {
		registry.purgeExpiredSessions(now)
	}
	tombstoneExpiry := now.Add(time.Minute)
	if expiresAt.Before(tombstoneExpiry) {
		tombstoneExpiry = expiresAt
	}
	registry.revokedSessions[tokenID] = tombstoneExpiry
	hubs := make([]*Hub, 0, len(registry.sessionHubs[tokenID]))
	for hub := range registry.sessionHubs[tokenID] {
		hubs = append(hubs, hub)
	}
	registry.mu.Unlock()
	for _, hub := range hubs {
		hub.DisconnectToken(tokenID)
	}
}

func (registry *Registry) clientRegistrar(hub *Hub, gameID int64) func(*Client) bool {
	return func(client *Client) bool {
		registry.mu.Lock()
		defer registry.mu.Unlock()
		now := time.Now()
		if client.ExpiresAt.IsZero() || !client.ExpiresAt.After(now) {
			return false
		}
		if !registry.sessionAllowedLocked(client.TokenID, now) {
			return false
		}
		if gameID != 0 && !registry.battleAllowedLocked(gameID, client.UserID, now) {
			return false
		}
		if !hub.Register(client) {
			return false
		}
		hubs := registry.sessionHubs[client.TokenID]
		if hubs == nil {
			hubs = make(map[*Hub]int)
			registry.sessionHubs[client.TokenID] = hubs
		}
		hubs[hub]++
		registry.hubConnections[hub]++
		return true
	}
}

func (registry *Registry) connectionClosed(tokenID string, hub *Hub) {
	registry.mu.Lock()
	hubs := registry.sessionHubs[tokenID]
	if hubs != nil {
		if hubs[hub] <= 1 {
			delete(hubs, hub)
		} else {
			hubs[hub]--
		}
		if len(hubs) == 0 {
			delete(registry.sessionHubs, tokenID)
		}
	}
	if registry.hubConnections[hub] > 1 {
		registry.hubConnections[hub]--
		registry.mu.Unlock()
		return
	}
	delete(registry.hubConnections, hub)
	gameID, gameHub := registry.hubGames[hub]
	if gameHub && registry.gameHubs[gameID] == hub {
		delete(registry.gameHubs, gameID)
		delete(registry.hubGames, hub)
	}
	registry.mu.Unlock()
	if gameHub {
		hub.Close()
	}
}

func (registry *Registry) discardUnusedGameHub(gameID int64, hub *Hub) {
	registry.mu.Lock()
	if registry.gameHubs[gameID] != hub || registry.hubConnections[hub] != 0 {
		registry.mu.Unlock()
		return
	}
	delete(registry.gameHubs, gameID)
	delete(registry.hubGames, hub)
	registry.mu.Unlock()
	hub.Close()
}

func (registry *Registry) DisconnectBattleUser(gameID, userID int64) {
	registry.mu.Lock()
	now := time.Now()
	users := registry.revokedMembers[gameID]
	if users == nil {
		users = make(map[int64]time.Time)
		registry.revokedMembers[gameID] = users
	}
	if len(users) >= 1000 {
		for id, expiresAt := range users {
			if !expiresAt.After(now) {
				delete(users, id)
			}
		}
	}
	expiresAt := now.Add(time.Minute)
	users[userID] = expiresAt
	hub := registry.gameHubs[gameID]
	registry.mu.Unlock()
	if hub != nil {
		hub.RevokeUser(userID, expiresAt)
	}
}

func (registry *Registry) AllowBattleUser(gameID, userID int64) {
	registry.mu.Lock()
	if users := registry.revokedMembers[gameID]; users != nil {
		delete(users, userID)
		if len(users) == 0 {
			delete(registry.revokedMembers, gameID)
		}
	}
	hub := registry.gameHubs[gameID]
	registry.mu.Unlock()
	if hub != nil {
		hub.GrantUser(userID)
	}
}

func (registry *Registry) DisconnectUser(userID int64) {
	registry.eventsHub.DisconnectUser(userID)
	registry.worldChatHub.DisconnectUser(userID)
	registry.mu.Lock()
	hubs := make([]*Hub, 0, len(registry.gameHubs))
	for _, hub := range registry.gameHubs {
		hubs = append(hubs, hub)
	}
	registry.mu.Unlock()
	for _, hub := range hubs {
		hub.DisconnectUser(userID)
	}
}

func (registry *Registry) Close() {
	registry.closeOnce.Do(func() { close(registry.registryDone) })
	registry.eventsHub.Close()
	registry.worldChatHub.Close()
	registry.mu.Lock()
	defer registry.mu.Unlock()
	for id, hub := range registry.gameHubs {
		hub.Close()
		delete(registry.gameHubs, id)
	}
}

func (registry *Registry) notifyLobbyChange() {
	registry.lobbyVersion.Add(1)
	select {
	case registry.lobbyChanges <- struct{}{}:
	case <-registry.registryDone:
	default:
	}
}

func (registry *Registry) runLobbyNotifier() {
	for {
		select {
		case <-registry.registryDone:
			return
		case <-registry.lobbyChanges:
			timer := time.NewTimer(250 * time.Millisecond)
			select {
			case <-timer.C:
			case <-registry.registryDone:
				if !timer.Stop() {
					<-timer.C
				}
				return
			}
			for {
				select {
				case <-registry.lobbyChanges:
					continue
				default:
					payload, err := json.Marshal(struct {
						Type    string `json:"type"`
						Version uint64 `json:"version"`
					}{Type: "sync", Version: registry.lobbyVersion.Load()})
					if err == nil {
						registry.eventsHub.Broadcast(Message{Type: websocket.TextMessage, Data: payload})
					}
					goto next
				}
			}
		next:
		}
	}
}

func (registry *Registry) runTombstoneJanitor() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-registry.registryDone:
			return
		case now := <-ticker.C:
			registry.mu.Lock()
			registry.purgeExpiredTombstonesLocked(now)
			registry.mu.Unlock()
		}
	}
}

func (registry *Registry) purgeExpiredTombstonesLocked(now time.Time) {
	registry.purgeExpiredSessions(now)
	for gameID, users := range registry.revokedMembers {
		for userID, expiresAt := range users {
			if !expiresAt.After(now) {
				delete(users, userID)
			}
		}
		if len(users) == 0 {
			delete(registry.revokedMembers, gameID)
		}
	}
	for gameID, expiresAt := range registry.closedGames {
		if !expiresAt.After(now) {
			delete(registry.closedGames, gameID)
		}
	}
}

func (registry *Registry) gameHubForUser(gameID, userID int64, tokenID string) (*Hub, bool) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	now := time.Now()
	if !registry.battleAllowedLocked(gameID, userID, now) {
		return nil, false
	}
	if !registry.sessionAllowedLocked(tokenID, now) {
		return nil, false
	}
	if hub, exists := registry.gameHubs[gameID]; exists {
		return hub, true
	}
	hub := NewHub()
	registry.gameHubs[gameID] = hub
	registry.hubGames[hub] = gameID
	go hub.Run()
	return hub, true
}

func (registry *Registry) battleAllowedLocked(gameID, userID int64, now time.Time) bool {
	if expiresAt, closed := registry.closedGames[gameID]; closed {
		if expiresAt.After(now) {
			return false
		}
		delete(registry.closedGames, gameID)
	}
	if users := registry.revokedMembers[gameID]; users != nil {
		if expiresAt, revoked := users[userID]; revoked {
			if expiresAt.After(now) {
				return false
			}
			delete(users, userID)
			if len(users) == 0 {
				delete(registry.revokedMembers, gameID)
			}
		}
	}
	return true
}

func (registry *Registry) sessionAllowed(tokenID string) bool {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	return registry.sessionAllowedLocked(tokenID, time.Now())
}

func (registry *Registry) sessionAllowedLocked(tokenID string, now time.Time) bool {
	if tokenID == "" {
		return false
	}
	expiresAt, revoked := registry.revokedSessions[tokenID]
	if revoked && !expiresAt.After(now) {
		delete(registry.revokedSessions, tokenID)
		return true
	}
	return !revoked
}

func (registry *Registry) purgeExpiredSessions(now time.Time) {
	for tokenID, expiresAt := range registry.revokedSessions {
		if !expiresAt.After(now) {
			delete(registry.revokedSessions, tokenID)
		}
	}
}

func (registry *Registry) closeGameHub(gameID int64, finalMessage *Message) {
	registry.mu.Lock()
	now := time.Now()
	if len(registry.closedGames) >= 1000 {
		for id, expiresAt := range registry.closedGames {
			if !expiresAt.After(now) {
				delete(registry.closedGames, id)
			}
		}
	}
	registry.closedGames[gameID] = now.Add(time.Minute)
	hub := registry.gameHubs[gameID]
	delete(registry.gameHubs, gameID)
	if hub != nil {
		delete(registry.hubGames, hub)
		delete(registry.hubConnections, hub)
	}
	delete(registry.revokedMembers, gameID)
	registry.mu.Unlock()
	if hub != nil {
		if finalMessage != nil {
			hub.CloseWithMessage(*finalMessage)
		} else {
			hub.Close()
		}
	}
}

func (registry *Registry) checkOrigin(request *http.Request) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	if strings.EqualFold(parsed.Host, request.Host) {
		return true
	}
	_, allowed := registry.allowedOrigins[strings.TrimSuffix(origin, "/")]
	return allowed
}
