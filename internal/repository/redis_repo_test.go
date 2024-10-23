package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

// Create a mock Redis Client using miniredis
func createMockRedis() (*redis.Client, *miniredis.Miniredis) {
	// Start a mock Redis server
	mockRedis, err := miniredis.Run()
	if err != nil {
		panic("Unable to start mock redis server")
	}

	// Create a Redis Client connected to the mock Redis server
	rdb := redis.NewClient(&redis.Options{
		Addr: mockRedis.Addr(),
	})

	return rdb, mockRedis
}

// Test for SetKey: ensure that a key can be set in Redis and no errors are returned.
func TestSetKey(t *testing.T) {
	// Create a context for Redis operations
	ctx := context.TODO()
	// Create mock Redis Client and miniredis for testing
	rdb, mock := createMockRedis()
	// Initialize RedisRepo with the mock Redis Client
	redisRepo := &RedisRepo{Client: rdb}

	// Set a test key-value pair in Redis
	key := "shortURL123"
	value := "http://example.com"

	// Act: Set the key in Redis using RedisRepo
	redisRepo.SetKey(ctx, key, value, 0)

	storedValue, err := mock.Get(key)
	if err != nil {
		t.Fatalf("Failed to get key from mock Redis: %v", err)
	}
	if storedValue != value {
		t.Errorf("Expected %s, got %s", value, storedValue)
	}
}

// Test for GetLongURL: ensure that a stored key-value pair can be retrieved successfully.
func TestGetLongURL(t *testing.T) {
	// Create a context for Redis operations
	ctx := context.TODO()
	// Create mock Redis Client and miniredis for testing
	rdb, mock := createMockRedis()
	// Initialize RedisRepo with the mock Redis Client
	redisRepo := &RedisRepo{Client: rdb}

	// Set a test key-value pair in Redis (using miniredis directly)
	key := "shortURL123"
	value := "http://example.com"

	// Check the error return value of mockRedis.Set
	if err := mock.Set(key, value); err != nil {
		t.Fatalf("Failed to set key in mock Redis: %v", err)
	}

	// Act: Try to retrieve the key from Redis
	retrievedValue, err := redisRepo.GetLongURL(ctx, nil, key, 10*time.Minute)
	if err != nil {
		t.Fatalf("Failed to retrieve key from Redis: %v", err)
	}

	// Assert: Check if the retrieved value matches the stored value
	if retrievedValue != value {
		t.Errorf("Expected %s, got %s", value, retrievedValue)
	}
}
