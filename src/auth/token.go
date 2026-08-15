package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/golang-jwt/jwt/v5"
)

const (
	issuer                = "quizbattle"
	audience              = "quizbattle-web"
	TokenValidationLeeway = 30 * time.Second
	MaxTokenLength        = 8 << 10
)

var ErrInvalidToken = errors.New("invalid access token")

type Identity struct {
	UserID    int64
	Username  string
	FullName  string
	TokenID   string
	ExpiresAt time.Time
}

type Claims struct {
	Username string `json:"username"`
	FullName string `json:"fullName"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	key []byte
	ttl time.Duration
	now func() time.Time
}

func NewTokenManager(secret string, ttl time.Duration) (*TokenManager, error) {
	secret = strings.TrimSpace(secret)
	if len(secret) < 32 {
		return nil, fmt.Errorf("JWT secret must contain at least 32 characters")
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("token lifetime must be positive")
	}
	return &TokenManager{key: []byte(secret), ttl: ttl, now: time.Now}, nil
}

func (manager *TokenManager) Issue(user entites.User) (string, error) {
	if user.ID <= 0 {
		return "", fmt.Errorf("cannot issue a token without a valid user ID")
	}
	now := manager.now().UTC()
	tokenID, err := randomTokenID()
	if err != nil {
		return "", err
	}
	claims := Claims{
		Username: user.Username,
		FullName: user.Fullname,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   strconv.FormatInt(user.ID, 10),
			Audience:  jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(manager.ttl)),
			NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        tokenID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(manager.key)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return signed, nil
}

func (manager *TokenManager) Verify(raw string) (Identity, error) {
	if strings.TrimSpace(raw) == "" || len(raw) > MaxTokenLength {
		return Identity{}, ErrInvalidToken
	}
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(
		raw,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, ErrInvalidToken
			}
			return manager.key, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(manager.now),
		jwt.WithLeeway(TokenValidationLeeway),
	)
	if err != nil || !token.Valid {
		return Identity{}, ErrInvalidToken
	}
	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || userID <= 0 || claims.Username == "" || len(claims.Username) > 128 || len(claims.FullName) > 320 {
		return Identity{}, ErrInvalidToken
	}
	if claims.ExpiresAt == nil || claims.IssuedAt == nil || claims.NotBefore == nil {
		return Identity{}, ErrInvalidToken
	}
	decodedTokenID, err := hex.DecodeString(claims.ID)
	if err != nil || len(decodedTokenID) != 16 || hex.EncodeToString(decodedTokenID) != claims.ID || claims.ExpiresAt.Time.Before(claims.IssuedAt.Time) || claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time) > manager.ttl+TokenValidationLeeway {
		return Identity{}, ErrInvalidToken
	}
	return Identity{
		UserID:    userID,
		Username:  claims.Username,
		FullName:  claims.FullName,
		TokenID:   claims.ID,
		ExpiresAt: claims.ExpiresAt.Time,
	}, nil
}

func (manager *TokenManager) TTL() time.Duration {
	return manager.ttl
}

func randomTokenID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate token ID: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

type identityContextKey struct{}

func WithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, identityContextKey{}, identity)
}

func IdentityFromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityContextKey{}).(Identity)
	return identity, ok && identity.UserID > 0
}
