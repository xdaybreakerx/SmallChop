package middleware

import (
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Mock handler to wrap with the rate limiter
func mockHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		log.Printf("Error writing response: %v", err)
	}
}

// Test if the rate limiter allows requests within the limit
func TestRateLimiter_AllowsRequests(t *testing.T) {
	// Wrap the mock handler with the rate limiter
	limiter := PerClientRateLimiter(mockHandler)

	// Create a test HTTP server
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	w := httptest.NewRecorder()

	// Perform a request within the rate limit
	for i := 0; i < 2; i++ {
		limiter.ServeHTTP(w, req)
		if w.Result().StatusCode != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Result().StatusCode)
		}
	}
}

// Test if the rate limiter rejects requests when the limit is exceeded
func TestRateLimiter_RejectsExcessiveRequests(t *testing.T) {
	// Wrap the mock handler with the rate limiter
	limiter := PerClientRateLimiter(mockHandler)

	// Create a test HTTP server
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	w := httptest.NewRecorder()

	// First, make allowed requests
	for i := 0; i < 12; i++ {
		limiter.ServeHTTP(w, req)
		if w.Result().StatusCode != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Result().StatusCode)
		}
	}

	// Exceed the rate limit
	for i := 0; i < 10; i++ {
		w = httptest.NewRecorder() // Reset the response recorder
		limiter.ServeHTTP(w, req)
		if w.Result().StatusCode != http.StatusTooManyRequests {
			t.Errorf("Expected status TooManyRequests, got %v", w.Result().StatusCode)
		}
	}
}

// Test if the rate limiter is enforced on a per-client basis
func TestRateLimiter_PerClientEnforcement(t *testing.T) {
	// Wrap the mock handler with the rate limiter
	limiter := PerClientRateLimiter(mockHandler)

	// Create two different test clients
	reqClient1 := httptest.NewRequest("GET", "/", nil)
	reqClient1.RemoteAddr = "192.168.1.1:1234" // Client 1 IP
	reqClient2 := httptest.NewRequest("GET", "/", nil)
	reqClient2.RemoteAddr = "192.168.1.2:5678" // Client 2 IP

	// Client 1 makes requests
	w1 := httptest.NewRecorder()
	limiter.ServeHTTP(w1, reqClient1)
	if w1.Result().StatusCode != http.StatusOK {
		t.Errorf("Expected status OK for Client 1, got %v", w1.Result().StatusCode)
	}

	// Client 2 makes requests
	w2 := httptest.NewRecorder()
	limiter.ServeHTTP(w2, reqClient2)
	if w2.Result().StatusCode != http.StatusOK {
		t.Errorf("Expected status OK for Client 2, got %v", w2.Result().StatusCode)
	}

	// Client 1 exceeds the rate limit
	for i := 0; i < 10; i++ {
		w1 = httptest.NewRecorder()
		limiter.ServeHTTP(w1, reqClient1)
	}
	if w1.Result().StatusCode != http.StatusTooManyRequests {
		t.Errorf("Expected status TooManyRequests for Client 1, got %v", w1.Result().StatusCode)
	}

	// Client 2 should still be OK
	w2 = httptest.NewRecorder()
	limiter.ServeHTTP(w2, reqClient2)
	if w2.Result().StatusCode != http.StatusOK {
		t.Errorf("Expected status OK for Client 2, got %v", w2.Result().StatusCode)
	}
}
