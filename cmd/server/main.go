package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-redis/redis/v8"
	"gochop-it/internal/repository"
	"gochop-it/internal/utils"
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

	// Fetch HTML template
	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Could not get working directory: %v", err)
	}
	// Construct the absolute path to the HTML template
	templatePath := filepath.Join(cwd, "internal", "templates", "index.html")

	// GET localhost:8080/
	http.HandleFunc("/", func(writer http.ResponseWriter, req *http.Request) {
		// Check if the request is a GET request
		if req.Method != http.MethodGet {
			http.Error(writer, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}
		tmpl := template.Must(template.ParseFiles(templatePath))
		tmpl.Execute(writer, nil)
		fmt.Println("Serving index.html!")
	})

	// POST localhost:8080/shorten
	http.HandleFunc("/shorten", func(writer http.ResponseWriter, req *http.Request) {
		// Check if the request is a POST request
		if req.Method != http.MethodPost {
			http.Error(writer, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		// Get the URL to shorten from the request
		url := req.FormValue("url")

		// Close the body when done
		fmt.Println("Payload: ", url)

		// Shorten the URL
		shortURL := utils.GetShortCode()
		fullShortURL := fmt.Sprintf("http://localhost:8080/r/%s",
			shortURL)
		// Generated short URL
		// Log to console
		fmt.Printf("Generated short URL: %s\n", fullShortURL)
		fmt.Printf("Generated short URL: %s\n", shortURL)
		// Set the key in Redis
		repository.SetKey(&ctx, dbClient, shortURL, url, 0)
		fmt.Fprintf(writer,
			`<p class="mt-4 text-green-600">Shortened URL: <a 
			href="/r/%s" class="underline">%s</a></p>`, shortURL, fullShortURL)
	})

	// Start the server on port 8080
	fmt.Println("Server is running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
