package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync"
	"testing"
	"time"

	"gochop-it/internal/config"
)

func newTestLimiter() (*clientLimiter, *time.Time) {
	now := time.Unix(1000, 0)
	l := PerClientRateLimiter(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }), config.RatePolicy{Rate: 2, Burst: 4}, []netip.Addr{netip.MustParseAddr("172.30.80.2")}).(*clientLimiter)
	l.now = func() time.Time { return now }
	return l, &now
}

func request(l http.Handler, peer string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = peer
	for _, header := range headers {
		req.Header.Add("X-Forwarded-For", header)
	}
	w := httptest.NewRecorder()
	l.ServeHTTP(w, req)
	return w
}

func TestLimiterBurstAndRefill(t *testing.T) {
	l, now := newTestLimiter()
	for i := 0; i < 4; i++ {
		if got := request(l, "192.0.2.1:1000").Code; got != 204 {
			t.Fatalf("request %d = %d", i+1, got)
		}
	}
	w := request(l, "192.0.2.1:1001")
	if w.Code != 429 || w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("limit response: %v", w)
	}
	*now = now.Add(500 * time.Millisecond)
	if got := request(l, "192.0.2.1:1000").Code; got != 204 {
		t.Fatalf("refill = %d", got)
	}
	if got := request(l, "192.0.2.1:1000").Code; got != 429 {
		t.Fatalf("spent token = %d", got)
	}
}

func TestTrustedProxyClientSeparation(t *testing.T) {
	l, _ := newTestLimiter()
	for i := 0; i < 4; i++ {
		if got := request(l, "172.30.80.2:1000", "192.0.2.1").Code; got != 204 {
			t.Fatalf("request %d = %d", i, got)
		}
	}
	if got := request(l, "172.30.80.2:1000", "192.0.2.1").Code; got != 429 {
		t.Fatalf("client one = %d", got)
	}
	if got := request(l, "172.30.80.2:1000", "192.0.2.2").Code; got != 204 {
		t.Fatalf("client two = %d", got)
	}
}

func TestUntrustedHeadersCannotBypass(t *testing.T) {
	l, _ := newTestLimiter()
	for i := 0; i < 4; i++ {
		request(l, "192.0.2.1:1000", "198.51.100.1")
	}
	if got := request(l, "192.0.2.1:1000", "198.51.100.2").Code; got != 429 {
		t.Fatalf("spoof bypass = %d", got)
	}
	if got := request(l, "192.0.2.2:1000", "198.51.100.1").Code; got != 204 {
		t.Fatalf("distinct direct peer = %d", got)
	}
}

func TestClientIdentityValidation(t *testing.T) {
	cases := []struct {
		peer    string
		headers []string
		want    string
	}{
		{"[::1]:8080", nil, "::1"}, {"[::ffff:192.0.2.1]:8080", nil, "192.0.2.1"},
		{"172.30.80.2:8080", []string{"2001:db8::1"}, "2001:db8::1"},
		{"172.30.80.2:8080", []string{"::ffff:192.0.2.1"}, "192.0.2.1"},
		{"172.30.80.2:8080", nil, ""}, {"172.30.80.2:8080", []string{"bad"}, ""},
		{"172.30.80.2:8080", []string{"192.0.2.1, 192.0.2.2"}, ""},
		{"172.30.80.2:8080", []string{"192.0.2.1", "192.0.2.2"}, ""},
		{"172.30.80.2:8080", []string{"fe80::1%eth0"}, ""}, {"bad-peer", nil, ""},
	}
	for _, tc := range cases {
		l, _ := newTestLimiter()
		w := request(l, tc.peer, tc.headers...)
		if tc.want == "" {
			if w.Code != 400 {
				t.Errorf("%s %v = %d; want 400", tc.peer, tc.headers, w.Code)
			}
		} else if w.Code != 204 || l.clients[netip.MustParseAddr(tc.want)] == nil {
			t.Errorf("%s %v was not identified as %s", tc.peer, tc.headers, tc.want)
		}
	}
}

func TestLimiterIdleCleanup(t *testing.T) {
	l, now := newTestLimiter()
	request(l, "192.0.2.1:1000")
	*now = now.Add(4 * time.Minute)
	request(l, "192.0.2.2:1000")
	if len(l.clients) != 1 {
		t.Fatalf("idle buckets retained: %d", len(l.clients))
	}
	l.policy = config.RatePolicy{Rate: 0.001, Burst: 4}
	request(l, "192.0.2.3:1000")
	*now = now.Add(4 * time.Minute)
	request(l, "192.0.2.4:1000")
	if l.clients[netip.MustParseAddr("192.0.2.3")] == nil {
		t.Fatal("partially refilled bucket was reset")
	}
}

func TestLimiterConcurrentRequests(t *testing.T) {
	l, _ := newTestLimiter()
	var wg sync.WaitGroup
	codes := make(chan int, 32)
	for i := 0; i < 32; i++ {
		wg.Go(func() { codes <- request(l, "192.0.2.1:1000").Code })
	}
	wg.Wait()
	close(codes)
	allowed := 0
	for code := range codes {
		if code == 204 {
			allowed++
		} else if code != 429 {
			t.Fatalf("unexpected code: %d", code)
		}
	}
	if allowed != 4 {
		t.Fatalf("concurrent burst allowed %d; want 4", allowed)
	}
}
