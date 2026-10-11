package handlers

import (
	"context"
	"net/http"
)

// Liveness deliberately does not consult dependencies.
func (h *Handlers) Liveness(w http.ResponseWriter, r *http.Request) {
	if !healthMethod(w, r) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("live\n"))
}

// Readiness follows the full service contract: Mongo is required, Redis optional.
func (h *Handlers) Readiness(w http.ResponseWriter, r *http.Request) {
	if !healthMethod(w, r) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	ctx, cancel := context.WithTimeout(r.Context(), min(h.Timeouts.Mongo, h.Timeouts.Request))
	defer cancel()
	if h.MongoHealth == nil || h.MongoHealth.Ping(ctx) != nil {
		h.Observer.DependencyFailure(ctx, "mongo", "ping")
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready\n"))
}

func healthMethod(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return false
	}
	return true
}
