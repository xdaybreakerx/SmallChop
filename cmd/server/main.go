package main

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"log"
	"net/http"
)

func main() {
	// Redis setup
	rdb := redis.NewClient(&redis.Options{
		Addr: "redis:6379", // Redis service hostname in Docker Compose
	})

	// Test Redis connection
	_, err := rdb.Ping(context.Background()).Result()
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}
	fmt.Println("Connected to Redis!")

	http.HandleFunc("/", func(writer http.ResponseWriter, req *http.Request) {
		fmt.Fprintln(writer, "hello world")
	})

	// Start the server on port 8080
	fmt.Println("Server is running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
