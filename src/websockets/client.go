package websockets

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/akorwash/QuizBattle/resources"
	"github.com/gorilla/websocket"
)

const (
	writeWait       = 10 * time.Second
	pongWait        = 60 * time.Second
	pingPeriod      = (pongWait * 9) / 10
	maxTextMessage  = 4096
	maxVoiceMessage = 16 * 1024
	authCheckPeriod = 10 * time.Second
)

type Client struct {
	hub           *Hub
	UserID        int64
	TokenID       string
	FullName      string
	ExpiresAt     time.Time
	HubUserLimit  int
	conn          *websocket.Conn
	send          chan Message
	release       func()
	releaseMu     sync.Once
	canceled      atomic.Bool
	sessionActive func() bool
}

type messageHandler func(messageType int, data []byte) (Message, bool)

func (client *Client) readPump(maxMessageSize int64, handler messageHandler) {
	defer func() {
		client.hub.Unregister(client)
		_ = client.conn.Close()
		client.releaseQuota()
	}()
	client.conn.SetReadLimit(maxMessageSize)
	_ = client.conn.SetReadDeadline(client.readDeadline())
	client.conn.SetPongHandler(func(string) error {
		return client.conn.SetReadDeadline(client.readDeadline())
	})
	for {
		messageType, data, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				slog.Debug("websocket read closed", "error", err)
			}
			return
		}
		if client.canceled.Load() {
			return
		}
		if handler == nil {
			// Receive-only channels reject client-authored frames instead of
			// silently consuming an unbounded stream of useless messages.
			return
		}
		message, ok := handler(messageType, data)
		if !ok {
			return
		}
		if client.canceled.Load() {
			return
		}
		client.hub.Broadcast(message)
	}
}

func (client *Client) readDeadline() time.Time {
	deadline := time.Now().Add(pongWait)
	if !client.ExpiresAt.IsZero() && client.ExpiresAt.Before(deadline) {
		return client.ExpiresAt
	}
	return deadline
}

func (client *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	authTicker := time.NewTicker(authCheckPeriod)
	defer func() {
		ticker.Stop()
		authTicker.Stop()
		_ = client.conn.Close()
	}()
	for {
		select {
		case message, ok := <-client.send:
			_ = client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := client.conn.WriteMessage(message.Type, message.Data); err != nil {
				return
			}
		case <-ticker.C:
			_ = client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-authTicker.C:
			if client.sessionActive != nil && !client.sessionActive() {
				client.terminate()
				return
			}
		}
	}
}

func (client *Client) releaseQuota() {
	client.releaseMu.Do(func() {
		if client.release != nil {
			client.release()
		}
	})
}

func (client *Client) terminate() {
	client.canceled.Store(true)
	if client.conn != nil {
		_ = client.conn.Close()
	}
}

type incomingChatMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type outgoingChatMessage struct {
	resources.ChatMessage
	Type string `json:"type"`
}

// ChatMessageStore is the persistence boundary used by the realtime transport.
// Implementations must stamp IDs/timestamps and must never trust client identity.
type ChatMessageStore interface {
	Save(ctx context.Context, userID int64, username, fullName, text string) (*resources.ChatMessage, error)
}

type chatWindow struct {
	started time.Time
	count   int
}

type chatRateLimiter struct {
	mu     sync.Mutex
	users  map[int64]chatWindow
	limit  int
	window time.Duration
}

func newChatRateLimiter(limit int, window time.Duration) *chatRateLimiter {
	return &chatRateLimiter{users: make(map[int64]chatWindow), limit: limit, window: window}
}

func (limiter *chatRateLimiter) allow(userID int64) bool {
	now := time.Now()
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	entry, known := limiter.users[userID]
	if !known && len(limiter.users) >= 10000 {
		for id, candidate := range limiter.users {
			if now.Sub(candidate.started) >= limiter.window {
				delete(limiter.users, id)
			}
		}
		if len(limiter.users) >= 10000 {
			return false
		}
	}
	if entry.started.IsZero() || now.Sub(entry.started) >= limiter.window {
		entry = chatWindow{started: now}
	}
	entry.count++
	limiter.users[userID] = entry
	return entry.count <= limiter.limit
}

func chatHandler(userID int64, username, fullName string, limiter *chatRateLimiter, store ChatMessageStore) messageHandler {
	return func(messageType int, data []byte) (Message, bool) {
		if messageType != websocket.TextMessage {
			return Message{}, false
		}
		if limiter == nil || !limiter.allow(userID) {
			return Message{}, false
		}
		var incoming incomingChatMessage
		if err := json.Unmarshal(data, &incoming); err != nil || incoming.Type != "text" {
			return Message{}, false
		}
		incoming.Message = strings.TrimSpace(incoming.Message)
		if !validChatText(incoming.Message) {
			return Message{}, false
		}
		if store == nil {
			return Message{}, false
		}
		stored, err := store.Save(context.Background(), userID, username, fullName, incoming.Message)
		if err != nil || stored == nil {
			return Message{}, false
		}
		payload, err := json.Marshal(outgoingChatMessage{ChatMessage: *stored, Type: "text"})
		if err != nil {
			return Message{}, false
		}
		return Message{Type: websocket.TextMessage, Data: payload}, true
	}
}

func validChatText(message string) bool {
	if !utf8.ValidString(message) {
		return false
	}
	count := utf8.RuneCountInString(message)
	if count == 0 || count > 500 {
		return false
	}
	for _, char := range message {
		if unicode.IsControl(char) && char != '\n' && char != '\t' {
			return false
		}
	}
	return true
}

type voiceSessionDescription struct {
	Type string `json:"type"`
	SDP  string `json:"sdp"`
}

