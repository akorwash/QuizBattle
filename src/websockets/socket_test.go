package websockets

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/akorwash/QuizBattle/resources"
	"github.com/gorilla/websocket"
)

type chatStoreStub struct {
	fail bool
}

func (store chatStoreStub) Save(_ context.Context, userID int64, username, fullName, text string) (*resources.ChatMessage, error) {
	if store.fail {
		return nil, context.DeadlineExceeded
	}
	return &resources.ChatMessage{
		ID:        9001,
		UserID:    userID,
		Username:  username,
		FullName:  fullName,
		Message:   text,
		CreatedAt: time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC),
	}, nil
}

func TestChatHandlerStampsAuthenticatedIdentity(t *testing.T) {
	handler := chatHandler(42, "server-user", "Server Name", newChatRateLimiter(5, time.Second), chatStoreStub{})
	message, ok := handler(websocket.TextMessage, []byte(`{"type":"text","message":" hello ","userId":99,"fullName":"Spoofed"}`))
	if !ok {
		t.Fatal("valid chat message was rejected")
	}
	var outgoing outgoingChatMessage
	if err := json.Unmarshal(message.Data, &outgoing); err != nil {
		t.Fatal(err)
	}
	if outgoing.UserID != 42 || outgoing.Username != "server-user" || outgoing.FullName != "Server Name" || outgoing.Message != "hello" {
		t.Fatalf("client identity was trusted: %#v", outgoing)
	}
	var raw map[string]any
	if err := json.Unmarshal(message.Data, &raw); err != nil || raw["userId"] != "42" {
		t.Fatalf("chat user ID was not encoded as a string: %s", message.Data)
	}
}

func TestChatHandlerRejectsUnsafeOrFloodedMessages(t *testing.T) {
	handler := chatHandler(42, "player", "Player", newChatRateLimiter(5, time.Second), chatStoreStub{})
	if _, ok := handler(websocket.BinaryMessage, []byte("binary")); ok {
		t.Fatal("binary message was accepted")
	}
	if _, ok := handler(websocket.TextMessage, []byte("{not-json")); ok {
		t.Fatal("malformed JSON was accepted")
	}
	if _, ok := handler(websocket.TextMessage, []byte(`{"type":"text","message":"\u0001"}`)); ok {
		t.Fatal("control character was accepted")
	}

	handler = chatHandler(42, "player", "Player", newChatRateLimiter(5, time.Second), chatStoreStub{})
	for index := 0; index < 5; index++ {
		if _, ok := handler(websocket.TextMessage, []byte(`{"type":"text","message":"ok"}`)); !ok {
			t.Fatalf("message %d was unexpectedly rate limited", index+1)
		}
	}
	if _, ok := handler(websocket.TextMessage, []byte(`{"type":"text","message":"six"}`)); ok {
		t.Fatal("sixth message in one second was accepted")
	}
	if validChatText(strings.Repeat("a", 501)) {
		t.Fatal("oversized chat message was accepted")
	}
}

func TestChatHandlerDoesNotBroadcastUnpersistedMessages(t *testing.T) {
	handler := chatHandler(42, "player", "Player", newChatRateLimiter(5, time.Second), chatStoreStub{fail: true})
	if _, ok := handler(websocket.TextMessage, []byte(`{"type":"text","message":"must persist"}`)); ok {
		t.Fatal("message was broadcast even though persistence failed")
	}
}

func TestVoiceSignalHandlerSanitizesAndRoutesSignals(t *testing.T) {
	handler := voiceSignalHandler(42, newChatRateLimiter(40, 5*time.Second))
	message, ok := handler(websocket.TextMessage, []byte(`{"type":"voice_offer","fromUserId":"99","payload":{"sdp":{"type":"offer","sdp":"v=0\r\na=group:BUNDLE 0"}}}`))
	if !ok {
		t.Fatal("valid voice offer was rejected")
	}
	if message.ExcludeUserID != 42 {
		t.Fatalf("voice signal was not excluded from its sender: %d", message.ExcludeUserID)
	}
	var outgoing outgoingVoiceSignal
	if err := json.Unmarshal(message.Data, &outgoing); err != nil {
		t.Fatal(err)
	}
	if outgoing.FromUserID != 42 || outgoing.Type != "voice_offer" || outgoing.Payload.SDP == nil || outgoing.Payload.SDP.Type != "offer" {
		t.Fatalf("voice signal trusted spoofed identity or changed payload: %#v", outgoing)
	}
	var raw map[string]any
	if err := json.Unmarshal(message.Data, &raw); err != nil || raw["fromUserId"] != "42" {
		t.Fatalf("voice user ID was not encoded as a string: %s", message.Data)
	}
}

