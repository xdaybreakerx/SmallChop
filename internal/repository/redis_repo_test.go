package repository

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

var ctx = context.Background()

// Create a mock Redis client using miniredis
func createMockRedis() (*redis.Client, *miniredis.Miniredis) {
	// Start a mock Redis server
	mockRedis, err := miniredis.Run()
	if err != nil {
		panic("Unable to start mock redis server")
	}

	// Create a Redis client connected to the mock Redis server
	rdb := redis.NewClient(&redis.Options{
		Addr: mockRedis.Addr(),
	})

	return rdb, mockRedis
}

// Test for SetKey: ensure that a key can be set in Redis and no errors are returned.
func TestSetKey(t *testing.T) {
	// Create mock Redis client and server
	rdb, mockRedis := createMockRedis()
	defer mockRedis.Close()

	// Set a test key-value pair in Redis
	key := "shortURL123"
	value := "http://example.com"

	// Act: Set the key in Redis
	SetKey(ctx, rdb, key, value, 0)

	// Assert: Check if the value was set correctly in the mock Redis
	storedValue, err := mockRedis.Get(key)
	if err != nil {
		t.Fatalf("Failed to get key from mock Redis: %v", err)
	}
	if storedValue != value {
		t.Errorf("Expected %s, got %s", value, storedValue)
	}
}

// Test for GetLongURL: ensure that a stored key-value pair can be retrieved successfully.
func TestGetLongURL(t *testing.T) {
	// Create mock Redis client and server
	rdb, mockRedis := createMockRedis()
	defer mockRedis.Close()

	// Set a test key-value pair in Redis (using miniredis directly)
	key := "shortURL123"
	value := "http://example.com"
	mockRedis.Set(key, value)

	// Check the error return value of mockRedis.Set
	if err := mockRedis.Set(key, value); err != nil {
		t.Fatalf("Failed to set key in mock Redis: %v", err)
	}

	// Act: Try to retrieve the key from Redis
	retrievedValue, err := GetLongURL(ctx, rdb, key)
	if err != nil {
		t.Fatalf("Failed to retrieve key from Redis: %v", err)
	}

	// Assert: Check if the retrieved value matches the stored value
	if retrievedValue != value {
		t.Errorf("Expected %s, got %s", value, retrievedValue)
	}
}

// Test for GetLongURL: check behavior when key doesn't exist
func TestGetLongURL_NotFound(t *testing.T) {
	// Create mock Redis client and server
	rdb, mockRedis := createMockRedis()
	defer mockRedis.Close()

	// Try to get a key that doesn't exist
	nonExistentKey := "nonExistentKey"
	_, err := GetLongURL(ctx, rdb, nonExistentKey)

	// Assert: Check if the appropriate error is returned
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
	expectedError := "short URL not found"
	if err.Error() != expectedError {
		t.Errorf("Expected error %s, got %v", expectedError, err)
	}
}
