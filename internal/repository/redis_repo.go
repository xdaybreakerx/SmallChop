package repository

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
)

type RedisRepo struct {
	client *redis.Client
}

// construction function to initialise a new instance of the RedisRepo struct
func NewRedisRepo() *RedisRepo {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Docker
	})
	return &RedisRepo{client: rdb}
}

// takes a short URL and its corresponding original URL as input, and saves the mapping to Redis
func SetKey(ctx *context.Context, rdb *redis.Client, key string, value string, ttl int) {
	// We set the key value pair in Redis, we use the context defined in main by reference and a TTL of 0 (no expiration)
	fmt.Println("Setting key", key, "to", value, "in Redis")

	rdb.Set(*ctx, key, value, 0)

	fmt.Println("The key", key, "has been set to", value, "successfully")
}

// retrieves and returns the original URL corresponding to the given short URL from Redis
func GetLongURL(ctx *context.Context, rdb *redis.Client, shortURL string) (string, error) {

	longURL, err := rdb.Get(*ctx, shortURL).Result()

	if err == redis.Nil {
		return "", fmt.Errorf("short URL not found")

	} else if err != nil {
		return "", fmt.Errorf("failed to retrieve from Redis: %v", err)

	}

	return longURL, nil
}
