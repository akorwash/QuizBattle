package api

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	gameauth "github.com/akorwash/QuizBattle/auth"
)

type middleware func(http.Handler) http.Handler

func limitConcurrency(maximum int) middleware {
	if maximum < 1 {
		maximum = 1
	}
	semaphore := make(chan struct{}, maximum)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
				next.ServeHTTP(w, r)
			default:
				w.Header().Set("Retry-After", "1")
				http.Error(w, "server is busy", http.StatusServiceUnavailable)
			}
		})
	}
}

func chain(handler http.Handler, middlewares ...middleware) http.Handler {
	for index := len(middlewares) - 1; index >= 0; index-- {
		handler = middlewares[index](handler)
	}
	return handler
}

func securityHeaders(enableHSTS bool) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'")
			w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
			w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
			w.Header().Set("Permissions-Policy", "camera=(), geolocation=(), microphone=(self)")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			if enableHSTS {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

func requireSameOrigin(allowedOrigins []string) middleware {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[strings.TrimSuffix(origin, "/")] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			origin := strings.TrimSuffix(r.Header.Get("Origin"), "/")
			if origin == "" {
				// Non-browser API clients may not send Origin. Browser form/fetch
				// requests do, which is the CSRF boundary this check enforces.
				next.ServeHTTP(w, r)
				return
			}
			parsed, err := url.Parse(origin)
			if err == nil && strings.EqualFold(parsed.Host, r.Host) {
				next.ServeHTTP(w, r)
				return
			}
			if _, ok := allowed[origin]; ok {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "cross-origin request rejected", http.StatusForbidden)
		})
	}
}

func recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("panic while handling request", "panic", recovered, "stack", string(debug.Stack()))
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (recorder *statusRecorder) Write(data []byte) (int, error) {
	return recorder.ResponseWriter.Write(data)
}

func (recorder *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := recorder.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (recorder *statusRecorder) Flush() {
	if flusher, ok := recorder.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (recorder *statusRecorder) Push(target string, options *http.PushOptions) error {
	if pusher, ok := recorder.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, options)
	}
	return http.ErrNotSupported
}

func (recorder *statusRecorder) ReadFrom(reader io.Reader) (int64, error) {
	if readerFrom, ok := recorder.ResponseWriter.(io.ReaderFrom); ok {
		return readerFrom.ReadFrom(reader)
	}
	return io.Copy(recorder.ResponseWriter, reader)
}

func (recorder *statusRecorder) WriteHeader(status int) {
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}

func requestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		slog.Info("http request", "method", r.Method, "path", r.URL.Path, "status", recorder.status, "duration", time.Since(started))
	})
}

type rateWindow struct {
	started time.Time
	count   int
}

type ipRateLimiter struct {
	mu             sync.Mutex
	clients        map[string]rateWindow
	limit          int
	window         time.Duration
	trustedProxies []*net.IPNet
}

type identityRateLimiter struct {
	mu     sync.Mutex
	users  map[int64]rateWindow
	limit  int
	window time.Duration
}

func newIdentityRateLimiter(limit int, window time.Duration) *identityRateLimiter {
	return &identityRateLimiter{users: make(map[int64]rateWindow), limit: limit, window: window}
}

func (limiter *identityRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := gameauth.IdentityFromContext(r.Context())
		if !ok {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		now := time.Now()
		limiter.mu.Lock()
		entry, known := limiter.users[identity.UserID]
		if !known && len(limiter.users) >= 10000 {
			for id, candidate := range limiter.users {
				if now.Sub(candidate.started) >= limiter.window {
					delete(limiter.users, id)
				}
			}
			if len(limiter.users) >= 10000 {
				limiter.mu.Unlock()
				w.Header().Set("Retry-After", "60")
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}
		}
		if entry.started.IsZero() || now.Sub(entry.started) >= limiter.window {
			entry = rateWindow{started: now}
		}
		entry.count++
		limiter.users[identity.UserID] = entry
		allowed := entry.count <= limiter.limit
		limiter.mu.Unlock()
		if !allowed {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func newIPRateLimiter(limit int, window time.Duration, trustedProxyCIDRs ...string) *ipRateLimiter {
	limiter := &ipRateLimiter{clients: make(map[string]rateWindow), limit: limit, window: window}
	for _, value := range trustedProxyCIDRs {
		if _, network, err := net.ParseCIDR(value); err == nil {
			limiter.trustedProxies = append(limiter.trustedProxies, network)
		}
	}
	return limiter
}

func (limiter *ipRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := limiter.clientIP(r)
		now := time.Now()
		limiter.mu.Lock()
		entry, known := limiter.clients[host]
		if !known && len(limiter.clients) >= 10000 {
			for key, candidate := range limiter.clients {
				if now.Sub(candidate.started) >= limiter.window {
					delete(limiter.clients, key)
				}
			}
			if len(limiter.clients) >= 10000 {
				limiter.mu.Unlock()
				w.Header().Set("Retry-After", "60")
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}
		}
		if entry.started.IsZero() || now.Sub(entry.started) >= limiter.window {
			entry = rateWindow{started: now}
		}
		entry.count++
		limiter.clients[host] = entry
		allowed := entry.count <= limiter.limit
		limiter.mu.Unlock()
		if !allowed {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (limiter *ipRateLimiter) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(strings.TrimSpace(host))
	if peer == nil || !limiter.isTrustedProxy(peer) {
		return host
	}
	forwarded := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	current := peer
	for index := len(forwarded) - 1; index >= 0 && limiter.isTrustedProxy(current); index-- {
		candidate := net.ParseIP(strings.TrimSpace(forwarded[index]))
		if candidate == nil {
			break
		}
		current = candidate
	}
	return current.String()
}

func (limiter *ipRateLimiter) isTrustedProxy(ip net.IP) bool {
	for _, network := range limiter.trustedProxies {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
