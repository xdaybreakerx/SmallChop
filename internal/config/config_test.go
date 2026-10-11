package config

import (
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	values := map[string]string{
		"PUBLIC_BASE_URL":   "https://smallchop.net/",
		"TRUSTED_PROXY_IPS": "172.30.81.2, ::ffff:192.0.2.1",
		"REQUEST_TIMEOUT":   "4s", "MONGO_TIMEOUT": "1s", "CACHE_TIMEOUT": "100ms", "STARTUP_TIMEOUT": "8s",
		"CREATE_RATE_PER_SECOND": "3.5", "CREATE_RATE_BURST": "6",
	}
	cfg, err := load(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PublicBaseURL != "https://smallchop.net" || len(cfg.TrustedProxies) != 2 || cfg.TrustedProxies[1].String() != "192.0.2.1" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.Timeouts != (Timeouts{Request: 4 * time.Second, Mongo: time.Second, Cache: 100 * time.Millisecond, Startup: 8 * time.Second}) {
		t.Fatalf("unexpected timeouts: %+v", cfg.Timeouts)
	}
	if cfg.CreatePolicy.Rate != 3.5 || cfg.CreatePolicy.Burst != 6 || cfg.RedirectPolicy.Rate != 10 || cfg.RedirectPolicy.Burst != 20 {
		t.Fatalf("unexpected policies: %+v", cfg)
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("PUBLIC_BASE_URL", "http://127.0.0.1:8080")
	t.Setenv("TRUSTED_PROXY_IPS", "")
	for _, key := range []string{"CACHE_ENABLED", "CREATE_RATE_PER_SECOND", "CREATE_RATE_BURST", "REDIRECT_RATE_PER_SECOND", "REDIRECT_RATE_BURST", "REQUEST_TIMEOUT", "MONGO_TIMEOUT", "CACHE_TIMEOUT", "STARTUP_TIMEOUT"} {
		t.Setenv(key, "")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.CacheEnabled || cfg.Timeouts != DefaultTimeouts() {
		t.Fatalf("unexpected timeouts: %+v", cfg.Timeouts)
	}
	if cfg.CreatePolicy.Rate != 2 || cfg.CreatePolicy.Burst != 4 || len(cfg.TrustedProxies) != 0 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadInvalid(t *testing.T) {
	cases := []struct{ key, value string }{
		{"PUBLIC_BASE_URL", ""}, {"PUBLIC_BASE_URL", "https:///path"}, {"PUBLIC_BASE_URL", "ftp://example.com"},
		{"PUBLIC_BASE_URL", "https://user@example.com"}, {"PUBLIC_BASE_URL", "https://example.com/path"},
		{"PUBLIC_BASE_URL", "https://example.com/%2F"}, {"PUBLIC_BASE_URL", "https://example.com?"},
		{"PUBLIC_BASE_URL", "https://example.com#"}, {"PUBLIC_BASE_URL", "https://example.com?q=1"},
		{"PUBLIC_BASE_URL", "https://example.com:0"}, {"PUBLIC_BASE_URL", "https://example.com:99999"}, {"PUBLIC_BASE_URL", "http://example.com:"},
		{"TRUSTED_PROXY_IPS", "172.16.0.0/12"}, {"TRUSTED_PROXY_IPS", "caddy"}, {"TRUSTED_PROXY_IPS", "172.30.80.2,"},
		{"CREATE_RATE_PER_SECOND", "0"}, {"CREATE_RATE_PER_SECOND", "NaN"}, {"CREATE_RATE_PER_SECOND", "Inf"},
		{"REQUEST_TIMEOUT", "0"}, {"MONGO_TIMEOUT", "-1s"}, {"CACHE_TIMEOUT", "broken"}, {"STARTUP_TIMEOUT", "61s"},
		{"REDIRECT_RATE_PER_SECOND", "-1"}, {"CREATE_RATE_BURST", "0"}, {"REDIRECT_RATE_BURST", "no"},
		{"CACHE_ENABLED", "1"}, {"CACHE_ENABLED", "FALSE"}, {"CACHE_ENABLED", "broken"},
	}
	for _, tc := range cases {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			_, err := load(func(key string) string {
				if key == tc.key {
					return tc.value
				}
				if key == "PUBLIC_BASE_URL" {
					return "https://example.com"
				}
				return ""
			})
			if err == nil {
				t.Fatal("invalid configuration was accepted")
			}
		})
	}
}

func TestCacheDisabled(t *testing.T) {
	cfg, err := load(func(key string) string {
		if key == "PUBLIC_BASE_URL" {
			return "https://example.com"
		}
		if key == "CACHE_ENABLED" {
			return "false"
		}
		return ""
	})
	if err != nil || cfg.CacheEnabled {
		t.Fatalf("cache bypass: %+v, %v", cfg, err)
	}
}
