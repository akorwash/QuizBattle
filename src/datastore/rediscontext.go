package datastore

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisConfiguration config of redis
type RedisConfiguration struct {
	EndPoint string
	Username string
	Password string
	UseTLS   bool
}

// GetRedisContext get the redis context
func GetRedisContext(ctx context.Context, config RedisConfiguration) (*redis.Client, error) {
	if config.EndPoint == "" {
		return nil, fmt.Errorf("Redis endpoint is required")
	}
	options := &redis.Options{
		Addr:         config.EndPoint,
		Username:     config.Username,
		Password:     config.Password,
		DB:           0,
		MaxRetries:   3,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	if config.UseTLS {
		options.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	client := redis.NewClient(options)

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect to Redis: %w", err)
	}
	return client, nil
}
