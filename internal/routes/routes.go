package routes

import (
	"net/http"

	"gochop-it/internal/config"
	"gochop-it/internal/handlers"
	"gochop-it/internal/middleware"
)

func NewMux(h *handlers.Handlers, cfg config.Config) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.RootHandler)
	mux.Handle("/shorten", middleware.PerClientRateLimiter(http.HandlerFunc(h.ShortenURLHandler), cfg.CreatePolicy, cfg.TrustedProxies))
	mux.Handle("/r/", middleware.PerClientRateLimiter(http.HandlerFunc(h.RedirectHandler), cfg.RedirectPolicy, cfg.TrustedProxies))
	return mux
}
