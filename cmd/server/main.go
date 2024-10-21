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
	_, err := dbClient.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
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

	// Shorten URL handler
	http.HandleFunc("/shorten", func(writer http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(writer, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		url := req.FormValue("url")
		fmt.Println("Payload: ", url)

		shortURL := utils.GetShortCode()
		fullShortURL := fmt.Sprintf("http://localhost:8080/r/%s", shortURL)

		repository.SetKey(ctx, dbClient, shortURL, url, 0)

		fmt.Fprintf(writer, `<p class="mt-4 text-green-600">Shortened URL: <a href="/r/%s">%s</a></p>`, shortURL, fullShortURL)
	})

	// Redirect handler
	http.HandleFunc("/r/{code}", func(writer http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			http.Error(writer, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		key := req.URL.Path[len("/r/"):]

		if key == "" {
			http.Error(writer, "Invalid URL", http.StatusBadRequest)
			return
		}

		longURL, err := repository.GetLongURL(ctx, dbClient, key)
		if err != nil {
			http.Error(writer, "Shortened URL not found", http.StatusNotFound)
			return
		}

		http.Redirect(writer, req, longURL, http.StatusPermanentRedirect)
	})

	// Start the server
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
	fmt.Println("Server is running on http://localhost:8080")
}
