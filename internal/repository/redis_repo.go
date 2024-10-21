package repository

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
)

type RedisRepo struct {
	client *redis.Client
}

// Initialize a new instance of the RedisRepo struct
func NewRedisRepo() *RedisRepo {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Docker
	})
	return &RedisRepo{client: rdb}
}

// SetKey stores the short URL and original URL mapping to Redis
// Accepts context.Context by value, not by pointer
func SetKey(ctx context.Context, rdb *redis.Client, key string, value string, ttl int) {
	// We set the key-value pair in Redis with no expiration (ttl=0)
	fmt.Println("Setting key", key, "to", value, "in Redis")
	rdb.Set(ctx, key, value, 0)
	fmt.Println("The key", key, "has been set to", value, "successfully")
}

// GetLongURL retrieves the original URL from Redis based on the given short URL
// Accepts context.Context by value, not by pointer
func GetLongURL(ctx context.Context, rdb *redis.Client, shortURL string) (string, error) {
	longURL, err := rdb.Get(ctx, shortURL).Result()

	if err == redis.Nil {
		return "", fmt.Errorf("short URL not found")
	} else if err != nil {
		return "", fmt.Errorf("failed to retrieve from Redis: %v", err)
	}

	return longURL, nil
}