func TestVoiceSignalHandlerValidatesProtocolAndBounds(t *testing.T) {
	handler := voiceSignalHandler(42, newChatRateLimiter(40, 5*time.Second))
	valid := []string{
		`{"type":"voice_ready","payload":{}}`,
		`{"type":"voice_leave","payload":{}}`,
		`{"type":"voice_answer","payload":{"sdp":{"type":"answer","sdp":"v=0\r\n"}}}`,
		`{"type":"voice_ice","payload":{"candidate":{"candidate":"candidate:1 1 UDP 1 192.0.2.1 5000 typ host","sdpMid":"0","sdpMLineIndex":0}}}`,
	}
	for _, payload := range valid {
		if _, ok := handler(websocket.TextMessage, []byte(payload)); !ok {
			t.Fatalf("valid voice signal was rejected: %s", payload)
		}
	}

	invalid := []string{
		`{"type":"text","payload":{}}`,
		`{"type":"voice_ready","payload":{"candidate":{"candidate":"x"}}}`,
		`{"type":"voice_offer","payload":{"sdp":{"type":"answer","sdp":"v=0"}}}`,
		`{"type":"voice_ice","payload":{"candidate":{"candidate":"x","sdpMLineIndex":999}}}`,
		`{"type":"voice_offer","payload":{"sdp":{"type":"offer","sdp":"\u0001"}}}`,
	}
	for _, payload := range invalid {
		if _, ok := handler(websocket.TextMessage, []byte(payload)); ok {
			t.Fatalf("invalid voice signal was accepted: %s", payload)
		}
	}

	oversized, err := json.Marshal(incomingVoiceSignal{
		Type: "voice_offer",
		Payload: voiceSignalPayload{SDP: &voiceSessionDescription{
			Type: "offer",
			SDP:  strings.Repeat("a", 12*1024+1),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := handler(websocket.TextMessage, oversized); ok {
		t.Fatal("oversized SDP was accepted")
	}
}

func TestWebSocketOriginPolicy(t *testing.T) {
	registry := NewRegistry([]string{"https://play.example.com"})
	defer registry.Close()

	request := httptest.NewRequest("GET", "http://quiz.test/ws", nil)
	request.Host = "quiz.test"
	request.Header.Set("Origin", "https://evil.example")
	if registry.checkOrigin(request) {
		t.Fatal("cross-site origin was accepted")
	}
	request.Header.Set("Origin", "http://quiz.test")
	if !registry.checkOrigin(request) {
		t.Fatal("same-origin websocket was rejected")
	}
	request.Header.Set("Origin", "https://play.example.com")
	if !registry.checkOrigin(request) {
		t.Fatal("configured origin was rejected")
	}
}

func TestHubLimitsConnectionsPerAuthenticatedUser(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Close()
	for index := 0; index < maxConnectionsPerUser; index++ {
		client := &Client{UserID: 42, TokenID: "token", send: make(chan Message, 1)}
		if !hub.Register(client) {
			t.Fatalf("connection %d was unexpectedly rejected", index+1)
		}
	}
	extra := &Client{UserID: 42, TokenID: "token", send: make(chan Message, 1)}
	if hub.Register(extra) {
		t.Fatal("connection limit could be bypassed with another tab")
	}
}

func TestHubCanRestrictBattleUserToOneSignalingConnection(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Close()
	first := &Client{UserID: 42, TokenID: "token-1", HubUserLimit: 1, send: make(chan Message, 1)}
	if !hub.Register(first) {
		t.Fatal("first battle connection was rejected")
	}
	second := &Client{UserID: 42, TokenID: "token-2", HubUserLimit: 1, send: make(chan Message, 1)}
	if hub.Register(second) {
		t.Fatal("second battle signaling connection for the same user was accepted")
	}
}

func TestRegistryQuotaLimitsConnectionsAcrossHubs(t *testing.T) {
	quota := newConnectionQuota(2)
	if !quota.acquire(42) || !quota.acquire(42) {
		t.Fatal("valid aggregate websocket slots were rejected")
	}
	if quota.acquire(42) {
		t.Fatal("aggregate websocket limit was bypassed across hubs")
	}
	quota.release(42)
	if !quota.acquire(42) {
		t.Fatal("released websocket slot was not reusable")
	}
}

func TestHubRevocationRejectsRacingRegistration(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Close()
	first := &Client{UserID: 42, TokenID: "token", send: make(chan Message, 1)}
	if !hub.Register(first) {
		t.Fatal("initial connection was rejected")
	}
	hub.RevokeUser(42, time.Now().Add(time.Minute))
	if hub.Register(&Client{UserID: 42, TokenID: "token", send: make(chan Message, 1)}) {
		t.Fatal("revoked battle member reconnected")
	}
	hub.GrantUser(42)
	if !hub.Register(&Client{UserID: 42, TokenID: "token", send: make(chan Message, 1)}) {
		t.Fatal("rejoined battle member remained revoked")
	}
}

func TestHubDeliversFinalEventBeforeClosing(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	client := &Client{UserID: 42, TokenID: "token", send: make(chan Message, 1)}
	if !hub.Register(client) {
		t.Fatal("connection was rejected")
	}
	hub.CloseWithMessage(Message{Type: websocket.TextMessage, Data: []byte(`{"type":"closed"}`)})
	message, ok := <-client.send
	if !ok || !strings.Contains(string(message.Data), `"closed"`) {
		t.Fatalf("final close event was lost: ok=%v message=%q", ok, message.Data)
	}
}

func TestHubRejectsRegistrationRacingWithSessionRevocation(t *testing.T) {
	registry := NewRegistry(nil)
	defer registry.Close()
	hub := registry.eventsHub
	register := registry.clientRegistrar(hub, 0)
	old := &Client{UserID: 42, TokenID: "old-token", ExpiresAt: time.Now().Add(time.Hour), send: make(chan Message, 1)}
	if !register(old) {
		t.Fatal("initial session connection was rejected")
	}
	registry.DisconnectSession(42, "old-token", time.Now().Add(time.Hour))
	if register(&Client{UserID: 42, TokenID: "old-token", ExpiresAt: time.Now().Add(time.Hour), send: make(chan Message, 1)}) {
		t.Fatal("revoked session registered a websocket after logout")
	}
	if !register(&Client{UserID: 42, TokenID: "new-token", ExpiresAt: time.Now().Add(time.Hour), send: make(chan Message, 1)}) {
		t.Fatal("new session was blocked by an older session revocation")
	}
	if register(&Client{UserID: 42, TokenID: "new-token-2", ExpiresAt: time.Now().Add(-time.Second), send: make(chan Message, 1)}) {
		t.Fatal("session that expired during websocket upgrade was registered")
	}
}

func TestRejectedBattleUpgradeDoesNotLeakGameHub(t *testing.T) {
	registry := NewRegistry(nil)
	defer registry.Close()
	request := httptest.NewRequest("GET", "http://quiz.test/ws/game/7", nil)
	response := httptest.NewRecorder()
	registry.ServeBattle(7, 42, "token", "Player", time.Now().Add(time.Hour), response, request)

	registry.mu.Lock()
	_, gameHubExists := registry.gameHubs[7]
	hubCount := len(registry.hubGames)
	registry.mu.Unlock()
	if gameHubExists || hubCount != 0 {
		t.Fatalf("rejected websocket upgrade leaked a game hub: game=%v hubs=%d", gameHubExists, hubCount)
	}
}
