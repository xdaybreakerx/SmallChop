package utils

import (
	"encoding/base64"
	"testing"
)

// Test that GetShortCode returns a non-empty result and that two results are different
func TestGetShortCode(t *testing.T) {
	shortCode1 := GetShortCode()
	shortCode2 := GetShortCode()

	// Assert: Check if the shortCode is not empty
	if shortCode1 == "" {
		t.Errorf("Expected non-empty short code, got empty")
	}

	// Assert: Ensure two subsequent short codes are different
	if shortCode1 == shortCode2 {
		t.Errorf("Expected different short codes, got the same")
	}
}

// Test that GetShortCode returns a string of expected length
func TestGetShortCodeLength(t *testing.T) {
	shortCode := GetShortCode()

	expectedLength := len(shortCode)

	if len(shortCode) != expectedLength {
		t.Errorf("Expected length %d, but got %d", expectedLength, len(shortCode))
	}
}

// Test that GetShortCode generates a valid base64 string
func TestGetShortCodeBase64(t *testing.T) {
	shortCode := GetShortCode()

	// Ensure there are no invalid characters in the short code
	if _, err := base64.StdEncoding.DecodeString(shortCode + "=="); err != nil {
		t.Errorf("Generated short code is not valid base64: %v", err)
	}
}
