package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"

	"gochop-it/internal/utils"
)

// MockMongoRepo implements URLRepository for testing purposes
type MockMongoRepo struct{}

// Ensure MockMongoRepo implements URLRepository
var _ URLRepository = (*MockMongoRepo)(nil)

func (m *MockMongoRepo) FindURLByID(ctx context.Context, id int64) (*URL, error) {
	return &URL{
		ID:      id,
		LongURL: "https://example.com",
	}, nil
}

func (m *MockMongoRepo) IncrementAccessCount(ctx context.Context, id int64) error {
	return nil
}

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
	key := utils.Encode(12345) // Encoded short code
	value := "https://example.com"

	// Act: Set the key in Redis using RedisRepo
	err := redisRepo.SetKey(ctx, key, value, 0)
	if err != nil {
		t.Errorf("Failed to set key %s: %v", key, err)
	}

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
	key := utils.Encode(12345) // Encoded short code
	value := "https://example.com"

	// Check the error return value of mockRedis.Set
	if err := mock.Set(key, value); err != nil {
		t.Fatalf("Failed to set key in mock Redis: %v", err)
	}

	// Act: Try to retrieve the key from Redis
	longURL, err := redisRepo.GetLongURL(ctx, key, nil, 10*time.Minute)
	if err != nil {
		t.Fatalf("Failed to retrieve key from Redis: %v", err)
	}

	// Assert: Check if the retrieved value matches the stored value
	if longURL != value {
		t.Errorf("Expected %s, got %s", value, longURL)
	}
}

// TestGetLongURLMiss tests the GetLongURL function when the key is not found in Redis and must be fetched from MongoDB
func TestGetLongURLMiss(t *testing.T) {
	// Create a context for Redis operations
	ctx := context.TODO()
	// Create mock Redis Client and miniredis for testing
	rdb, mock := createMockRedis()
	// Initialize RedisRepo with the mock Redis Client
	redisRepo := &RedisRepo{Client: rdb}

	key := utils.Encode(12345) // Encoded short code
	expectedURL := "https://example.com"

	// Ensure the key does not exist in Redis
	if mock.Exists(key) {
		t.Fatalf("Key %s should not exist in Redis", key)
	}

	// Create an instance of MockMongoRepo
	mongoRepo := &MockMongoRepo{}

	// Act: Try to retrieve the key from Redis (will miss and fetch from MongoDB)
	longURL, err := redisRepo.GetLongURL(ctx, key, mongoRepo, 10*time.Minute)
	if err != nil {
		t.Fatalf("Failed to retrieve key from Redis: %v", err)
	}

	// Assert: Check if the retrieved value matches the expected value
	if longURL != expectedURL {
		t.Errorf("Expected %s, got %s", expectedURL, longURL)
	}

	// Verify that the key is now set in Redis
	storedValue, err := mock.Get(key)
	if err != nil {
		t.Fatalf("Failed to get key from mock Redis: %v", err)
	}
	if storedValue != expectedURL {
		t.Errorf("Expected %s in Redis, got %s", expectedURL, storedValue)
	}
}
