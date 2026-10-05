package middleware

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"time"

	"gochop-it/internal/config"
	"golang.org/x/time/rate"
)

type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type clientLimiter struct {
	mu          sync.Mutex
	clients     map[netip.Addr]*client
	lastCleanup time.Time
	policy      config.RatePolicy
	proxies     []netip.Addr
	now         func() time.Time
	next        http.Handler
}

func PerClientRateLimiter(next http.Handler, policy config.RatePolicy, proxies []netip.Addr) http.Handler {
	return &clientLimiter{
		clients: make(map[netip.Addr]*client), policy: policy,
		proxies: slices.Clone(proxies), now: time.Now, next: next,
	}
}

// Caddy overwrites X-Forwarded-For with one peer IP. Lists are deliberately
// rejected: this deployment has one ingress proxy, not a chain of proxies.
func clientIP(r *http.Request, proxies []netip.Addr) (netip.Addr, error) {
	peer, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}, err
	}
	ip := peer.Addr().Unmap()
	if !slices.Contains(proxies, ip) {
		return ip, nil
	}
	values := r.Header.Values("X-Forwarded-For")
	if len(values) != 1 {
		return netip.Addr{}, fmt.Errorf("trusted proxy must supply one client address")
	}
	forwarded, err := netip.ParseAddr(strings.TrimSpace(values[0]))
	if err != nil || forwarded.Zone() != "" {
		return netip.Addr{}, fmt.Errorf("invalid forwarded client address")
	}
	return forwarded.Unmap(), nil
}

func (l *clientLimiter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ip, err := clientIP(r, l.proxies)
	if err != nil {
		http.Error(w, "Invalid client address", http.StatusBadRequest)
		return
	}
	now := l.now()
	l.mu.Lock()
	// Clean up on requests rather than starting a goroutine for every mux.
	// Keep idle buckets until a full refill as well as the minimum idle period.
	if now.Sub(l.lastCleanup) >= time.Minute {
		for ip, c := range l.clients {
			if now.Sub(c.lastSeen) > 3*time.Minute && c.limiter.TokensAt(now) >= float64(l.policy.Burst) {
				delete(l.clients, ip)
			}
		}
		l.lastCleanup = now
	}
	c, exists := l.clients[ip]
	if !exists {
		c = &client{limiter: rate.NewLimiter(l.policy.Rate, l.policy.Burst)}
		l.clients[ip] = c
	}
	c.lastSeen = now
	allowed := c.limiter.AllowN(now, 1)
	l.mu.Unlock()
	if !allowed {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"status": "Request Failed", "body": "Rate limit exceeded, try again later.",
		}); err != nil {
			log.Printf("Error writing rate limit response: %v", err)
		}
		return
	}
	l.next.ServeHTTP(w, r)
}
