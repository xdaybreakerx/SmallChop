package routes

import (
	"net/http"

	"gochop-it/internal/config"
	"gochop-it/internal/handlers"
	"gochop-it/internal/middleware"
)

func NewMux(h *handlers.Handlers, cfg config.Config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/livez", h.Liveness)
	mux.HandleFunc("/readyz", h.Readiness)
	mux.HandleFunc("/", h.RootHandler)
	mux.Handle("/shorten", middleware.PerClientRateLimiter(http.HandlerFunc(h.ShortenURLHandler), cfg.CreatePolicy, cfg.TrustedProxies))
	mux.Handle("/r/", middleware.PerClientRateLimiter(http.HandlerFunc(h.RedirectHandler), cfg.RedirectPolicy, cfg.TrustedProxies))
	if h.Observer != nil {
		return h.Observer.HTTP(mux)
	}
	return mux
}
