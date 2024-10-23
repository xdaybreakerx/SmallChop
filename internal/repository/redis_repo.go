package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisRepo struct {
	Client *redis.Client
}

// Initialize a new instance of the RedisRepo struct
func NewRedisRepo() *RedisRepo {
	rdb := redis.NewClient(&redis.Options{
		Addr: "redis:6379", // Docker
	})
	return &RedisRepo{Client: rdb}
}

// SetKey stores the short URL and original URL mapping to Redis
func (r *RedisRepo) SetKey(ctx context.Context, key string, value string, ttl time.Duration) error {
	fmt.Println("Setting key", key, "to", value, "in Redis")
	err := r.Client.Set(ctx, key, value, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set key in Redis: %w", err)
	}
	fmt.Println("The key", key, "has been set to", value, "successfully")
	return nil
}

// GetLongURL retrieves the original URL from Redis based on the given short URL
// If not found, it lazy-loads from MongoDB and stores it in Redis
func (r *RedisRepo) GetLongURL(ctx context.Context, mongoRepo *MongoRepo, shortURL string, ttl time.Duration) (string, error) {
	// Try to get the long URL from Redis first
	longURL, err := r.Client.Get(ctx, shortURL).Result()
	if err == redis.Nil {
		// If not found in Redis, lazy-load from MongoDB
		urlDoc, err := mongoRepo.FindURL(ctx, shortURL)
		if err != nil {
			return "", fmt.Errorf("short URL not found in MongoDB: %w", err)
		}

		// Store the long URL in Redis with a TTL for future requests
		err = r.SetKey(ctx, shortURL, urlDoc.LongURL, ttl)
		if err != nil {
			return "", err
		}

		return urlDoc.LongURL, nil
	} else if err != nil {
		return "", fmt.Errorf("failed to retrieve from Redis: %v", err)
	}

	// Return the long URL if found in Redis
	return longURL, nil
}

// Ping tests the Redis connection
func (r *RedisRepo) Ping(ctx context.Context) error {
	_, err := r.Client.Ping(ctx).Result()
	return err
}
