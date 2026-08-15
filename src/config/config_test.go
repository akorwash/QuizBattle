package config

import (
	"testing"
	"time"
)

const validTestJWTSecret = "test-only-7X!f9Q#v2L@z6N$k3P&c8R*m"
const validTestReleaseSHA = "0123456789abcdef0123456789abcdef01234567"

var configEnvironmentNames = []string{
	"APP_ENV", "RELEASE_SHA", "PORT", "MONGO_URI", "MONGO_DATABASE", "MongoDBName",
	"REDIS_ADDRESS", "RedisEndPoint", "REDIS_PASSWORD", "RedisPassword",
	"REDIS_USERNAME", "REDIS_TLS",
	"JWT_SECRET", "ACCESS_SECRET", "SESSION_TTL", "COOKIE_SECURE",
	"ALLOWED_ORIGINS", "SEED_DATABASE", "MONGO_HOST", "MongoHostID",
	"TRUSTED_PROXY_CIDRS",
	"MONGO_PORT", "MongoPORT", "MONGO_USERNAME", "MongoUsername",
	"MONGO_PASSWORD", "MongoPassword",
}

func clearConfigEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range configEnvironmentNames {
		t.Setenv(name, "")
	}
}

func TestLoadValidatesAndNormalizesSecurityConfiguration(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("RELEASE_SHA", validTestReleaseSHA)
	t.Setenv("MONGO_URI", "mongodb://localhost:27017/?tls=true")
	t.Setenv("MONGO_DATABASE", "quizbattle")
	t.Setenv("JWT_SECRET", validTestJWTSecret)
	t.Setenv("SESSION_TTL", "2h")
	t.Setenv("ALLOWED_ORIGINS", "https://one.example/, https://two.example")
	t.Setenv("SEED_DATABASE", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ReleaseSHA != validTestReleaseSHA || !cfg.CookieSecure || cfg.SessionTTL != 2*time.Hour || !cfg.SeedDatabase {
		t.Fatalf("unexpected security configuration: %#v", cfg)
	}
	if len(cfg.AllowedOrigins) != 2 || cfg.AllowedOrigins[0] != "https://one.example" {
		t.Fatalf("origins were not normalized: %#v", cfg.AllowedOrigins)
	}
}

func TestLoadValidatesReleaseIdentity(t *testing.T) {
	t.Run("development may omit release", func(t *testing.T) {
		clearConfigEnvironment(t)
		t.Setenv("APP_ENV", "development")
		t.Setenv("MONGO_URI", "mongodb://localhost:27017")
		t.Setenv("MONGO_DATABASE", "quizbattle")
		t.Setenv("JWT_SECRET", validTestJWTSecret)

		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.ReleaseSHA != "" {
			t.Fatalf("unexpected development release identity: %q", cfg.ReleaseSHA)
		}
	})

	t.Run("production requires release", func(t *testing.T) {
		clearConfigEnvironment(t)
		t.Setenv("APP_ENV", "production")
		t.Setenv("MONGO_URI", "mongodb://localhost:27017/?tls=true")
		t.Setenv("MONGO_DATABASE", "quizbattle")
		t.Setenv("JWT_SECRET", validTestJWTSecret)

		if _, err := Load(); err == nil {
			t.Fatal("production configuration without RELEASE_SHA was accepted")
		}
	})

	for name, value := range map[string]string{
		"short":     "01234567",
		"uppercase": "0123456789abcdef0123456789abcdef0123456A",
		"non-hex":   "0123456789abcdef0123456789abcdef0123456g",
	} {
		t.Run(name, func(t *testing.T) {
			clearConfigEnvironment(t)
			t.Setenv("APP_ENV", "development")
			t.Setenv("RELEASE_SHA", value)
			t.Setenv("MONGO_URI", "mongodb://localhost:27017")
			t.Setenv("MONGO_DATABASE", "quizbattle")
			t.Setenv("JWT_SECRET", validTestJWTSecret)

			if _, err := Load(); err == nil {
				t.Fatalf("invalid RELEASE_SHA %q was accepted", value)
			}
		})
	}
}

func TestLoadRejectsUnsafeOriginsAndUniversalTrustedProxy(t *testing.T) {
	for name, value := range map[string]string{
		"origin": "null",
		"proxy":  "0.0.0.0/0",
	} {
		t.Run(name, func(t *testing.T) {
			clearConfigEnvironment(t)
			t.Setenv("APP_ENV", "production")
			t.Setenv("RELEASE_SHA", validTestReleaseSHA)
			t.Setenv("MONGO_URI", "mongodb://localhost:27017/?tls=true")
			t.Setenv("MONGO_DATABASE", "quizbattle")
			t.Setenv("JWT_SECRET", validTestJWTSecret)
			if name == "origin" {
				t.Setenv("ALLOWED_ORIGINS", value)
			} else {
				t.Setenv("TRUSTED_PROXY_CIDRS", value)
			}
			if _, err := Load(); err == nil {
				t.Fatalf("unsafe %s configuration was accepted", name)
			}
		})
	}
}

func TestLoadRejectsWeakSecretAndInvalidPort(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("MONGO_URI", "mongodb://localhost:27017")
	t.Setenv("MONGO_DATABASE", "quizbattle")
	t.Setenv("JWT_SECRET", "short")
	if _, err := Load(); err == nil {
		t.Fatal("weak JWT secret was accepted")
	}

	t.Setenv("JWT_SECRET", validTestJWTSecret)
	t.Setenv("PORT", "70000")
	if _, err := Load(); err == nil {
		t.Fatal("invalid port was accepted")
	}
}

func TestLoadRejectsLegacySecretVariableAndLowDiversity(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("APP_ENV", "development")
	t.Setenv("MONGO_URI", "mongodb://localhost:27017")
	t.Setenv("MONGO_DATABASE", "quizbattle")
	t.Setenv("ACCESS_SECRET", validTestJWTSecret)
	if _, err := Load(); err == nil {
		t.Fatal("legacy ACCESS_SECRET fallback was accepted")
	}

	t.Setenv("JWT_SECRET", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if _, err := Load(); err == nil {
		t.Fatal("low-diversity JWT secret was accepted")
	}
}

func TestLoadRejectsDocumentedPlaceholderSecret(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("APP_ENV", "development")
	t.Setenv("MONGO_URI", "mongodb://localhost:27017")
	t.Setenv("MONGO_DATABASE", "quizbattle")
	t.Setenv("JWT_SECRET", "replace-this-with-at-least-32-random-characters")
	if _, err := Load(); err == nil {
		t.Fatal("public example JWT secret was accepted")
	}
}

func TestLoadRejectsInsecureProductionCookie(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("RELEASE_SHA", validTestReleaseSHA)
	t.Setenv("MONGO_URI", "mongodb://localhost:27017")
	t.Setenv("MONGO_DATABASE", "quizbattle")
	t.Setenv("JWT_SECRET", validTestJWTSecret)
	t.Setenv("COOKIE_SECURE", "false")
	if _, err := Load(); err == nil {
		t.Fatal("insecure production cookie was accepted")
	}
}

func TestLoadRejectsInsecureProductionLikeEnvironment(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("APP_ENV", "staging")
	t.Setenv("RELEASE_SHA", validTestReleaseSHA)
	t.Setenv("MONGO_URI", "mongodb://localhost:27017/?tls=true")
	t.Setenv("MONGO_DATABASE", "quizbattle")
	t.Setenv("JWT_SECRET", validTestJWTSecret)
	t.Setenv("COOKIE_SECURE", "false")
	if _, err := Load(); err == nil {
		t.Fatal("staging environment accepted insecure cookies")
	}
}

func TestMongoTLSValidationRejectsExplicitOrCertificateBypasses(t *testing.T) {
	for _, uri := range []string{
		"mongodb+srv://cluster.example/db?tls=false",
		"mongodb://cluster.example/db?tls=true&tlsInsecure=true",
		"mongodb://cluster.example/db?ssl=true&tlsAllowInvalidCertificates=true",
		"mongodb://cluster.example/db?ssl=true&sslInsecure=true",
		"mongodb://cluster.example/db?tls=true&tlsDisableOCSPEndpointCheck=true",
		"mongodb+srv://cluster.example/db?authSource=admin;tls=false",
		"mongodb+srv://cluster.example/db?authSource=admin%3BsslInsecure=true",
	} {
		if mongoURIUsesTLS(uri) {
			t.Fatalf("insecure MongoDB URI passed TLS validation: %s", uri)
		}
	}
	for _, uri := range []string{
		"mongodb+srv://cluster.example/db",
		"mongodb://cluster.example/db?tls=true",
	} {
		if !mongoURIUsesTLS(uri) {
			t.Fatalf("secure MongoDB URI failed TLS validation: %s", uri)
		}
	}
}
