package utils

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestEncodeDecode(t *testing.T) {
	for _, id := range []int64{1, 2, base - 1, base, base + 1, 12345, math.MaxInt32, math.MaxInt64} {
		if got := Decode(Encode(id)); got != id {
			t.Errorf("Decode(Encode(%d)) = %d", id, got)
		}
	}
	if Encode(1) != "c" || Encode(base) != "cb" {
		t.Fatal("existing code alphabet changed")
	}
}

func TestDecodeInvalid(t *testing.T) {
	for _, code := range []string{"", "b", "bc", "bb", "a", "c/", "c?", "c ", "é", strings.Repeat("9", 20), Encode(math.MaxInt64) + "c"} {
		if got := Decode(code); got != -1 {
			t.Errorf("Decode(%q) = %d; want invalid", code, got)
		}
	}
}

func TestSanitizeURL(t *testing.T) {
	valid := []string{
		"https://example.com", "http://localhost:8080/path", "http://127.0.0.1/",
		"https://example.com/a%2Fb?q=a%2Bb&x=2&x=1#installation",
		"https://example.com/?text=javascript:%3Cscript%3E", "http://[::1]:8080/",
		"https://example.com/path?", "https://example.com/#", "https://example.com/" + strings.Repeat("x", 2028),
	}
	for _, raw := range valid {
		got, err := SanitizeURL(raw)
		if err != nil || got != raw {
			t.Errorf("SanitizeURL(%q) = %q, %v; want unchanged", raw, got, err)
		}
	}
	invalid := []string{
		"", "example.com", "https:///path", "https://", "javascript:alert(1)", "ftp://example.com",
		"https://user:password@example.com/", "https://example.com/%ZZ", "https://example.com/?q=%ZZ",
		"https://example.com/a b", "https://example.com/\n", "http://example.com:99999/", "http://example.com:/",
		"https://example.com/" + strings.Repeat("x", 2029),
	}
	for _, raw := range invalid {
		if _, err := SanitizeURL(raw); !errors.Is(err, ErrInvalidURL) {
			t.Errorf("SanitizeURL(%q) error = %v; want ErrInvalidURL", raw, err)
		}
	}
}

func FuzzEncodeDecode(f *testing.F) {
	for _, id := range []int64{1, 51, 52, 12345, math.MaxInt64} {
		f.Add(id)
	}
	f.Fuzz(func(t *testing.T, id int64) {
		if id <= 0 {
			return
		}
		if got := Decode(Encode(id)); got != id {
			t.Fatalf("round trip %d = %d", id, got)
		}
	})
}

func FuzzDecode(f *testing.F) {
	for _, code := range []string{"", "b", "bc", "c", "cb", "é", strings.Repeat("9", 20), Encode(math.MaxInt64)} {
		f.Add(code)
	}
	f.Fuzz(func(t *testing.T, code string) {
		id := Decode(code)
		if id == -1 {
			return
		}
		if id <= 0 || Encode(id) != code {
			t.Fatalf("accepted noncanonical/invalid code %q = %d", code, id)
		}
	})
}
