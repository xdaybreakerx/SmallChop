package main

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"log"
	"net/http"
	"gochop-it/internal/utils"
	"gochop-it/internal/repository"
)

var ctx = context.Background()

func main() {
	// Redis setup
	dbClient := redis.NewClient(&redis.Options{
		Addr: "redis:6379", // Redis service hostname in Docker Compose
	})

	// Test Redis connection
	_, err := dbClient.Ping(context.Background()).Result()
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}
	fmt.Println("Connected to Redis!")

	// http.HandleFunc("/", func(writer http.ResponseWriter, req *http.Request) {
	// 	fmt.Fprintln(writer, "hello world")
	// 	// @TODO: serve index page
	// })

	http.HandleFunc("/shorten", func(writer http.ResponseWriter, req *http.Request) {
		// Get the URL to shorten from the request
		url := req.FormValue("url")

		// Close the body when done
		fmt.Println("Payload: ", url)

		// Shorten the URL
		shortURL := utils.GetShortCode()
		fullShortURL := fmt.Sprintf("http://localhost:8080/r/%s",
			shortURL)
		// Generated short URL
		fmt.Printf("Generated short URL: %s\n", fullShortURL)
		fmt.Printf("Generated short URL: %s\n", shortURL) // Log to console

		// Set the key in Redis 
		repository.SetKey(&ctx, dbClient, shortURL, url, 0)  
		// @TODO return the shortened URL in the UI
	})

	// Start the server on port 8080
	fmt.Println("Server is running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