type voiceICECandidate struct {
	Candidate        string  `json:"candidate"`
	SDPMid           *string `json:"sdpMid,omitempty"`
	SDPMLineIndex    *int    `json:"sdpMLineIndex,omitempty"`
	UsernameFragment *string `json:"usernameFragment,omitempty"`
}

type voiceSignalPayload struct {
	SDP       *voiceSessionDescription `json:"sdp,omitempty"`
	Candidate *voiceICECandidate       `json:"candidate,omitempty"`
}

type incomingVoiceSignal struct {
	Type    string             `json:"type"`
	Payload voiceSignalPayload `json:"payload"`
}

type outgoingVoiceSignal struct {
	FromUserID int64              `json:"fromUserId,string"`
	Type       string             `json:"type"`
	Payload    voiceSignalPayload `json:"payload"`
}

func voiceSignalHandler(userID int64, limiter *chatRateLimiter) messageHandler {
	return func(messageType int, data []byte) (Message, bool) {
		if messageType != websocket.TextMessage || limiter == nil || !limiter.allow(userID) {
			return Message{}, false
		}
		var incoming incomingVoiceSignal
		if err := json.Unmarshal(data, &incoming); err != nil || !validVoiceSignal(incoming) {
			return Message{}, false
		}
		payload, err := json.Marshal(outgoingVoiceSignal{
			FromUserID: userID,
			Type:       incoming.Type,
			Payload:    incoming.Payload,
		})
		if err != nil {
			return Message{}, false
		}
		return Message{ExcludeUserID: userID, Type: websocket.TextMessage, Data: payload}, true
	}
}

func validVoiceSignal(signal incomingVoiceSignal) bool {
	switch signal.Type {
	case "voice_ready", "voice_leave":
		return signal.Payload.SDP == nil && signal.Payload.Candidate == nil
	case "voice_offer", "voice_answer":
		expectedType := strings.TrimPrefix(signal.Type, "voice_")
		return signal.Payload.Candidate == nil &&
			signal.Payload.SDP != nil &&
			signal.Payload.SDP.Type == expectedType &&
			validSignalText(signal.Payload.SDP.SDP, 12*1024, true)
	case "voice_ice":
		candidate := signal.Payload.Candidate
		if signal.Payload.SDP != nil || candidate == nil || !validSignalText(candidate.Candidate, 4096, false) {
			return false
		}
		if candidate.SDPMid != nil && !validSignalText(*candidate.SDPMid, 128, false) {
			return false
		}
		if candidate.SDPMLineIndex != nil && (*candidate.SDPMLineIndex < 0 || *candidate.SDPMLineIndex > 255) {
			return false
		}
		return candidate.UsernameFragment == nil || validSignalText(*candidate.UsernameFragment, 256, false)
	default:
		return false
	}
}

func validSignalText(value string, maximumBytes int, requireContent bool) bool {
	if !utf8.ValidString(value) || len(value) > maximumBytes || (requireContent && value == "") {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) && char != '\r' && char != '\n' && char != '\t' {
			return false
		}
	}
	return true
}

type connectionQuota struct {
	mu    sync.Mutex
	users map[int64]int
	limit int
}

func newConnectionQuota(limit int) *connectionQuota {
	return &connectionQuota{users: make(map[int64]int), limit: limit}
}

func (quota *connectionQuota) acquire(userID int64) bool {
	quota.mu.Lock()
	defer quota.mu.Unlock()
	if userID <= 0 || quota.users[userID] >= quota.limit {
		return false
	}
	quota.users[userID]++
	return true
}

func (quota *connectionQuota) release(userID int64) {
	quota.mu.Lock()
	defer quota.mu.Unlock()
	if quota.users[userID] <= 1 {
		delete(quota.users, userID)
		return
	}
	quota.users[userID]--
}

func serveClient(hub *Hub, quota *connectionQuota, upgrader *websocket.Upgrader, userID int64, tokenID, fullName string, expiresAt time.Time, maxMessageSize int64, hubUserLimit int, handler messageHandler, register func(*Client) bool, onClosed, onRejected func(), sessionActive func() bool, w http.ResponseWriter, r *http.Request) {
	if tokenID == "" || expiresAt.IsZero() || !expiresAt.After(time.Now()) {
		if onRejected != nil {
			onRejected()
		}
		http.Error(w, "authentication expired", http.StatusUnauthorized)
		return
	}
	if quota == nil || !quota.acquire(userID) {
		if onRejected != nil {
			onRejected()
		}
		w.Header().Set("Retry-After", "5")
		http.Error(w, "too many websocket connections", http.StatusTooManyRequests)
		return
	}
	quotaAcquired := true
	defer func() {
		if quotaAcquired {
			quota.release(userID)
		}
	}()
	connection, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if onRejected != nil {
			onRejected()
		}
		slog.Debug("websocket upgrade rejected", "error", err)
		return
	}
	if sessionActive != nil && !sessionActive() {
		_ = connection.Close()
		if onRejected != nil {
			onRejected()
		}
		return
	}
	client := &Client{
		hub:          hub,
		UserID:       userID,
		TokenID:      tokenID,
		FullName:     fullName,
		ExpiresAt:    expiresAt,
		HubUserLimit: hubUserLimit,
		conn:         connection,
		send:         make(chan Message, 64),
		release: func() {
			quota.release(userID)
			if onClosed != nil {
				onClosed()
			}
		},
		sessionActive: sessionActive,
	}
	if register == nil {
		register = hub.Register
	}
	if !register(client) {
		_ = connection.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "websocket connection rejected"),
			time.Now().Add(writeWait),
		)
		_ = connection.Close()
		if onRejected != nil {
			onRejected()
		}
		return
	}
	quotaAcquired = false
	go client.writePump()
	go client.readPump(maxMessageSize, handler)
}
