package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/akorwash/QuizBattle/config"
)

func TestHealthIncludesReleaseIdentity(t *testing.T) {
	const release = "0123456789abcdef0123456789abcdef01234567"
	app := &App{config: config.Config{ReleaseSHA: release}}
	response := httptest.NewRecorder()

	app.health(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", response.Code)
	}
	if response.Header().Get("Content-Type") != "application/json" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unexpected health headers: %#v", response.Header())
	}
	var payload struct {
		Status  string `json:"status"`
		Release string `json:"release"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Status != "ok" || payload.Release != release {
		t.Fatalf("unexpected health payload: %#v", payload)
	}
}
