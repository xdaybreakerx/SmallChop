package routes

import (
	"context"
	"html/template"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gochop-it/internal/config"
	"gochop-it/internal/handlers"
	"gochop-it/internal/observability"
	"gochop-it/internal/repository"
)

type routeStore struct{}

func (routeStore) SaveURL(context.Context, string) (string, error) { return "c", nil }
func (routeStore) FindURLByID(context.Context, int64) (*repository.URL, error) {
	return &repository.URL{ID: 1, LongURL: "https://example.com"}, nil
}

func TestRoutesPoliciesAndProxyIdentity(t *testing.T) {
	cache := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: cache.Addr()})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	h := &handlers.Handlers{
		Timeouts: config.DefaultTimeouts(), MongoRepo: routeStore{}, RedisRepo: &repository.RedisRepo{Client: client}, PublicBaseURL: "https://short.example",
		Template: template.Must(template.New("index").Parse("<form></form>")),
	}
	// Very slow refill makes burst assertions independent of scheduler timing.
	cfg := config.Config{TrustedProxies: []netip.Addr{netip.MustParseAddr("172.30.80.2")}, CreatePolicy: config.RatePolicy{Rate: 0.001, Burst: 4}, RedirectPolicy: config.RatePolicy{Rate: 0.001, Burst: 20}}
	mux := NewMux(h, cfg)
	call := func(method, path, clientIP string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader("url=https%3A%2F%2Fexample.com"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = "172.30.80.2:1234"
		req.Header.Set("X-Forwarded-For", clientIP)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		return w
	}
	for i := 0; i < 4; i++ {
		if w := call("POST", "/shorten", "192.0.2.1"); w.Code != 200 {
			t.Fatalf("creation %d = %d", i, w.Code)
		}
	}
	if w := call("POST", "/shorten", "192.0.2.1"); w.Code != 429 {
		t.Fatalf("creation overflow = %d", w.Code)
	}
	if w := call("POST", "/shorten", "192.0.2.2"); w.Code != 200 {
		t.Fatalf("second client = %d", w.Code)
	}
	for i := 0; i < 20; i++ {
		w := call("GET", "/r/c", "192.0.2.1")
		if w.Code != 308 || w.Header().Get("Location") != "https://example.com" {
			t.Fatalf("redirect %d = %d", i, w.Code)
		}
	}
	if w := call("GET", "/r/c", "192.0.2.1"); w.Code != 429 {
		t.Fatalf("redirect overflow = %d", w.Code)
	}
	if w := call("GET", "/r/c", "192.0.2.2"); w.Code != 308 {
		t.Fatalf("second redirect client = %d", w.Code)
	}
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/", 200}, {"POST", "/", 405}, {"GET", "/unknown", 404},
		{"GET", "/shorten", 405}, {"POST", "/r/c", 405}, {"GET", "/r/bc", 400},
	} {
		if w := call(tc.method, tc.path, "192.0.2.3"); w.Code != tc.status {
			t.Fatalf("%s %s = %d; want %d", tc.method, tc.path, w.Code, tc.status)
		}
	}
}

func TestHealthAndRateLimitObservability(t *testing.T) {
	h := &handlers.Handlers{Timeouts: config.DefaultTimeouts(), MongoRepo: routeStore{}, Observer: observability.New(nil)}
	cfg := config.Config{CreatePolicy: config.RatePolicy{Rate: .001, Burst: 1}, RedirectPolicy: config.RatePolicy{Rate: .001, Burst: 1}}
	mux := NewMux(h, cfg)
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/r/c", 308}, {"GET", "/r/c", 429},
		{"GET", "/livez", 200}, {"GET", "/livez", 200}, {"GET", "/readyz", 503},
		{"POST", "/livez", 405}, {"POST", "/readyz", 405}, {"GET", "/metrics", 404},
	} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.status || len(w.Header().Get("X-Request-ID")) != 32 {
			t.Fatalf("%s %s status=%d id=%s", tc.method, tc.path, w.Code, w.Header().Get("X-Request-ID"))
		}
	}
	w := httptest.NewRecorder()
	h.Observer.MetricsHandler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	for _, want := range []string{
		`smallchop_http_requests_total{method="GET",route="/r/{code}",status_class="4xx"} 1`,
		`smallchop_http_requests_total{method="GET",route="/livez",status_class="2xx"} 2`,
	} {
		if !strings.Contains(w.Body.String(), want) {
			t.Fatalf("missing %s", want)
		}
	}
}
