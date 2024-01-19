package storage

import (
	"fmt"

	"github.com/redis/go-redis/v9"
)

func RedisClient(address string, port int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", address, port),
		Password: "",
		DB:       0, // use default DB
	})
}
