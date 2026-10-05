package repository

import (
	"context"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisRepo struct {
	Client *redis.Client
}

func NewRedisRepo(timeout time.Duration) *RedisRepo {
	return &RedisRepo{Client: redis.NewClient(redisOptions(timeout))}
}

func redisOptions(timeout time.Duration) *redis.Options {
	return &redis.Options{
		Addr:                  "redis:6379",
		Password:              os.Getenv("REDIS_PASSWORD"),
		DialTimeout:           timeout,
		ReadTimeout:           timeout,
		WriteTimeout:          timeout,
		PoolTimeout:           timeout,
		ContextTimeoutEnabled: true,
		MaxRetries:            -1,
	}
}

func (r *RedisRepo) SetKey(ctx context.Context, key, value string, ttl time.Duration) error {
	return r.Client.Set(ctx, key, value, ttl).Err()
}

// GetLongURL reads only the cache. The caller owns database fallback and cache fill.
func (r *RedisRepo) GetLongURL(ctx context.Context, shortCode string) (string, error) {
	return r.Client.Get(ctx, shortCode).Result()
}

func (r *RedisRepo) Ping(ctx context.Context) error {
	return r.Client.Ping(ctx).Err()
}
