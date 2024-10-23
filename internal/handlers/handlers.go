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
	fmt.Println("Serving index.html!")
}

func (h *Handlers) ShortenURLHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	url := r.FormValue("url")
	fmt.Println("Payload: ", url)

	existingShortURL, err := h.MongoRepo.FindShortURLByLongURL(ctx, url)
	if err != nil {
		http.Error(w, "Failed to check URL existence", http.StatusInternalServerError)
		return
	}

	var shortURL string
	if existingShortURL != "" {
		fmt.Println("URL already exists in the database, returning existing short URL")
		shortURL = existingShortURL
	} else {
		shortURL = utils.GetShortCode()
		_, err := h.MongoRepo.SaveURL(ctx, shortURL, url)
		if err != nil {
			http.Error(w, "Failed to save URL", http.StatusInternalServerError)
			return
		}
		fmt.Println("URL not found, saved new short URL")
	}

	fullShortURL := fmt.Sprintf("http://localhost:8080/r/%s", shortURL)
	fmt.Fprintf(w, `<p class="mt-4 text-green-600">Shortened URL: <a href="/r/%s">%s</a></p>`, shortURL, fullShortURL)
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

	longURL, err := h.RedisRepo.GetLongURL(ctx, h.MongoRepo, key, 1*time.Hour)
	if err != nil {
		http.Error(w, "Shortened URL not found", http.StatusNotFound)
		return
	}

	err = h.MongoRepo.IncrementAccessCount(ctx, key)
	if err != nil {
		log.Printf("Failed to increment access count: %v", err)
	}

	http.Redirect(w, r, longURL, http.StatusPermanentRedirect)
}
