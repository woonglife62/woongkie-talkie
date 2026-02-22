package config

import (
	"net/http"
	"net/url"
	"os"
	"strings"
)

// AllowedOrigins returns the list of allowed WebSocket origins.
// In dev mode, localhost and 127.0.0.1 are always allowed.
// Additional origins can be set via ALLOWED_ORIGINS (comma-separated).
func AllowedOrigins() []string {
	allowed := []string{}

	// Always allow localhost in dev mode
	if Config.IsDev == "DEV" || Config.IsDev == "dev" || Config.IsDev == "develop" {
		allowed = append(allowed,
			"http://localhost",
			"https://localhost",
			"http://127.0.0.1",
			"https://127.0.0.1",
		)
	}

	// Add from environment variable
	if envOrigins := os.Getenv("ALLOWED_ORIGINS"); envOrigins != "" {
		for _, o := range strings.Split(envOrigins, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				allowed = append(allowed, o)
			}
		}
	}

	return allowed
}

// CheckOrigin validates WebSocket upgrade requests against allowed origins.
// It uses url.Parse to extract the exact hostname, preventing prefix-spoofing
// attacks (e.g. "localhost.evil.com" matching a "localhost" prefix check).
// When no origins are configured, falls back to same-host check (#225).
func CheckOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	// No Origin header (e.g. non-browser clients or same-origin) – allow
	if origin == "" {
		return true
	}

	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}
	originHost := originURL.Hostname() // strips port, returns bare hostname

	allowed := AllowedOrigins()
	// If no origins configured, fall back to same-host comparison (#225).
	// This prevents blocking all browsers in production when ALLOWED_ORIGINS is unset.
	if len(allowed) == 0 {
		reqHost := r.Host
		if i := strings.LastIndex(reqHost, ":"); i >= 0 {
			reqHost = reqHost[:i]
		}
		return strings.EqualFold(originHost, reqHost)
	}

	for _, o := range allowed {
		allowedURL, err := url.Parse(o)
		if err != nil {
			continue
		}
		allowedHost := allowedURL.Hostname()

		// Scheme must match
		if !strings.EqualFold(originURL.Scheme, allowedURL.Scheme) {
			continue
		}
		// Exact hostname match (case-insensitive); port is ignored to
		// allow arbitrary dev ports (e.g. localhost:3000, localhost:5173).
		if strings.EqualFold(originHost, allowedHost) {
			return true
		}
	}
	return false
}
