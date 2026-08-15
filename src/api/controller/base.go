package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	gameauth "github.com/akorwash/QuizBattle/auth"
	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/domain/economy"
	matchdomain "github.com/akorwash/QuizBattle/domain/match"
	"github.com/akorwash/QuizBattle/handler"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/service"
	"golang.org/x/sync/singleflight"
)

const (
	SessionCookieName     = "quizbattle_session"
	HostSessionCookieName = "__Host-quizbattle_session"
	maxRequestBody        = 64 << 10
	maxRevokedSessions    = 100000
	maxValidationCache    = 10000
	validationCacheTTL    = 5 * time.Second
	validationErrorLogTTL = 10 * time.Second
)

var responseHandler handler.WebResponseHandler

type Authenticator struct {
	tokens          *gameauth.TokenManager
	cookieSecure    bool
	cookieName      string
	revocations     *sessionRevocations
	revocationStore SessionRevocationStore
	validationCache *sessionValidationCache
	validationGroup singleflight.Group
	revocationGroup singleflight.Group
}

type SessionRevocationStore interface {
	SaveSessionRevocation(tokenID string, expiresAt time.Time) error
	IsSessionRevoked(tokenID string) (bool, error)
}

func NewAuthenticator(tokens *gameauth.TokenManager, cookieSecure bool, stores ...SessionRevocationStore) *Authenticator {
	cookieName := SessionCookieName
	if cookieSecure {
		cookieName = HostSessionCookieName
	}
	authenticator := &Authenticator{
		tokens:       tokens,
		cookieSecure: cookieSecure,
		cookieName:   cookieName,
		revocations:  &sessionRevocations{tokens: make(map[string]time.Time), now: time.Now},
		validationCache: &sessionValidationCache{
			entries: make(map[string]sessionValidation),
			now:     time.Now,
		},
	}
	if len(stores) > 0 {
		authenticator.revocationStore = stores[0]
	}
	return authenticator
}

func (authenticator *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawToken, err := authenticator.extractToken(r)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		identity, err := authenticator.Verify(rawToken)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r.WithContext(gameauth.WithIdentity(r.Context(), identity)))
	})
}

func (authenticator *Authenticator) PageMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawToken, err := authenticator.extractToken(r)
		if err != nil {
			http.Redirect(w, r, "/auth/signin", http.StatusSeeOther)
			return
		}
		identity, err := authenticator.Verify(rawToken)
		if err != nil {
			http.Redirect(w, r, "/auth/signin", http.StatusSeeOther)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r.WithContext(gameauth.WithIdentity(r.Context(), identity)))
	})
}

