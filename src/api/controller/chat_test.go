package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gameauth "github.com/akorwash/QuizBattle/auth"
	"github.com/akorwash/QuizBattle/resources"
)

type chatHistoryReaderStub struct {
	messages []resources.ChatMessage
	err      error
	called   bool
}

func (stub *chatHistoryReaderStub) Recent(context.Context) ([]resources.ChatMessage, error) {
	stub.called = true
	return stub.messages, stub.err
}

func TestChatMessagesRequiresIdentity(t *testing.T) {
	stub := &chatHistoryReaderStub{}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/chat/messages", nil)
	response := httptest.NewRecorder()
	new(ChatController).Messages(stub).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || stub.called {
		t.Fatalf("unauthenticated history request: status=%d called=%v", response.Code, stub.called)
	}
}

func TestChatMessagesReturnsLiveCompatibleJSON(t *testing.T) {
	createdAt := time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC)
	stub := &chatHistoryReaderStub{messages: []resources.ChatMessage{{
		ID: 901, UserID: 42, Username: "player_1", FullName: "لاعب أول", Message: "مرحبًا", CreatedAt: createdAt,
	}}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/chat/messages", nil)
	request = request.WithContext(gameauth.WithIdentity(request.Context(), gameauth.Identity{UserID: 42, Username: "player_1"}))
	response := httptest.NewRecorder()
	new(ChatController).Messages(stub).ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected response: status=%d content-type=%q", response.Code, response.Header().Get("Content-Type"))
	}
	var payload []map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload) != 1 || payload[0]["id"] != "901" || payload[0]["userId"] != "42" || payload[0]["message"] != "مرحبًا" || payload[0]["username"] != "player_1" {
		t.Fatalf("unexpected payload: %v", payload)
	}
}

func TestChatMessagesMapsStorageFailure(t *testing.T) {
	stub := &chatHistoryReaderStub{err: errors.New("database unavailable")}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/chat/messages", nil)
	request = request.WithContext(gameauth.WithIdentity(request.Context(), gameauth.Identity{UserID: 42, Username: "player_1"}))
	response := httptest.NewRecorder()
	new(ChatController).Messages(stub).ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("storage failure returned %d", response.Code)
	}
}
