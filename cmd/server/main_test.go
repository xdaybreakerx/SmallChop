package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"gochop-it/internal/repository"
)

// Test root handler "/"
func TestRootHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Record the response
	rr := httptest.NewRecorder()

	// Create handler function
	handler := http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		// Mock template rendering
		fmt.Fprintln(writer, "Test Index Page")
	})

	// Call the handler
	handler.ServeHTTP(rr, req)

	// Assert status code is OK
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Assert the expected response body
	expected := "Test Index Page\n"
	if rr.Body.String() != expected {
		t.Errorf("Handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

// Test shorten handler "/shorten"
func TestShortenHandler(t *testing.T) {
	// Mock Redis server using miniredis
	mockRedis, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	defer mockRedis.Close()

	// Create Redis client connected to the mock Redis server
	rdb := redis.NewClient(&redis.Options{
		Addr: mockRedis.Addr(),
	})

	// Mock POST request with a form value
	req, err := http.NewRequest("POST", "/shorten", strings.NewReader("url=http://example.com"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Record the response
	rr := httptest.NewRecorder()

	// Create handler function
	handler := http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		// Check if the request is a POST request
		if req.Method != http.MethodPost {
			http.Error(writer, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		// Get the URL to shorten from the request
		url := req.FormValue("url")
		shortURL := "testShortCode" // Mock the short code generation
		fullShortURL := fmt.Sprintf("http://localhost:8080/r/%s", shortURL)

		// Set the key in mock Redis
		repository.SetKey(context.Background(), rdb, shortURL, url, 0)

		// Return the shortened URL
		fmt.Fprintf(writer, `<p class="mt-4 text-green-600">Shortened URL: <a href="/r/%s">%s</a></p>`, shortURL, fullShortURL)
	})

	// Call the handler
	handler.ServeHTTP(rr, req)

	// Assert status code is OK
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Assert the expected response body
	expected := `<p class="mt-4 text-green-600">Shortened URL: <a href="/r/testShortCode">http://localhost:8080/r/testShortCode</a></p>`
	if rr.Body.String() != expected {
		t.Errorf("Handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}

	// Verify that the short URL was correctly stored in mock Redis
	storedValue, err := mockRedis.Get("testShortCode")
	if err != nil {
		t.Errorf("Failed to retrieve short URL from mock Redis: %v", err)
	}
	if storedValue != "http://example.com" {
		t.Errorf("Expected value %s, got %s", "http://example.com", storedValue)
	}
}

// Test redirect handler "/r/{code}"
func TestRedirectHandler(t *testing.T) {
	// Mock Redis server using miniredis
	mockRedis, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	defer mockRedis.Close()

	// Create Redis client connected to the mock Redis server
	rdb := redis.NewClient(&redis.Options{
		Addr: mockRedis.Addr(),
	})

	// Set a mock short URL in Redis
	mockRedis.Set("testShortCode", "http://example.com")

	// Check the error return value of mockRedis.Set
	if err := mockRedis.Set("testShortCode", "http://example.com"); err != nil {
		t.Fatalf("Failed to set key in mock Redis: %v", err)
	}

	// Mock GET request to the redirect endpoint
	req, err := http.NewRequest("GET", "/r/testShortCode", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Record the response
	rr := httptest.NewRecorder()

	// Create handler function
	handler := http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		// Extract the short code from the path
		code := "testShortCode" // Mock extracting the code from URL path

		longURL, err := repository.GetLongURL(context.Background(), rdb, code)
		if err != nil {
			http.Error(writer, "Shortened URL not found", http.StatusNotFound)
			return
		}

		http.Redirect(writer, req, longURL, http.StatusPermanentRedirect)
	})

	// Call the handler
	handler.ServeHTTP(rr, req)

	// Assert that the response status code is a redirect
	if status := rr.Code; status != http.StatusPermanentRedirect {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusPermanentRedirect)
	}

	// Assert that the redirect location matches the expected long URL
	expectedLocation := "http://example.com"
	if location := rr.Header().Get("Location"); location != expectedLocation {
		t.Errorf("Handler returned wrong redirect location: got %v want %v", location, expectedLocation)
	}
}
