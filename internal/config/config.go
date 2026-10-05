package config

import (
	"fmt"
	"math"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"

	"golang.org/x/time/rate"
)

type RatePolicy struct {
	Rate  rate.Limit
	Burst int
}

type Config struct {
	PublicBaseURL  string
	TrustedProxies []netip.Addr
	CreatePolicy   RatePolicy
	RedirectPolicy RatePolicy
}

func Load() (Config, error) { return load(os.Getenv) }

func load(getenv func(string) string) (Config, error) {
	var cfg Config
	base, err := url.Parse(getenv("PUBLIC_BASE_URL"))
	if err != nil || base == nil || (base.Scheme != "http" && base.Scheme != "https") ||
		base.Hostname() == "" || base.User != nil || base.Opaque != "" ||
		(base.Path != "" && base.Path != "/") || base.RawPath != "" || base.RawQuery != "" || base.ForceQuery ||
		base.Fragment != "" || strings.Contains(getenv("PUBLIC_BASE_URL"), "#") {
		return cfg, fmt.Errorf("PUBLIC_BASE_URL must be an absolute HTTP(S) origin without credentials, query, fragment, or path prefix")
	}
	if port := base.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return cfg, fmt.Errorf("PUBLIC_BASE_URL has an invalid port")
		}
	}
	if strings.HasSuffix(base.Host, ":") {
		return cfg, fmt.Errorf("PUBLIC_BASE_URL has an empty port")
	}
	cfg.PublicBaseURL = strings.TrimSuffix(base.String(), "/")
	if raw := getenv("TRUSTED_PROXY_IPS"); raw != "" {
		for _, value := range strings.Split(raw, ",") {
			ip, err := netip.ParseAddr(strings.TrimSpace(value))
			if err != nil || ip.Zone() != "" {
				return cfg, fmt.Errorf("TRUSTED_PROXY_IPS must contain exact IP addresses, not ranges or hostnames")
			}
			cfg.TrustedProxies = append(cfg.TrustedProxies, ip.Unmap())
		}
	}
	cfg.CreatePolicy, err = policy(getenv, "CREATE", 2, 4)
	if err != nil {
		return cfg, err
	}
	cfg.RedirectPolicy, err = policy(getenv, "REDIRECT", 10, 20)
	return cfg, err
}

func policy(getenv func(string) string, prefix string, defaultRate float64, defaultBurst int) (RatePolicy, error) {
	p := RatePolicy{Rate: rate.Limit(defaultRate), Burst: defaultBurst}
	if raw := getenv(prefix + "_RATE_PER_SECOND"); raw != "" {
		n, err := strconv.ParseFloat(raw, 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n <= 0 {
			return p, fmt.Errorf("%s_RATE_PER_SECOND must be a finite positive number", prefix)
		}
		p.Rate = rate.Limit(n)
	}
	if raw := getenv(prefix + "_RATE_BURST"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return p, fmt.Errorf("%s_RATE_BURST must be a positive integer", prefix)
		}
		p.Burst = n
	}
	return p, nil
}
