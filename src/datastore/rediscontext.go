package datastore

import (
	"fmt"
	"time"

	"github.com/go-redis/redis"
)

var cashStatus = false

//CheckCashingStatus get the status of cashing system
func CheckCashingStatus() bool {
	return cashStatus
}

//RedisConfiguration config of redis
type RedisConfiguration struct {
	EndPoint string
	Password string
}

//GetRedisContext get the redis context
func GetRedisContext(config RedisConfiguration) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:        config.EndPoint,
		Password:    config.Password,
		DB:          0,
		MaxRetries:  5,
		ReadTimeout: time.Minute,
	})

	_, err := client.Ping().Result()
	cashStatus = (err == nil)
	if !cashStatus {
		fmt.Println("faild to get redis context, ", err.Error())
	}

	return client, nil
}
