package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gameauth "github.com/akorwash/QuizBattle/auth"
)

func TestConcurrencyLimiterRejectsExcessWorkWithoutQueueing(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	handler := limitConcurrency(1)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "http://quiz.test/slow", nil))
		close(done)
	}()
	<-entered
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://quiz.test/second", nil))
	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Retry-After") == "" {
		t.Fatalf("excess concurrent request returned %d with headers %#v", response.Code, response.Header())
	}
	close(release)
	<-done
}

func TestSameOriginMiddlewareRejectsCrossSiteWrites(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := requireSameOrigin([]string{"https://trusted.example"})(next)

	request := httptest.NewRequest(http.MethodPost, "http://quiz.test/api", nil)
	request.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-site write returned %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "http://quiz.test/api", nil)
	request.Header.Set("Origin", "http://quiz.test")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("same-origin write returned %d", response.Code)
	}
}

func TestSecurityHeadersAndRateLimit(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	secure := securityHeaders(true)(next)
	response := httptest.NewRecorder()
	secure.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "https://quiz.test/", nil))
	if response.Header().Get("Content-Security-Policy") == "" || response.Header().Get("Strict-Transport-Security") == "" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("security headers missing: %#v", response.Header())
	}
	if permissions := response.Header().Get("Permissions-Policy"); permissions != "camera=(), geolocation=(), microphone=(self)" {
		t.Fatalf("unexpected microphone permission policy: %q", permissions)
	}

	limited := newIPRateLimiter(1, time.Minute).Middleware(next)
	request := httptest.NewRequest(http.MethodPost, "http://quiz.test/login", nil)
	request.RemoteAddr = "192.0.2.1:1000"
	limited.ServeHTTP(httptest.NewRecorder(), request)
	request = httptest.NewRequest(http.MethodPost, "http://quiz.test/login", nil)
	request.RemoteAddr = "192.0.2.1:2000"
	response = httptest.NewRecorder()
	limited.ServeHTTP(response, request)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("rate limit returned %d", response.Code)
	}
}

func TestPathIDValidation(t *testing.T) {
	if id, err := strconvPathID("42"); err != nil || id != 42 {
		t.Fatalf("valid ID failed: %d %v", id, err)
	}
	for _, value := range []string{"", "0", "-1", "not-a-number"} {
		if _, err := strconvPathID(value); err == nil {
			t.Fatalf("invalid ID %q was accepted", value)
		}
	}
}

func TestAuthenticatedMutationRateLimitUsesServerIdentity(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := newIdentityRateLimiter(1, time.Minute).Middleware(next)
	request := httptest.NewRequest(http.MethodPost, "http://quiz.test/api/v1/game", nil)
	request = request.WithContext(gameauth.WithIdentity(request.Context(), gameauth.Identity{UserID: 42, Username: "player"}))
	handler.ServeHTTP(httptest.NewRecorder(), request)

	request = httptest.NewRequest(http.MethodPost, "http://quiz.test/api/v1/game", nil)
	request = request.WithContext(gameauth.WithIdentity(request.Context(), gameauth.Identity{UserID: 42, Username: "player"}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("authenticated mutation limit returned %d", response.Code)
	}
}

func TestIPRateLimiterTrustsForwardingHeadersOnlyFromConfiguredProxies(t *testing.T) {
	limiter := newIPRateLimiter(10, time.Minute, "10.0.0.0/8")
	untrusted := httptest.NewRequest(http.MethodPost, "http://quiz.test/login", nil)
	untrusted.RemoteAddr = "192.0.2.10:1234"
	untrusted.Header.Set("X-Forwarded-For", "203.0.113.5")
	if got := limiter.clientIP(untrusted); got != "192.0.2.10" {
		t.Fatalf("forged forwarding header was trusted: %q", got)
	}

	trusted := httptest.NewRequest(http.MethodPost, "http://quiz.test/login", nil)
	trusted.RemoteAddr = "10.0.0.5:1234"
	trusted.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.4")
	if got := limiter.clientIP(trusted); got != "203.0.113.5" {
		t.Fatalf("trusted proxy chain resolved to %q", got)
	}
}