func (authenticator *Authenticator) IssueSession(w http.ResponseWriter, user entites.User) error {
	token, err := authenticator.tokens.Issue(user)
	if err != nil {
		return err
	}
	// #nosec G124 -- config rejects Secure=false in production; local HTTP
	// development and tests may explicitly disable it.
	http.SetCookie(w, &http.Cookie{
		Name:     authenticator.cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(authenticator.tokens.TTL().Seconds()),
		HttpOnly: true,
		Secure:   authenticator.cookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
	return nil
}

func (authenticator *Authenticator) ClearSession(w http.ResponseWriter) {
	// #nosec G124 -- config rejects Secure=false in production; local HTTP
	// development and tests may explicitly disable it.
	http.SetCookie(w, &http.Cookie{
		Name:     authenticator.cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   authenticator.cookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
	if authenticator.cookieName != SessionCookieName {
		// #nosec G124 -- this expires the migration-only legacy cookie; secure
		// environments still require Secure=true through validated config.
		http.SetCookie(w, &http.Cookie{
			Name:     SessionCookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			Expires:  time.Unix(1, 0),
			HttpOnly: true,
			Secure:   authenticator.cookieSecure,
			SameSite: http.SameSiteStrictMode,
		})
	}
}

func Identity(r *http.Request) (gameauth.Identity, error) {
	identity, ok := gameauth.IdentityFromContext(r.Context())
	if !ok {
		return gameauth.Identity{}, gameauth.ErrInvalidToken
	}
	return identity, nil
}

func (authenticator *Authenticator) extractToken(r *http.Request) (string, error) {
	if authorization := strings.TrimSpace(r.Header.Get("Authorization")); authorization != "" {
		parts := strings.Fields(authorization)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			return "", gameauth.ErrInvalidToken
		}
		return parts[1], nil
	}
	cookie, err := r.Cookie(authenticator.cookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return "", gameauth.ErrInvalidToken
	}
	return cookie.Value, nil
}

func (authenticator *Authenticator) Verify(rawToken string) (gameauth.Identity, error) {
	identity, err := authenticator.tokens.Verify(rawToken)
	if err != nil || !authenticator.SessionActive(identity.TokenID) {
		return gameauth.Identity{}, gameauth.ErrInvalidToken
	}
	return identity, nil
}

func (authenticator *Authenticator) SessionActive(tokenID string) bool {
	if authenticator.revocations.isRevoked(tokenID) {
		return false
	}
	if authenticator.revocationStore == nil {
		return true
	}
	if active, cached := authenticator.validationCache.get(tokenID); cached {
		return active
	}

	value, storeErr, _ := authenticator.validationGroup.Do(tokenID, func() (any, error) {
		if authenticator.revocations.isRevoked(tokenID) {
			return false, nil
		}
		if active, cached := authenticator.validationCache.get(tokenID); cached {
			return active, nil
		}
		revoked, err := authenticator.revocationStore.IsSessionRevoked(tokenID)
		if err != nil {
			return false, err
		}
		active := !revoked
		authenticator.validationCache.put(tokenID, active)
		return active, nil
	})
	if storeErr != nil {
		authenticator.validationCache.put(tokenID, false)
		authenticator.validationCache.logStoreError(storeErr)
		return false
	}
	active, ok := value.(bool)
	return ok && active
}

func (authenticator *Authenticator) verifySignedToken(rawToken string) (gameauth.Identity, error) {
	return authenticator.tokens.Verify(rawToken)
}

func (authenticator *Authenticator) Revoke(identity gameauth.Identity) error {
	revocationExpiry := authenticator.revocationExpiry(identity)
	if identity.TokenID == "" || !revocationExpiry.After(authenticator.revocations.now()) {
		return nil
	}
	_, err, _ := authenticator.revocationGroup.Do(identity.TokenID, func() (any, error) {
		if authenticator.revocations.isRevoked(identity.TokenID) {
			return nil, nil
		}
		if authenticator.revocationStore != nil {
			alreadyRevoked, lookupErr := authenticator.revocationStore.IsSessionRevoked(identity.TokenID)
			if lookupErr == nil && alreadyRevoked {
				_ = authenticator.revocations.add(identity.TokenID, revocationExpiry)
				authenticator.validationCache.put(identity.TokenID, false)
				return nil, nil
			}
			// A lookup outage must not turn logout into a false success. Persisting
			// the idempotent upsert is safe and lets the caller retry on failure.
			if saveErr := authenticator.revocationStore.SaveSessionRevocation(identity.TokenID, revocationExpiry); saveErr != nil {
				return nil, fmt.Errorf("persist session revocation: %w", saveErr)
			}
		}
		if !authenticator.revocations.add(identity.TokenID, revocationExpiry) && authenticator.revocationStore == nil {
			return nil, fmt.Errorf("session revocation capacity reached")
		}
		authenticator.validationCache.put(identity.TokenID, false)
		return nil, nil
	})
	return err
}

func (authenticator *Authenticator) revocationExpiry(identity gameauth.Identity) time.Time {
	return identity.ExpiresAt.Add(gameauth.TokenValidationLeeway)
}

type sessionRevocations struct {
	mu     sync.Mutex
	tokens map[string]time.Time
	now    func() time.Time
}

type sessionValidation struct {
	active    bool
	expiresAt time.Time
}

type sessionValidationCache struct {
	mu                sync.Mutex
	entries           map[string]sessionValidation
	now               func() time.Time
	lastStoreErrorLog time.Time
}

func (cache *sessionValidationCache) get(tokenID string) (bool, bool) {
	if tokenID == "" {
		return false, true
	}
	now := cache.now()
	cache.mu.Lock()
	defer cache.mu.Unlock()
	entry, exists := cache.entries[tokenID]
	if exists && !entry.expiresAt.After(now) {
		delete(cache.entries, tokenID)
		return false, false
	}
	return entry.active, exists
}

func (cache *sessionValidationCache) put(tokenID string, active bool) {
	if tokenID == "" {
		return
	}
	now := cache.now()
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if _, exists := cache.entries[tokenID]; !exists && len(cache.entries) >= maxValidationCache {
		for id := range cache.entries {
			delete(cache.entries, id)
			break
		}
	}
	cache.entries[tokenID] = sessionValidation{active: active, expiresAt: now.Add(validationCacheTTL)}
}

func (cache *sessionValidationCache) logStoreError(err error) {
	now := cache.now()
	cache.mu.Lock()
	if now.Sub(cache.lastStoreErrorLog) < validationErrorLogTTL {
		cache.mu.Unlock()
		return
	}
	cache.lastStoreErrorLog = now
	cache.mu.Unlock()
	slog.Error("session revocation lookup failed", "error", err)
}

func (revocations *sessionRevocations) add(tokenID string, expiresAt time.Time) bool {
	now := revocations.now()
	revocations.mu.Lock()
	defer revocations.mu.Unlock()
	revocations.purgeExpired(now)
	if len(revocations.tokens) >= maxRevokedSessions {
		return false
	}
	revocations.tokens[tokenID] = expiresAt
	return true
}

func (revocations *sessionRevocations) isRevoked(tokenID string) bool {
	if tokenID == "" {
		return true
	}
	now := revocations.now()
	revocations.mu.Lock()
	defer revocations.mu.Unlock()
	expiresAt, exists := revocations.tokens[tokenID]
	if exists && !expiresAt.After(now) {
		delete(revocations.tokens, tokenID)
		return false
	}
	return exists
}

func (revocations *sessionRevocations) purgeExpired(now time.Time) {
	for tokenID, expiresAt := range revocations.tokens {
		if !expiresAt.After(now) {
			delete(revocations.tokens, tokenID)
		}
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	if contentType := r.Header.Get("Content-Type"); contentType != "" && !strings.HasPrefix(strings.ToLower(contentType), "application/json") {
		return fmt.Errorf("Content-Type must be application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("request body must contain exactly one JSON object")
	}
	return nil
}

func respondServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		responseHandler.RespondWithError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrInvalidCredentials):
		responseHandler.RespondWithError(w, http.StatusUnauthorized, "invalid identifier or password")
	case errors.Is(err, service.ErrForbidden):
		responseHandler.RespondWithError(w, http.StatusForbidden, "you do not have access to this resource")
	case errors.Is(err, economy.ErrNotOwner), errors.Is(err, matchdomain.ErrNotOwner), errors.Is(err, matchdomain.ErrNotPlayer), errors.Is(err, matchdomain.ErrNotEligible):
		responseHandler.RespondWithError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, economy.ErrInvalidPrice), errors.Is(err, economy.ErrInvalidEconomyState), errors.Is(err, economy.ErrInvalidTrade),
		errors.Is(err, matchdomain.ErrInvalidMatch), errors.Is(err, matchdomain.ErrInvalidDeck), errors.Is(err, matchdomain.ErrInvalidCommandID),
		errors.Is(err, matchdomain.ErrInvalidMode), errors.Is(err, matchdomain.ErrInvalidOption), errors.Is(err, matchdomain.ErrInvalidTurn):
		responseHandler.RespondWithError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, economy.ErrCardUnavailable), errors.Is(err, economy.ErrInsufficientCoins), errors.Is(err, economy.ErrSelfPurchase),
		errors.Is(err, economy.ErrInvalidListing), errors.Is(err, matchdomain.ErrDecksNotReady), errors.Is(err, matchdomain.ErrInvalidState),
		errors.Is(err, matchdomain.ErrTurnClosed), errors.Is(err, matchdomain.ErrAlreadyAnswered), errors.Is(err, matchdomain.ErrTieBreakPoolExhausted):
		responseHandler.RespondWithError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrAccountExists):
		responseHandler.RespondWithError(w, http.StatusConflict, "account details are already in use")
	case errors.Is(err, service.ErrGameClosed):
		responseHandler.RespondWithError(w, http.StatusGone, err.Error())
	case errors.Is(err, service.ErrAlreadyJoined), errors.Is(err, service.ErrActiveGameLimit), errors.Is(err, service.ErrBattleFull), errors.Is(err, service.ErrArenaNotReady), errors.Is(err, service.ErrMatchInProgress):
		responseHandler.RespondWithError(w, http.StatusConflict, err.Error())
	case errors.Is(err, repository.ErrConflict):
		responseHandler.RespondWithError(w, http.StatusConflict, "resource state changed; refresh and retry")
	case errors.Is(err, repository.ErrNotFound):
		responseHandler.RespondWithError(w, http.StatusNotFound, "resource not found")
	default:
		slog.Error("request failed", "error", err)
		responseHandler.RespondWithError(w, http.StatusInternalServerError, "internal server error")
	}
}
