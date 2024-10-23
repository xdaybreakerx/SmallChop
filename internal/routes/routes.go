package routes

import (
	"net/http"

	"gochop-it/internal/handlers"
	"gochop-it/internal/middleware"
)

func RegisterRoutes(h *handlers.Handlers) {
	http.HandleFunc("/", h.RootHandler)
	http.Handle("/shorten", middleware.PerClientRateLimiter(http.HandlerFunc(h.ShortenURLHandler)))
	http.Handle("/r/", middleware.PerClientRateLimiter(http.HandlerFunc(h.RedirectHandler)))
}
