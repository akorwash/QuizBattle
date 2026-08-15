// Package config owns all process configuration and validates it at startup.
package config

import (
	"crypto/sha256"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const compromisedJWTSecretSHA256 = "f873a0fad42aacdecfaafa1546f5c997b" + //gitleaks:allow -- non-secret SHA-256 denylist fingerprint
	"376d07a2b4cf2a6252cbfdc47bb837e"

type Config struct {
	Environment       string
	ReleaseSHA        string
	Port              string
	MongoURI          string
	MongoDatabase     string
	RedisAddress      string
	RedisUsername     string
	RedisPassword     string
	RedisTLS          bool
	JWTSecret         string
	SessionTTL        time.Duration
	CookieSecure      bool
	AllowedOrigins    []string
	TrustedProxyCIDRs []string
	SeedDatabase      bool
	QuestionBankPath  string
}

func Load() (Config, error) {
	environment := strings.ToLower(valueOrDefault("APP_ENV", "production"))
	cfg := Config{
		Environment:       environment,
		ReleaseSHA:        strings.TrimSpace(os.Getenv("RELEASE_SHA")),
		Port:              valueOrDefault("PORT", "8080"),
		MongoURI:          strings.TrimSpace(os.Getenv("MONGO_URI")),
		MongoDatabase:     firstNonEmpty("MONGO_DATABASE", "MongoDBName"),
		RedisAddress:      firstNonEmpty("REDIS_ADDRESS", "RedisEndPoint"),
		RedisUsername:     strings.TrimSpace(os.Getenv("REDIS_USERNAME")),
		RedisPassword:     firstNonEmpty("REDIS_PASSWORD", "RedisPassword"),
		RedisTLS:          parseBool(os.Getenv("REDIS_TLS")),
		JWTSecret:         strings.TrimSpace(os.Getenv("JWT_SECRET")),
		SessionTTL:        time.Hour,
		CookieSecure:      environment != "development" && environment != "test",
		AllowedOrigins:    splitCSV(os.Getenv("ALLOWED_ORIGINS")),
		TrustedProxyCIDRs: splitCSV(os.Getenv("TRUSTED_PROXY_CIDRS")),
		SeedDatabase:      parseBool(os.Getenv("SEED_DATABASE")),
		QuestionBankPath:  valueOrDefault("QUESTION_BANK_PATH", "../data/question-bank/questions.ar.jsonl"),
	}

	if cfg.MongoURI == "" {
		cfg.MongoURI = legacyMongoURI()
	}
	if cfg.MongoURI == "" {
		return Config{}, fmt.Errorf("MONGO_URI is required")
	}
	if cfg.MongoDatabase == "" {
		return Config{}, fmt.Errorf("MONGO_DATABASE is required")
	}
	if !validJWTSecret(cfg.JWTSecret) {
		return Config{}, fmt.Errorf("JWT_SECRET must be a deployment-unique random value of at least 32 characters")
	}
	port, err := strconv.Atoi(cfg.Port)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("PORT must be a number between 1 and 65535")
	}
	if raw := strings.TrimSpace(os.Getenv("SESSION_TTL")); raw != "" {
		ttl, err := time.ParseDuration(raw)
		if err != nil || ttl < 15*time.Minute || ttl > 24*time.Hour {
			return Config{}, fmt.Errorf("SESSION_TTL must be between 15m and 24h")
		}
		cfg.SessionTTL = ttl
	}
	if raw := strings.TrimSpace(os.Getenv("COOKIE_SECURE")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("COOKIE_SECURE must be true or false")
		}
		cfg.CookieSecure = value
	}
	productionLike := cfg.Environment != "development" && cfg.Environment != "test"
	if cfg.ReleaseSHA == "" && productionLike {
		return Config{}, fmt.Errorf("RELEASE_SHA is required outside development or test")
	}
	if cfg.ReleaseSHA != "" && !validReleaseSHA(cfg.ReleaseSHA) {
		return Config{}, fmt.Errorf("RELEASE_SHA must be a 40-character lowercase hexadecimal Git commit SHA")
	}
	if productionLike && !cfg.CookieSecure {
		return Config{}, fmt.Errorf("COOKIE_SECURE cannot be disabled outside development or test")
	}
	if productionLike && !mongoURIUsesTLS(cfg.MongoURI) {
		return Config{}, fmt.Errorf("MONGO_URI must enable TLS outside development or test")
	}
	if productionLike && cfg.RedisAddress != "" && !cfg.RedisTLS {
		return Config{}, fmt.Errorf("REDIS_TLS must be enabled outside development or test")
	}
	for _, origin := range cfg.AllowedOrigins {
		parsed, err := url.Parse(origin)
		if err != nil || parsed == nil {
			return Config{}, fmt.Errorf("ALLOWED_ORIGINS contains invalid origin %q", origin)
		}
		validScheme := parsed.Scheme == "https" || !productionLike && parsed.Scheme == "http"
		if !validScheme || parsed.Host == "" || parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
			return Config{}, fmt.Errorf("ALLOWED_ORIGINS contains invalid origin %q", origin)
		}
	}
	for _, network := range cfg.TrustedProxyCIDRs {
		_, parsedNetwork, err := net.ParseCIDR(network)
		if err != nil {
			return Config{}, fmt.Errorf("TRUSTED_PROXY_CIDRS contains invalid CIDR %q", network)
		}
		ones, bits := parsedNetwork.Mask.Size()
		if bits == 32 && ones < 8 || bits == 128 && ones < 32 {
			return Config{}, fmt.Errorf("TRUSTED_PROXY_CIDRS contains an excessively broad network")
		}
	}

	return cfg, nil
}

