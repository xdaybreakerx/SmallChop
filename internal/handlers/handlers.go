package handlers

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gochop-it/internal/repository"
	"gochop-it/internal/utils"
)

type URLStore interface {
	repository.URLRepository
	SaveURL(ctx context.Context, longURL string) (string, error)
}

type Handlers struct {
	MongoRepo     URLStore
	RedisRepo     *repository.RedisRepo
	Template      *template.Template
	TemplatePath  string
	PublicBaseURL string
}

func NewHandlers(mongoRepo URLStore, redisRepo *repository.RedisRepo, publicBaseURL string) (*Handlers, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("could not get working directory: %v", err)
	}
	templatePath := filepath.Join(cwd, "internal", "templates", "index.html")
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return nil, fmt.Errorf("parse index template: %w", err)
	}

	return &Handlers{
		MongoRepo:     mongoRepo,
		RedisRepo:     redisRepo,
		Template:      tmpl,
		TemplatePath:  templatePath,
		PublicBaseURL: publicBaseURL,
	}, nil
}

func (h *Handlers) RootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
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
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "Request body is too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "Invalid form", http.StatusBadRequest)
		}
		return
	}
	values := r.PostForm["url"]
	if len(values) != 1 {
		http.Error(w, "Provide one destination URL in the form body", http.StatusBadRequest)
		return
	}
	destination, err := utils.SanitizeURL(values[0])
	if err != nil {
		http.Error(w, "Invalid destination URL", http.StatusBadRequest)
		return
	}

	shortCode, err := h.MongoRepo.SaveURL(ctx, destination)
	if err != nil {
		if errors.Is(err, utils.ErrInvalidURL) {
			http.Error(w, "Invalid destination URL", http.StatusBadRequest)
			return
		}
		http.Error(w, "Failed to save URL", http.StatusInternalServerError)
		return
	}

	fullShortURL := h.PublicBaseURL + "/r/" + shortCode
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := fmt.Fprintf(w, `<p class="mt-4 text-green-600">Shortened URL: <a href="/r/%s">%s</a></p>`, template.HTMLEscapeString(shortCode), template.HTMLEscapeString(fullShortURL)); err != nil {
		log.Printf("Error writing shortened URL response: %v", err)
	}
}

func (h *Handlers) RedirectHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	key, found := strings.CutPrefix(r.URL.Path, "/r/")
	if !found || key == "" {
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
