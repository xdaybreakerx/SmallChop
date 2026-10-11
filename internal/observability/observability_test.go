package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestRequestMetricsAndLogsHaveBoundedDimensions(t *testing.T) {
	var logs bytes.Buffer
	o := New(slog.New(slog.NewJSONHandler(&logs, nil)))
	mux := http.NewServeMux()
	mux.HandleFunc("/r/", func(w http.ResponseWriter, r *http.Request) {
		o.DependencyFailure(r.Context(), "redis", "get")
		w.Header().Set("Location", "https://secret.example/?token=destination-secret")
		w.WriteHeader(308)
		w.WriteHeader(500) // The first final status is authoritative.
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { http.NotFound(w, nil) })
	handler := o.HTTP(mux)
	var ids []string
	for _, tc := range []struct{ method, path string }{{"GET", "/r/private-code?token=query-secret"}, {"CUSTOM-secret", "/unknown-secret"}} {
		request := httptest.NewRequest(tc.method, tc.path, strings.NewReader("body-secret"))
		request.Header.Set("X-Request-ID", "caller-secret")
		request.RemoteAddr = "192.0.2.123:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, request)
		id := w.Header().Get("X-Request-ID")
		if len(id) != 32 || id == "caller-secret" {
			t.Fatalf("invalid generated ID %q", id)
		}
		ids = append(ids, id)
	}
	if ids[0] == ids[1] {
		t.Fatal("request ID reused")
	}
	w := httptest.NewRecorder()
	o.MetricsHandler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	metrics := w.Body.String()
	for _, want := range []string{
		`smallchop_http_requests_total{method="GET",route="/r/{code}",status_class="3xx"} 1`,
		`smallchop_http_request_duration_seconds_count{method="GET",route="/r/{code}",status_class="3xx"} 1`,
		`smallchop_http_requests_total{method="other",route="unmatched",status_class="4xx"} 1`,
		`smallchop_dependency_failures_total{dependency="redis",operation="get"} 1`,
	} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("missing %s in %s", want, metrics)
		}
	}
	for _, secret := range []string{"private-code", "query-secret", "destination-secret", "caller-secret", "body-secret", "192.0.2.123", "unknown-secret", "CUSTOM-secret"} {
		if strings.Contains(logs.String(), secret) || strings.Contains(metrics, secret) {
			t.Fatalf("leaked %s", secret)
		}
	}
	decoder := json.NewDecoder(&logs)
	var warning, completed map[string]any
	if err := decoder.Decode(&warning); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&completed); err != nil {
		t.Fatal(err)
	}
	if warning["request_id"] != ids[0] || completed["request_id"] != ids[0] || completed["status"] != float64(308) {
		t.Fatal("warning/response/request log correlation failed")
	}
}

func TestConcurrentMetricsAndUnknownDimensions(t *testing.T) {
	o := New(nil)
	o.CacheRead(context.Background(), "arbitrary-secret")
	o.DependencyFailure(context.Background(), "unknown", "arbitrary-secret")
	handler := o.HTTP(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }))
	var group sync.WaitGroup
	for range 40 {
		group.Go(func() {
			o.CacheRead(context.Background(), "hit")
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
		})
	}
	group.Wait()
	w := httptest.NewRecorder()
	o.MetricsHandler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	if !strings.Contains(w.Body.String(), `smallchop_cache_reads_total{outcome="hit"} 40`) || strings.Contains(w.Body.String(), "arbitrary-secret") {
		t.Fatal("bad concurrent counter/cardinality")
	}
}

func TestMetricsMuxMethodAndPath(t *testing.T) {
	mux := New(nil).MetricsHandler()
	for _, tc := range []struct {
		method, path string
		status       int
	}{{"GET", "/metrics", 200}, {"POST", "/metrics", 405}, {"GET", "/", 404}} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.status {
			t.Fatalf("%s %s = %d", tc.method, tc.path, w.Code)
		}
	}
}
