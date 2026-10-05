package config

import "testing"

func TestLoad(t *testing.T) {
	values := map[string]string{
		"PUBLIC_BASE_URL":        "https://smallchop.net/",
		"TRUSTED_PROXY_IPS":      "172.30.81.2, ::ffff:192.0.2.1",
		"CREATE_RATE_PER_SECOND": "3.5", "CREATE_RATE_BURST": "6",
	}
	cfg, err := load(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PublicBaseURL != "https://smallchop.net" || len(cfg.TrustedProxies) != 2 || cfg.TrustedProxies[1].String() != "192.0.2.1" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.CreatePolicy.Rate != 3.5 || cfg.CreatePolicy.Burst != 6 || cfg.RedirectPolicy.Rate != 10 || cfg.RedirectPolicy.Burst != 20 {
		t.Fatalf("unexpected policies: %+v", cfg)
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("PUBLIC_BASE_URL", "http://127.0.0.1:8080")
	t.Setenv("TRUSTED_PROXY_IPS", "")
	for _, key := range []string{"CREATE_RATE_PER_SECOND", "CREATE_RATE_BURST", "REDIRECT_RATE_PER_SECOND", "REDIRECT_RATE_BURST"} {
		t.Setenv(key, "")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
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
		{"REDIRECT_RATE_PER_SECOND", "-1"}, {"CREATE_RATE_BURST", "0"}, {"REDIRECT_RATE_BURST", "no"},
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
