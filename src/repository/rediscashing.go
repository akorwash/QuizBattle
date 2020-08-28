package repository

import (
	"time"

	"github.com/akorwash/QuizBattle/datastore"
	"github.com/go-redis/redis"
)

//RedisCashingRepository repo to redis cashing
type RedisCashingRepository struct {
	context *redis.Client
}

//NewRedisCashingRepository ctor for RedisCashingRepository
func NewRedisCashingRepository(config datastore.RedisConfiguration) (*RedisCashingRepository, error) {
	context, err := datastore.GetRedisContext(config)
	if err != nil {
		println("Error while get redis context: %v\n", err)
		return nil, err
	}
	repo := RedisCashingRepository{}
	repo.context = context
	return &repo, nil
}

//SetString set string ket value
func (repos *RedisCashingRepository) SetString(key string, value string, expiration time.Duration) error {
	return repos.context.Set(key, value, expiration).Err()
}

//SetByte set object
func (repos *RedisCashingRepository) SetByte(key string, value []byte, expiration time.Duration) error {
	return repos.context.Set(key, value, expiration).Err()
}

//Get get value by key
func (repos *RedisCashingRepository) Get(key string) (string, error) {
	return repos.context.Get(key).Result()
}