func validReleaseSHA(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
}

func validJWTSecret(secret string) bool {
	if len(secret) < 32 || strings.EqualFold(secret, "replace-this-with-at-least-32-random-characters") {
		return false
	}
	if fmt.Sprintf("%x", sha256.Sum256([]byte(secret))) == compromisedJWTSecretSHA256 {
		return false
	}
	unique := make(map[byte]struct{}, 16)
	for index := 0; index < len(secret); index++ {
		unique[secret[index]] = struct{}{}
	}
	if len(unique) < 8 {
		return false
	}
	for period := 1; period <= len(secret)/2; period++ {
		if len(secret)%period == 0 && strings.Repeat(secret[:period], len(secret)/period) == secret {
			return false
		}
	}
	return true
}

func mongoURIUsesTLS(uri string) bool {
	parsed, err := url.Parse(uri)
	if err != nil {
		return false
	}
	decodedQuery, err := url.QueryUnescape(parsed.RawQuery)
	if err != nil || strings.Contains(decodedQuery, ";") {
		// The MongoDB driver treats semicolons as option separators while
		// net/url does not parse them consistently. Reject the ambiguous form.
		return false
	}
	query := parsed.Query()
	for name, values := range query {
		normalized := strings.ToLower(name)
		for _, value := range values {
			enabled, parseErr := strconv.ParseBool(value)
			if parseErr != nil {
				continue
			}
			if (normalized == "tls" || normalized == "ssl") && !enabled {
				return false
			}
			if enabled && (normalized == "tlsinsecure" || normalized == "sslinsecure" || normalized == "tlsallowinvalidcertificates" || normalized == "tlsallowinvalidhostnames" || normalized == "sslinvalidhostnameallowed" || normalized == "tlsdisableocspendpointcheck") {
				return false
			}
		}
	}
	if strings.EqualFold(parsed.Scheme, "mongodb+srv") {
		return true
	}
	for _, name := range []string{"tls", "ssl"} {
		value := query.Get(name)
		enabled, err := strconv.ParseBool(value)
		if err == nil && enabled {
			return true
		}
	}
	return false
}

func legacyMongoURI() string {
	host := firstNonEmpty("MONGO_HOST", "MongoHostID")
	port := firstNonEmpty("MONGO_PORT", "MongoPORT")
	username := firstNonEmpty("MONGO_USERNAME", "MongoUsername")
	password := firstNonEmpty("MONGO_PASSWORD", "MongoPassword")
	if host == "" {
		return ""
	}
	if port != "" {
		host = net.JoinHostPort(host, port)
	}
	u := &url.URL{Scheme: "mongodb", Host: host}
	if username != "" {
		u.User = url.UserPassword(username, password)
	}
	return u.String()
}

func firstNonEmpty(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func valueOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func splitCSV(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, strings.TrimSuffix(item, "/"))
		}
	}
	return values
}

func parseBool(value string) bool {
	parsed, _ := strconv.ParseBool(strings.TrimSpace(value))
	return parsed
}
