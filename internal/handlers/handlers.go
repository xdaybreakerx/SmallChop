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

	"gochop-it/internal/config"
	"gochop-it/internal/repository"
	"gochop-it/internal/utils"
)

type URLStore interface {
	FindURLByID(ctx context.Context, id int64) (*repository.URL, error)
	SaveURL(ctx context.Context, longURL string) (string, error)
}

type URLCache interface {
	GetLongURL(ctx context.Context, shortCode string) (string, error)
	SetKey(ctx context.Context, key, value string, ttl time.Duration) error
}

type Handlers struct {
	Timeouts      config.Timeouts
	MongoRepo     URLStore
	RedisRepo     URLCache
	Template      *template.Template
	TemplatePath  string
	PublicBaseURL string
}

func NewHandlers(mongoRepo URLStore, redisRepo URLCache, publicBaseURL string, timeouts config.Timeouts) (*Handlers, error) {
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
		Timeouts:      timeouts,
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

	ctx, cancel := context.WithTimeout(r.Context(), h.Timeouts.Request)
	defer cancel()
	dbCtx, dbCancel := context.WithTimeout(ctx, h.Timeouts.Mongo)
	defer dbCancel()
	shortCode, err := h.MongoRepo.SaveURL(dbCtx, destination)
	if err != nil {
		if errors.Is(err, utils.ErrInvalidURL) {
			http.Error(w, "Invalid destination URL", http.StatusBadRequest)
			return
		}
		storageFailure(w, err)
		return
	}

	fullShortURL := h.PublicBaseURL + "/r/" + shortCode
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := fmt.Fprintf(w, `<p class="mt-4 text-green-600">Shortened URL: <a href="/r/%s">%s</a></p>`, template.HTMLEscapeString(shortCode), template.HTMLEscapeString(fullShortURL)); err != nil {
		log.Printf("Error writing shortened URL response: %v", err)
	}
}

func (h *Handlers) RedirectHandler(w http.ResponseWriter, r *http.Request) {
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

	ctx, cancel := context.WithTimeout(r.Context(), h.Timeouts.Request)
	defer cancel()
	cacheCtx, cacheCancel := context.WithTimeout(ctx, h.Timeouts.Cache)
	longURL, cacheErr := h.RedisRepo.GetLongURL(cacheCtx, key)
	cacheCancel()
	if ctx.Err() != nil {
		storageFailure(w, ctx.Err())
		return
	}
	if cacheErr != nil {
		dbCtx, dbCancel := context.WithTimeout(ctx, h.Timeouts.Mongo)
		urlDoc, err := h.MongoRepo.FindURLByID(dbCtx, id)
		dbCancel()
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				http.Error(w, "Shortened URL not found", http.StatusNotFound)
			} else {
				storageFailure(w, err)
			}
			return
		}
		longURL = urlDoc.LongURL
		fillCtx, fillCancel := context.WithTimeout(ctx, h.Timeouts.Cache)
		err = h.RedisRepo.SetKey(fillCtx, key, longURL, time.Hour)
		fillCancel()
		if err != nil {
			log.Println("Cache fill failed; serving database destination")
		}
	}
	http.Redirect(w, r, longURL, http.StatusPermanentRedirect)
}

func storageFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, repository.ErrUnavailable) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		http.Error(w, "Storage temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	http.Error(w, "Storage operation failed", http.StatusInternalServerError)
}
