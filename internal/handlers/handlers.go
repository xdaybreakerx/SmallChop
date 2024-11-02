package handlers

import (
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

type Handlers struct {
	MongoRepo    *repository.MongoRepo
	RedisRepo    *repository.RedisRepo
	Template     *template.Template
	TemplatePath string
}

func NewHandlers(mongoRepo *repository.MongoRepo, redisRepo *repository.RedisRepo) (*Handlers, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("could not get working directory: %v", err)
	}
	templatePath := filepath.Join(cwd, "internal", "templates", "index.html")
	tmpl := template.Must(template.ParseFiles(templatePath))

	return &Handlers{
		MongoRepo:    mongoRepo,
		RedisRepo:    redisRepo,
		Template:     tmpl,
		TemplatePath: templatePath,
	}, nil
}

func (h *Handlers) RootHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	if err := h.Template.Execute(w, nil); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}

}

func (h *Handlers) ShortenURLHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	url := r.FormValue("url")
	fmt.Println("Payload: ", url)

	shortCode, err := h.MongoRepo.SaveURL(ctx, url)
	if err != nil {
		http.Error(w, "Failed to save URL", http.StatusInternalServerError)
		return
	}

	fullShortURL := fmt.Sprintf("http://smallchop.net/r/%s", shortCode)
	fmt.Fprintf(w, `<p class="mt-4 text-green-600">Shortened URL: <a href="/r/%s">%s</a></p>`, shortCode, fullShortURL)
}

func (h *Handlers) RedirectHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Path[len("/r/"):]
	if key == "" {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	// Decode the short code to get the integer ID
	id := utils.Decode(key)
	if id == -1 {
		http.Error(w, "Invalid short URL", http.StatusBadRequest)
		return
	}

	// Try to get the long URL from Redis cache
	longURL, err := h.RedisRepo.GetLongURL(ctx, key, h.MongoRepo, 1*time.Hour)
	if err != nil {
		// If not found in Redis, get the URL from MongoDB
		urlDoc, err := h.MongoRepo.FindURLByID(ctx, id)
		if err != nil {
			http.Error(w, "Shortened URL not found", http.StatusNotFound)
			return
		}
		longURL = urlDoc.LongURL

		// Store in Redis for future requests
		err = h.RedisRepo.SetKey(ctx, key, longURL, 1*time.Hour)
		if err != nil {
			log.Printf("Failed to set Redis cache: %v", err)
		}
	}

	// Increment the access count
	err = h.MongoRepo.IncrementAccessCount(ctx, id)
	if err != nil {
		log.Printf("Failed to increment access count: %v", err)
	}

	http.Redirect(w, r, longURL, http.StatusPermanentRedirect)
}
