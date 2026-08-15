package websockets

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Message struct {
	ExcludeUserID int64
	Type          int
	Data          []byte
}

type Hub struct {
	clients         map[*Client]struct{}
	revoked         map[int64]time.Time
	broadcast       chan Message
	register        chan registration
	unregister      chan *Client
	disconnect      chan int64
	disconnectToken chan tokenDisconnection
	revoke          chan membershipChange
	grant           chan membershipChange
	shutdown        chan shutdownRequest
	done            chan struct{}
	closeOnce       sync.Once
}

type registration struct {
	client   *Client
	accepted chan bool
}

type membershipChange struct {
	userID    int64
	expiresAt time.Time
	done      chan struct{}
}

type shutdownRequest struct {
	finalMessage *Message
}

type tokenDisconnection struct {
	tokenID string
	done    chan struct{}
}

const maxConnectionsPerUser = 4

func NewHub() *Hub {
	return &Hub{
		broadcast:       make(chan Message, 256),
		register:        make(chan registration),
		unregister:      make(chan *Client),
		disconnect:      make(chan int64),
		disconnectToken: make(chan tokenDisconnection),
		revoke:          make(chan membershipChange),
		grant:           make(chan membershipChange),
		shutdown:        make(chan shutdownRequest),
		clients:         make(map[*Client]struct{}),
		revoked:         make(map[int64]time.Time),
		done:            make(chan struct{}),
	}
}

func (hub *Hub) Run() {
	janitor := time.NewTicker(30 * time.Second)
	defer janitor.Stop()
	for {
		select {
		case now := <-janitor.C:
			hub.purgeExpiredMemberships(now)
		case request := <-hub.shutdown:
			for client := range hub.clients {
				if request.finalMessage != nil {
					select {
					case client.send <- *request.finalMessage:
					default:
					}
				}
				close(client.send)
				delete(hub.clients, client)
			}
			close(hub.done)
			return
		case request := <-hub.register:
			hub.purgeExpiredMemberships(time.Now())
			connectionLimit := request.client.HubUserLimit
			if connectionLimit <= 0 || connectionLimit > maxConnectionsPerUser {
				connectionLimit = maxConnectionsPerUser
			}
			connectionCount := 0
			for client := range hub.clients {
				if client.UserID == request.client.UserID {
					connectionCount++
				}
			}
			membershipExpiry, revoked := hub.revoked[request.client.UserID]
			if revoked && !membershipExpiry.After(time.Now()) {
				delete(hub.revoked, request.client.UserID)
				revoked = false
			}
			if request.client.UserID <= 0 || request.client.TokenID == "" || revoked || connectionCount >= connectionLimit {
				request.accepted <- false
				continue
			}
			hub.clients[request.client] = struct{}{}
			request.accepted <- true
		case client := <-hub.unregister:
			if _, exists := hub.clients[client]; exists {
				delete(hub.clients, client)
				close(client.send)
			}
		case userID := <-hub.disconnect:
			for client := range hub.clients {
				if client.UserID == userID {
					delete(hub.clients, client)
					client.terminate()
					close(client.send)
				}
			}
		case request := <-hub.disconnectToken:
			for client := range hub.clients {
				if client.TokenID == request.tokenID {
					delete(hub.clients, client)
					client.terminate()
					close(client.send)
				}
			}
			close(request.done)
		case change := <-hub.revoke:
			hub.purgeExpiredMemberships(time.Now())
			hub.revoked[change.userID] = change.expiresAt
			for client := range hub.clients {
				if client.UserID == change.userID {
					delete(hub.clients, client)
					client.terminate()
					close(client.send)
				}
			}
			close(change.done)
		case change := <-hub.grant:
			delete(hub.revoked, change.userID)
			close(change.done)
		case message := <-hub.broadcast:
			for client := range hub.clients {
				if message.ExcludeUserID != 0 && client.UserID == message.ExcludeUserID {
					continue
				}
				select {
				case client.send <- message:
				default:
					client.terminate()
					close(client.send)
					delete(hub.clients, client)
				}
			}
		}
	}
}

func (hub *Hub) purgeExpiredMemberships(now time.Time) {
	for userID, expiresAt := range hub.revoked {
		if !expiresAt.After(now) {
			delete(hub.revoked, userID)
		}
	}
}

func (hub *Hub) Register(client *Client) bool {
	accepted := make(chan bool, 1)
	select {
	case hub.register <- registration{client: client, accepted: accepted}:
	case <-hub.done:
		return false
	}
	select {
	case result := <-accepted:
		return result
	case <-hub.done:
		return false
	}
}

func (hub *Hub) DisconnectUser(userID int64) {
	select {
	case hub.disconnect <- userID:
	case <-hub.done:
	}
}

func (hub *Hub) RevokeUser(userID int64, expiresAt time.Time) {
	hub.changeMembership(hub.revoke, userID, expiresAt)
}

func (hub *Hub) GrantUser(userID int64) {
	hub.changeMembership(hub.grant, userID, time.Time{})
}

func (hub *Hub) DisconnectToken(tokenID string) {
	if tokenID == "" {
		return
	}
	done := make(chan struct{})
	select {
	case hub.disconnectToken <- tokenDisconnection{tokenID: tokenID, done: done}:
	case <-hub.done:
		return
	}
	select {
	case <-done:
	case <-hub.done:
	}
}

func (hub *Hub) changeMembership(channel chan membershipChange, userID int64, expiresAt time.Time) {
	done := make(chan struct{})
	select {
	case channel <- membershipChange{userID: userID, expiresAt: expiresAt, done: done}:
	case <-hub.done:
		return
	}
	select {
	case <-done:
	case <-hub.done:
	}
}

func (hub *Hub) Unregister(client *Client) {
	select {
	case hub.unregister <- client:
	case <-hub.done:
	}
}

func (hub *Hub) Broadcast(message Message) {
	message.Data = append([]byte(nil), message.Data...)
	if message.Type == 0 {
		message.Type = websocket.TextMessage
	}
	select {
	case hub.broadcast <- message:
	case <-hub.done:
	default:
		// Event delivery is best effort; HTTP/database operations must never be
		// blocked by a slow real-time client.
	}
}

func (hub *Hub) Close() {
	hub.closeWith(nil)
}

func (hub *Hub) CloseWithMessage(message Message) {
	message.Data = append([]byte(nil), message.Data...)
	if message.Type == 0 {
		message.Type = websocket.TextMessage
	}
	hub.closeWith(&message)
}

func (hub *Hub) closeWith(finalMessage *Message) {
	hub.closeOnce.Do(func() {
		select {
		case hub.shutdown <- shutdownRequest{finalMessage: finalMessage}:
		case <-hub.done:
		}
	})
}
