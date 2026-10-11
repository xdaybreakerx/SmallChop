package handlers

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"gochop-it/internal/observability"
	"gochop-it/internal/repository"
)

func TestRedirectOperationMetrics(t *testing.T) {
	for _, tc := range []struct {
		name                                   string
		getErr, setErr, mongoErr               error
		disabled                               bool
		status                                 int
		readOutcome, fillOutcome               string
		getFailures, setFailures, findFailures int
	}{
		{name: "hit", status: 308, readOutcome: "hit"},
		{name: "miss", getErr: repository.ErrCacheMiss, status: 308, readOutcome: "miss", fillOutcome: "success"},
		{name: "read failure", getErr: errors.New("private-driver-error"), status: 308, readOutcome: "error", fillOutcome: "success", getFailures: 1},
		{name: "fill failure", getErr: repository.ErrCacheMiss, setErr: errors.New("private-driver-error"), status: 308, readOutcome: "miss", fillOutcome: "error", setFailures: 1},
		{name: "disabled", disabled: true, status: 308},
		{name: "not found", getErr: repository.ErrCacheMiss, mongoErr: repository.ErrNotFound, status: 404, readOutcome: "miss"},
		{name: "Mongo unavailable", disabled: true, mongoErr: repository.ErrUnavailable, status: 503, findFailures: 1},
		{name: "Mongo unexpected failure", disabled: true, mongoErr: errors.New("private-driver-error"), status: 500, findFailures: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeStore{destination: "https://example.com/?secret=destination", findErr: tc.mongoErr}
			h, _ := testHandlers(t, store)
			h.Observer = observability.New(nil)
			h.RedisRepo = nil
			if !tc.disabled {
				h.RedisRepo = &fakeCache{get: func(context.Context) (string, error) { return store.destination, tc.getErr }, set: func(context.Context) error { return tc.setErr }}
			}
			w := httptest.NewRecorder()
			h.RedirectHandler(w, httptest.NewRequest("GET", "/r/c", nil))
			if w.Code != tc.status {
				t.Fatalf("status %d", w.Code)
			}
			w = httptest.NewRecorder()
			h.Observer.MetricsHandler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
			metrics := w.Body.String()
			for _, outcome := range []string{"hit", "miss", "error"} {
				count := "0"
				if outcome == tc.readOutcome {
					count = "1"
				}
				want := `smallchop_cache_reads_total{outcome="` + outcome + `"} ` + count + "\n"
				if !strings.Contains(metrics, want) {
					t.Fatalf("missing %s", want)
				}
			}
			for _, outcome := range []string{"success", "error"} {
				count := "0"
				if outcome == tc.fillOutcome {
					count = "1"
				}
				if !strings.Contains(metrics, `smallchop_cache_fills_total{outcome="`+outcome+`"} `+count+"\n") {
					t.Fatalf("bad fill counter: %s", metrics)
				}
			}
			for _, failure := range []struct {
				dependency, operation string
				count                 int
			}{{"redis", "get", tc.getFailures}, {"redis", "set", tc.setFailures}, {"mongo", "find", tc.findFailures}} {
				count := "0"
				if failure.count == 1 {
					count = "1"
				}
				if !strings.Contains(metrics, `smallchop_dependency_failures_total{dependency="`+failure.dependency+`",operation="`+failure.operation+`"} `+count+"\n") {
					t.Fatalf("bad failure counter: %s", metrics)
				}
			}
		})
	}
}

func TestCreateFailuresExcludeInvalidInput(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		h, _ := testHandlers(t, &fakeStore{saveErr: repository.ErrUnavailable})
		h.Observer = observability.New(nil)
		body := "url=https%3A%2F%2Fexample.com"
		count := "1"
		if invalid {
			body = "url=invalid"
			count = "0"
		}
		h.ShortenURLHandler(httptest.NewRecorder(), formRequest(body))
		w := httptest.NewRecorder()
		h.Observer.MetricsHandler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
		if !strings.Contains(w.Body.String(), `smallchop_dependency_failures_total{dependency="mongo",operation="save"} `+count+"\n") {
			t.Fatal("input validation miscounted as dependency failure")
		}
	}
}
