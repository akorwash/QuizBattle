package auth

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "0123456789abcdef0123456789abcdef" //gitleaks:allow -- deterministic test fixture

func TestTokenRoundTripUsesMinimalClaims(t *testing.T) {
	manager, err := NewTokenManager(testSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	token, err := manager.Issue(entites.User{ID: 42, Username: "player", Fullname: "Player One", Email: "private@example.com", MobileNumber: "01000000000"})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := manager.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if identity.UserID != 42 || identity.Username != "player" || identity.FullName != "Player One" || identity.TokenID == "" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
	parts := strings.Split(token, ".")
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "private@example.com") || strings.Contains(string(payload), "01000000000") {
		t.Fatalf("token leaked private account data: %s", payload)
	}
}

func TestTokenRejectsExpiredAndWrongSecret(t *testing.T) {
	manager, _ := NewTokenManager(testSecret, time.Minute)
	issuedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return issuedAt }
	token, err := manager.Issue(entites.User{ID: 1, Username: "player"})
	if err != nil {
		t.Fatal(err)
	}
	manager.now = func() time.Time { return issuedAt.Add(2 * time.Minute) }
	if _, err := manager.Verify(token); err == nil {
		t.Fatal("expected expired token to be rejected")
	}

	other, _ := NewTokenManager(strings.Repeat("x", 32), time.Hour)
	other.now = func() time.Time { return issuedAt.Add(30 * time.Second) }
	if _, err := other.Verify(token); err == nil {
		t.Fatal("expected token signed with another key to be rejected")
	}
}

func TestTokenManagerValidation(t *testing.T) {
	if _, err := NewTokenManager("short", time.Hour); err == nil {
		t.Fatal("expected a short JWT secret to be rejected")
	}
	manager, _ := NewTokenManager(testSecret, time.Hour)
	if _, err := manager.Issue(entites.User{}); err == nil {
		t.Fatal("expected user ID zero to be rejected")
	}
	if _, err := manager.Verify(strings.Repeat("a", MaxTokenLength+1)); err == nil {
		t.Fatal("oversized token was accepted")
	}
}

func TestTokenRejectsMissingOrOutOfContractRegisteredClaims(t *testing.T) {
	manager, _ := NewTokenManager(testSecret, time.Hour)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	valid := jwt.RegisteredClaims{
		Issuer:    issuer,
		Subject:   "42",
		Audience:  jwt.ClaimStrings{audience},
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		NotBefore: jwt.NewNumericDate(now.Add(-time.Second)),
		IssuedAt:  jwt.NewNumericDate(now),
		ID:        "0123456789abcdef0123456789abcdef",
	}
	tests := map[string]jwt.RegisteredClaims{
		"missing issued-at": func() jwt.RegisteredClaims {
			claims := valid
			claims.IssuedAt = nil
			return claims
		}(),
		"malformed token id": func() jwt.RegisteredClaims {
			claims := valid
			claims.ID = "not-a-random-jti"
			return claims
		}(),
		"excessive lifetime": func() jwt.RegisteredClaims {
			claims := valid
			claims.ExpiresAt = jwt.NewNumericDate(now.Add(2 * time.Hour))
			return claims
		}(),
	}
	for name, registered := range tests {
		t.Run(name, func(t *testing.T) {
			raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{Username: "player", RegisteredClaims: registered}).SignedString(manager.key)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := manager.Verify(raw); err == nil {
				t.Fatal("out-of-contract token was accepted")
			}
		})
	}
}
