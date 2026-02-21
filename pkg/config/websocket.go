package config

import (
	"net/http"
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
		// Also add with any port
		allowed = append(allowed, "localhost", "127.0.0.1")
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
func CheckOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	// No Origin header (e.g. non-browser clients or same-origin) – allow
	if origin == "" {
		return true
	}

	allowed := AllowedOrigins()
	for _, o := range allowed {
		if strings.EqualFold(origin, o) {
			return true
		}
		// Support prefix matching for localhost with arbitrary ports
		if strings.HasPrefix(o, "localhost") || strings.HasPrefix(o, "127.0.0.1") {
			// Strip scheme from origin for comparison
			stripped := strings.TrimPrefix(origin, "http://")
			stripped = strings.TrimPrefix(stripped, "https://")
			if strings.HasPrefix(stripped, o) {
				return true
			}
		}
	}
	return false
}
