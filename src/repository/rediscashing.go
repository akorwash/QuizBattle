package repository

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCashingRepository struct {
	client *redis.Client
}

func NewRedisCashingRepository(client *redis.Client) *RedisCashingRepository {
	return &RedisCashingRepository{client: client}
}

func (repository *RedisCashingRepository) SetString(ctx context.Context, key, value string, expiration time.Duration) error {
	return repository.client.Set(ctx, key, value, expiration).Err()
}

func (repository *RedisCashingRepository) SetByte(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	return repository.client.Set(ctx, key, value, expiration).Err()
}

func (repository *RedisCashingRepository) Get(ctx context.Context, key string) (string, error) {
	return repository.client.Get(ctx, key).Result()
}
