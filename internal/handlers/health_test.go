package handlers

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"gochop-it/internal/config"
)

type probeFunc func(context.Context) error

func (p probeFunc) Ping(ctx context.Context) error { return p(ctx) }

func TestHealthDependencyContract(t *testing.T) {
	calls := 0
	h := &Handlers{Timeouts: config.DefaultTimeouts(), MongoHealth: probeFunc(func(context.Context) error { calls++; return nil })}
	w := httptest.NewRecorder()
	h.Liveness(w, httptest.NewRequest("GET", "/livez", nil))
	if w.Code != 200 || calls != 0 {
		t.Fatalf("liveness status=%d probe calls=%d", w.Code, calls)
	}
	w = httptest.NewRecorder()
	h.Readiness(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 200 || calls != 1 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("readiness status=%d calls=%d", w.Code, calls)
	}
	h.MongoHealth = nil
	w = httptest.NewRecorder()
	h.Readiness(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 503 {
		t.Fatalf("absent probe status=%d", w.Code)
	}
}

func TestReadinessDeadlineAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		mongo, request time.Duration
		canceled       bool
	}{
		{20 * time.Millisecond, time.Second, false},
		{time.Second, 20 * time.Millisecond, false},
		{time.Second, time.Second, true},
	} {
		h := &Handlers{Timeouts: config.Timeouts{Mongo: tc.mongo, Request: tc.request}}
		var observed error
		h.MongoHealth = probeFunc(func(ctx context.Context) error { <-ctx.Done(); observed = ctx.Err(); return observed })
		ctx, cancel := context.WithCancel(context.Background())
		if tc.canceled {
			cancel()
		}
		w := httptest.NewRecorder()
		start := time.Now()
		h.Readiness(w, httptest.NewRequest("GET", "/readyz", nil).WithContext(ctx))
		cancel()
		want := context.DeadlineExceeded
		if tc.canceled {
			want = context.Canceled
		}
		if w.Code != 503 || !errors.Is(observed, want) || time.Since(start) > 500*time.Millisecond {
			t.Fatalf("status=%d observed=%v", w.Code, observed)
		}
	}
}
