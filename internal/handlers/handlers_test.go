package handlers

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"gochop-it/internal/repository"
	"gochop-it/internal/utils"
)

type fakeStore struct {
	destination          string
	saveErr              error
	findErr              error
	countErr             error
	saved                string
	saves, finds, counts int
	foundID, countedID   int64
}

func (s *fakeStore) SaveURL(_ context.Context, destination string) (string, error) {
	s.saves++
	s.saved = destination
	return utils.Encode(12345), s.saveErr
}
func (s *fakeStore) FindURLByID(_ context.Context, id int64) (*repository.URL, error) {
	s.finds++
	s.foundID = id
	if s.findErr != nil {
		return nil, s.findErr
	}
	return &repository.URL{ID: id, LongURL: s.destination}, nil
}
func (s *fakeStore) IncrementAccessCount(_ context.Context, id int64) error {
	s.counts++
	s.countedID = id
	return s.countErr
}

func testHandlers(t *testing.T, store *fakeStore) (*Handlers, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	tmpl, err := template.ParseFiles("../templates/index.html")
	if err != nil {
		t.Fatal(err)
	}
	return &Handlers{MongoRepo: store, RedisRepo: &repository.RedisRepo{Client: client}, Template: tmpl, PublicBaseURL: "https://short.example"}, server
}

func formRequest(body string) *http.Request {
	req := httptest.NewRequest("POST", "/shorten", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestRootHandler(t *testing.T) {
	h, _ := testHandlers(t, &fakeStore{})
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/", 200}, {"POST", "/", 405}, {"GET", "/unknown", 404},
	} {
		w := httptest.NewRecorder()
		h.RootHandler(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.status {
			t.Fatalf("%s %s = %d", tc.method, tc.path, w.Code)
		}
		if tc.status == 200 && !strings.Contains(w.Body.String(), "<form") {
			t.Fatal("real template form absent")
		}
		if tc.status == 405 && w.Header().Get("Allow") != "GET" {
			t.Fatal("missing Allow header")
		}
	}
}

func TestNewHandlers(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	h, err := NewHandlers(&fakeStore{}, nil, "https://short.example")
	if err != nil || h == nil || h.Template == nil {
		t.Fatalf("constructor: %v", err)
	}
	t.Chdir(t.TempDir())
	if _, err := NewHandlers(&fakeStore{}, nil, "https://short.example"); err == nil {
		t.Fatal("missing template accepted")
	}
}

func TestShortenURLHandler(t *testing.T) {
	store := &fakeStore{}
	h, _ := testHandlers(t, store)
	target := "https://example.com/a%2Fb?q=a%2Bb&x=2&x=1#installation"
	req := formRequest(url.Values{"url": {target}}.Encode())
	req.Host = "attacker.example"
	req.Header.Set("X-Forwarded-Host", "attacker.example")
	w := httptest.NewRecorder()
	h.ShortenURLHandler(w, req)
	code := utils.Encode(12345)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `href="/r/`+code+`"`) || !strings.Contains(w.Body.String(), "https://short.example/r/"+code) {
		t.Fatalf("unexpected creation response: %d %s", w.Code, w.Body.String())
	}
	if store.saved != target || store.saves != 1 || strings.Contains(w.Body.String(), "attacker.example") {
		t.Fatal("destination/origin contract failed")
	}
}

func TestShortenEscapesHTML(t *testing.T) {
	h, _ := testHandlers(t, &fakeStore{})
	h.PublicBaseURL = `https://short.example/"<script>` // Defense in depth if constructed outside startup config.
	w := httptest.NewRecorder()
	h.ShortenURLHandler(w, formRequest("url=https%3A%2F%2Fexample.com"))
	if w.Code != 200 || strings.Contains(w.Body.String(), "<script>") || !strings.Contains(w.Body.String(), "&lt;script&gt;") {
		t.Fatalf("unescaped output: %s", w.Body.String())
	}
}

func TestShortenRejectsInvalidInputBeforeStorage(t *testing.T) {
	cases := []struct {
		name, body string
		status     int
	}{
		{"missing", "", 400}, {"empty", "url=", 400}, {"relative", "url=example.com", 400},
		{"missing host", "url=https%3A%2F%2F%2Fpath", 400}, {"scheme", "url=ftp%3A%2F%2Fexample.com", 400},
		{"credentials", "url=https%3A%2F%2Fuser%40example.com", 400}, {"bad form encoding", "url=%ZZ", 400},
		{"duplicates", "url=https%3A%2F%2Fexample.com&url=https%3A%2F%2Fother.com", 400},
		{"URL limit", url.Values{"url": {"https://example.com/" + strings.Repeat("x", 2048)}}.Encode(), 400},
		{"body limit", "url=" + strings.Repeat("x", 16<<10), 413},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeStore{}
			h := &Handlers{MongoRepo: store}
			w := httptest.NewRecorder()
			h.ShortenURLHandler(w, formRequest(tc.body))
			if w.Code != tc.status || store.saves != 0 {
				t.Fatalf("status %d, saves %d; want %d, 0", w.Code, store.saves, tc.status)
			}
		})
	}
	h := &Handlers{}
	for _, req := range []*http.Request{
		httptest.NewRequest("GET", "/shorten", nil),
		formRequest(""),
		httptest.NewRequest("POST", "/shorten?url=https://example.com", nil),
	} {
		w := httptest.NewRecorder()
		h.ShortenURLHandler(w, req)
		if w.Code != 400 && w.Code != 405 {
			t.Fatalf("unexpected status: %d", w.Code)
		}
	}
}

func TestShortenStorageErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{utils.ErrInvalidURL, 400}, {errors.New("storage failed"), 500}} {
		h := &Handlers{MongoRepo: &fakeStore{saveErr: tc.err}}
		w := httptest.NewRecorder()
		h.ShortenURLHandler(w, formRequest("url=https%3A%2F%2Fexample.com"))
		if w.Code != tc.status {
			t.Fatalf("error %v = %d", tc.err, w.Code)
		}
	}
}

func TestRedirectHandlerColdAndWarm(t *testing.T) {
	target := "https://example.com/a%2Fb?q=a%2Bb&x=2&x=1#installation"
	store := &fakeStore{destination: target}
	h, cache := testHandlers(t, store)
	code := utils.Encode(12345)
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		h.RedirectHandler(w, httptest.NewRequest("GET", "/r/"+code, nil))
		if w.Code != 308 || w.Header().Get("Location") != target {
			t.Fatalf("redirect %d: %d %q", i, w.Code, w.Header().Get("Location"))
		}
	}
	if store.finds != 1 || store.counts != 2 || store.foundID != 12345 || store.countedID != 12345 {
		t.Fatalf("unexpected storage calls: %+v", store)
	}
	cached, err := cache.Get(code)
	if err != nil || cached != target || cache.TTL(code) <= 0 {
		t.Fatalf("cache = %q, %v", cached, err)
	}
}

func TestRedirectValidation(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"POST", "/r/c", 405}, {"GET", "/r/", 400}, {"GET", "/", 400},
		{"GET", "/r/b", 400}, {"GET", "/r/bc", 400}, {"GET", "/r/a", 400}, {"GET", "/r/" + strings.Repeat("9", 20), 400},
	} {
		w := httptest.NewRecorder()
		(&Handlers{}).RedirectHandler(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.status {
			t.Fatalf("%s %s = %d; want %d", tc.method, tc.path, w.Code, tc.status)
		}
	}
}

func TestRedirectAbsentMapping(t *testing.T) {
	h, _ := testHandlers(t, &fakeStore{findErr: mongo.ErrNoDocuments})
	w := httptest.NewRecorder()
	h.RedirectHandler(w, httptest.NewRequest("GET", "/r/c", nil))
	if w.Code != 404 {
		t.Fatalf("absent mapping = %d", w.Code)
	}
	// Duplicate fallback and dependency outage classification are Priority 2.
}

func TestCounterFailureDoesNotPreventRedirect(t *testing.T) {
	h, _ := testHandlers(t, &fakeStore{destination: "https://example.com", countErr: errors.New("counter failed")})
	w := httptest.NewRecorder()
	h.RedirectHandler(w, httptest.NewRequest("GET", "/r/c", nil))
	if w.Code != 308 {
		t.Fatalf("counter failure prevented redirect: %d", w.Code)
	}
}
