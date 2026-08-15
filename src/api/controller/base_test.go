package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gameauth "github.com/akorwash/QuizBattle/auth"
	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/resources"
)

const controllerTestSecret = "0123456789abcdef0123456789abcdef" //gitleaks:allow -- deterministic test fixture

type updateCapture struct {
	called bool
	userID int64
	input  resources.UpdateAccountModel
}

type revocationStoreStub struct {
	tokens    map[string]time.Time
	saveErr   error
	lookupErr error
	saveCalls int
	lookups   int
}

func (store *revocationStoreStub) SaveSessionRevocation(tokenID string, expiresAt time.Time) error {
	store.saveCalls++
	if store.saveErr != nil {
		return store.saveErr
	}
	store.tokens[tokenID] = expiresAt
	return nil
}

func (store *revocationStoreStub) IsSessionRevoked(tokenID string) (bool, error) {
	store.lookups++
	if store.lookupErr != nil {
		return false, store.lookupErr
	}
	expiresAt, exists := store.tokens[tokenID]
	return exists && expiresAt.After(time.Now()), nil
}

func TestSessionValidationCoalescesRepeatedStoreLookups(t *testing.T) {
	tokens, err := gameauth.NewTokenManager(controllerTestSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	store := &revocationStoreStub{tokens: make(map[string]time.Time)}
	authenticator := NewAuthenticator(tokens, true, store)
	if !authenticator.SessionActive("same-token") || !authenticator.SessionActive("same-token") {
		t.Fatal("active session was rejected")
	}
	if store.lookups != 1 {
		t.Fatalf("repeated validation issued %d store lookups; want one", store.lookups)
	}
}

func TestLogoutPersistsRevocationEvenWhenLookupIsUnavailable(t *testing.T) {
	tokens, err := gameauth.NewTokenManager(controllerTestSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	store := &revocationStoreStub{tokens: make(map[string]time.Time), lookupErr: errors.New("lookup unavailable")}
	authenticator := NewAuthenticator(tokens, true, store)
	token, err := tokens.Issue(entites.User{ID: 42, Username: "player"})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/user/logout", nil)
	request.AddCookie(&http.Cookie{Name: HostSessionCookieName, Value: token})
	response := httptest.NewRecorder()
	NewUserController(authenticator, nil).Logout(response, request)
	if response.Code != http.StatusNoContent || store.saveCalls != 1 {
		t.Fatalf("logout did not persist revocation safely: status=%d saves=%d", response.Code, store.saveCalls)
	}
}

func TestRepeatedLogoutPersistsRevocationOnlyOnce(t *testing.T) {
	tokens, err := gameauth.NewTokenManager(controllerTestSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	store := &revocationStoreStub{tokens: make(map[string]time.Time)}
	authenticator := NewAuthenticator(tokens, true, store)
	token, err := tokens.Issue(entites.User{ID: 42, Username: "player"})
	if err != nil {
		t.Fatal(err)
	}
	controller := NewUserController(authenticator, nil)
	for attempt := 0; attempt < 2; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/user/logout", nil)
		request.AddCookie(&http.Cookie{Name: HostSessionCookieName, Value: token})
		response := httptest.NewRecorder()
		controller.Logout(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("logout attempt %d returned %d", attempt+1, response.Code)
		}
	}
	if store.saveCalls != 1 {
		t.Fatalf("repeated logout persisted %d revocations; want one", store.saveCalls)
	}
}

func TestLogoutKeepsCookieWhenRevocationCannotBePersisted(t *testing.T) {
	tokens, err := gameauth.NewTokenManager(controllerTestSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	store := &revocationStoreStub{tokens: make(map[string]time.Time), saveErr: errors.New("write unavailable")}
	authenticator := NewAuthenticator(tokens, true, store)
	token, err := tokens.Issue(entites.User{ID: 42, Username: "player"})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/user/logout", nil)
	request.AddCookie(&http.Cookie{Name: HostSessionCookieName, Value: token})
	response := httptest.NewRecorder()
	NewUserController(authenticator, nil).Logout(response, request)
	if response.Code != http.StatusServiceUnavailable || len(response.Result().Cookies()) != 0 {
		t.Fatalf("failed revocation falsely cleared session: status=%d cookies=%#v", response.Code, response.Result().Cookies())
	}
}

func (capture *updateCapture) UpdateUser(userID int64, input resources.UpdateAccountModel) (*resources.UserAccount, error) {
	capture.called = true
	capture.userID = userID
	capture.input = input
	return &resources.UserAccount{UserID: userID, Username: "player", FullName: input.FullName}, nil
}

func testAuthenticator(t *testing.T) *Authenticator {
	t.Helper()
	tokens, err := gameauth.NewTokenManager(controllerTestSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return NewAuthenticator(tokens, true)
}

func TestAuthenticationDoesNotAcceptQueryStringTokens(t *testing.T) {
	authenticator := testAuthenticator(t)
	token, err := authenticator.tokens.Issue(entites.User{ID: 42, Username: "player"})
	if err != nil {
		t.Fatal(err)
	}
	nextCalled := false
	handler := authenticator.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		nextCalled = true
	}))

	request := httptest.NewRequest(http.MethodGet, "/private?token="+token, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || nextCalled {
		t.Fatalf("query token was accepted: status=%d called=%v", response.Code, nextCalled)
	}
}

func TestAuthenticationCookieInjectsServerIdentity(t *testing.T) {
	authenticator := testAuthenticator(t)
	token, err := authenticator.tokens.Issue(entites.User{ID: 42, Username: "player", Fullname: "Player One"})
	if err != nil {
		t.Fatal(err)
	}
	handler := authenticator.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, identityErr := Identity(r)
		if identityErr != nil || identity.UserID != 42 {
			t.Fatalf("identity not injected: %#v %v", identity, identityErr)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/private", nil)
	request.AddCookie(&http.Cookie{Name: HostSessionCookieName, Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("unexpected status %d", response.Code)
	}
}

func TestIssuedSessionCookieUsesBrowserSecurityFlags(t *testing.T) {
	authenticator := testAuthenticator(t)
	response := httptest.NewRecorder()
	if err := authenticator.IssueSession(response, entites.User{ID: 1, Username: "player"}); err != nil {
		t.Fatal(err)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one cookie, got %d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != HostSessionCookieName || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.MaxAge <= 0 {
		t.Fatalf("insecure session cookie: %#v", cookie)
	}
}

func TestRevokedSessionCannotBeReused(t *testing.T) {
	authenticator := testAuthenticator(t)
	token, err := authenticator.tokens.Issue(entites.User{ID: 42, Username: "player"})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := authenticator.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if err := authenticator.Revoke(identity); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/private", nil)
	request.AddCookie(&http.Cookie{Name: HostSessionCookieName, Value: token})
	response := httptest.NewRecorder()
	authenticator.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("revoked session reached protected handler")
	})).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session returned %d", response.Code)
	}
}

func TestRevocationSurvivesTokenExpirationLeeway(t *testing.T) {
	authenticator := testAuthenticator(t)
	base := time.Now().UTC()
	authenticator.revocations.now = func() time.Time { return base }
	identity := gameauth.Identity{TokenID: "token-id", ExpiresAt: base.Add(time.Second)}
	if err := authenticator.Revoke(identity); err != nil {
		t.Fatal(err)
	}
	authenticator.revocations.now = func() time.Time { return base.Add(20 * time.Second) }
	if !authenticator.revocations.isRevoked(identity.TokenID) {
		t.Fatal("revocation was dropped while JWT expiration leeway was still active")
	}
	authenticator.revocations.now = func() time.Time { return base.Add(32 * time.Second) }
	if authenticator.revocations.isRevoked(identity.TokenID) {
		t.Fatal("expired revocation entry was not purged")
	}
}

func TestRevocationSurvivesAuthenticatorRestartThroughStore(t *testing.T) {
	tokens, err := gameauth.NewTokenManager(controllerTestSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	store := &revocationStoreStub{tokens: make(map[string]time.Time)}
	first := NewAuthenticator(tokens, true, store)
	token, err := tokens.Issue(entites.User{ID: 42, Username: "player"})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := first.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Revoke(identity); err != nil {
		t.Fatal(err)
	}
	restarted := NewAuthenticator(tokens, true, store)
	if _, err := restarted.Verify(token); err == nil {
		t.Fatal("persisted revoked session became valid after authenticator restart")
	}
}

func TestClearSecureSessionAlsoExpiresLegacyCookieName(t *testing.T) {
	authenticator := testAuthenticator(t)
	response := httptest.NewRecorder()
	authenticator.ClearSession(response)
	cookies := response.Result().Cookies()
	if len(cookies) != 2 || cookies[0].Name != HostSessionCookieName || cookies[1].Name != SessionCookieName {
		t.Fatalf("cookie migration cleanup was incomplete: %#v", cookies)
	}
}

func TestUpdateRejectsClientSuppliedIDAndUsesContextIdentity(t *testing.T) {
	authenticator := testAuthenticator(t)
	controller := NewUserController(authenticator, nil)
	capture := &updateCapture{}
	handler := controller.UpdateUser(capture)

	unknownID := httptest.NewRequest(http.MethodPost, "/api/v1/user", strings.NewReader(`{"id":99,"fullName":"New Name","yearOfBirth":2000,"monthOfBirth":5,"dayOfBirth":10}`))
	unknownID.Header.Set("Content-Type", "application/json")
	unknownID = unknownID.WithContext(gameauth.WithIdentity(unknownID.Context(), gameauth.Identity{UserID: 42, Username: "player"}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, unknownID)
	if response.Code != http.StatusBadRequest || capture.called {
		t.Fatalf("client ID was not rejected: status=%d called=%v", response.Code, capture.called)
	}

	valid := httptest.NewRequest(http.MethodPost, "/api/v1/user", strings.NewReader(`{"fullName":"New Name","yearOfBirth":2000,"monthOfBirth":5,"dayOfBirth":10}`))
	valid.Header.Set("Content-Type", "application/json")
	valid = valid.WithContext(gameauth.WithIdentity(valid.Context(), gameauth.Identity{UserID: 42, Username: "player"}))
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, valid)
	if response.Code != http.StatusOK || !capture.called || capture.userID != 42 {
		t.Fatalf("authenticated ID was not used: status=%d capture=%#v", response.Code, capture)
	}
}
