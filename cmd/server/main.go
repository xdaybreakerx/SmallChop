package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gochop-it/internal/repository"
	"gochop-it/internal/utils"
)

var ctx = context.Background()

func main() {
	// MongoDB setup
	mongoRepo, err := repository.NewMongoRepo(ctx)
	if err != nil {
		log.Fatalf("Could not connect to MongoDB: %v", err)
	}
	fmt.Println("Connected to MongoDB!")

	// Ensure MongoDB connection is valid
	err = mongoRepo.Client.Ping(ctx, nil)
	if err != nil {
		log.Fatalf("MongoDB ping failed: %v", err)
	}
	fmt.Println("MongoDB connection is active!")

	// Redis setup using RedisRepo
	redisRepo := repository.NewRedisRepo()

	// Ensure Redis connection is valid
	e := redisRepo.Ping(ctx)
	if e != nil {
		log.Fatalf("Could not connect to Redis: %v", e)
	}
	fmt.Println("Connected to Redis!")

	// Fetch HTML template
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Could not get working directory: %v", err)
	}
	templatePath := filepath.Join(cwd, "internal", "templates", "index.html")

	// Root handler
	http.HandleFunc("/", func(writer http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			http.Error(writer, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}
		tmpl := template.Must(template.ParseFiles(templatePath))
		if err := tmpl.Execute(writer, nil); err != nil {
			log.Printf("Error executing template: %v", err)
			http.Error(writer, "Internal Server Error", http.StatusInternalServerError)
		}
		fmt.Println("Serving index.html!")
	})

	// Shorten URL handler with rate limiting
	http.Handle("/shorten", utils.PerClientRateLimiter(func(writer http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(writer, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		url := req.FormValue("url")
		fmt.Println("Payload: ", url)

		// Check if the long URL already exists in the database
		existingShortURL, err := mongoRepo.FindShortURLByLongURL(ctx, url)
		if err != nil {
			http.Error(writer, "Failed to check URL existence", http.StatusInternalServerError)
			return
		}

		var shortURL string
		// If the long URL already exists, use the existing short URL
		if existingShortURL != "" {
			fmt.Println("URL already exists in the database, returning existing short URL")
			shortURL = existingShortURL
		} else {
			// Generate a new short URL if not found in the database
			shortURL = utils.GetShortCode()
			_, err := mongoRepo.SaveURL(ctx, shortURL, url)
			if err != nil {
				http.Error(writer, "Failed to save URL", http.StatusInternalServerError)
				return
			}
			fmt.Println("URL not found, saved new short URL")
		}

		fullShortURL := fmt.Sprintf("http://localhost:8080/r/%s", shortURL)

		// Respond with the shortened URL
		fmt.Fprintf(writer, `<p class="mt-4 text-green-600">Shortened URL: <a href="/r/%s">%s</a></p>`, shortURL, fullShortURL)
	}))

	// Redirect handler with rate limiting and Redis caching with lazy loading
	http.Handle("/r/", utils.PerClientRateLimiter(func(writer http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			http.Error(writer, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		// Extract the key (short URL) from the request URL
		key := req.URL.Path[len("/r/"):]

		if key == "" {
			http.Error(writer, "Invalid URL", http.StatusBadRequest)
			return
		}

		// Try to get the long URL from Redis with lazy loading from MongoDB
		longURL, err := redisRepo.GetLongURL(ctx, mongoRepo, key, 1*time.Hour) // TTL = 1 hour for Redis caching
		if err != nil {
			http.Error(writer, "Shortened URL not found", http.StatusNotFound)
			return
		}

		// Increment the access count for the short URL in MongoDB
		err = mongoRepo.IncrementAccessCount(ctx, key)
		if err != nil {
			log.Printf("Failed to increment access count: %v", err)
		}

		// Redirect the user to the long URL
		http.Redirect(writer, req, longURL, http.StatusPermanentRedirect)
	}))

	// Start the server
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
	fmt.Println("Server is running on http://localhost:8080")
}
