package repository

import (
	"context"
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
func (r *RedisRepo) SaveURL(shortURL, originalURL string) error {
	return r.client.Set(context.Background(), shortURL, originalURL, 0).Err()
}

// retrieves and returns the original URL corresponding to the given short URL from Redis
func (r *RedisRepo) GetURL(shortURL string) (string, error) {
	return r.client.Get(context.Background(), shortURL).Result()
}
