package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// newSecurityEcho creates an Echo instance with securityHeaders middleware applied.
func newSecurityEcho() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	securityHeaders(e)
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	return e
}

// TestSecurityHeaders_XFrameOptions verifies that X-Frame-Options header is set.
func TestSecurityHeaders_XFrameOptions(t *testing.T) {
	e := newSecurityEcho()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
}

// TestSecurityHeaders_XContentTypeOptions verifies that X-Content-Type-Options header is set to nosniff.
func TestSecurityHeaders_XContentTypeOptions(t *testing.T) {
	e := newSecurityEcho()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
}

// TestSecurityHeaders_CSP verifies that Content-Security-Policy header is set.
func TestSecurityHeaders_CSP(t *testing.T) {
	e := newSecurityEcho()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	csp := rec.Header().Get("Content-Security-Policy")
	assert.NotEmpty(t, csp, "Content-Security-Policy header should be set")
	assert.Contains(t, csp, "default-src")
}

// TestSecurityHeaders_XSSProtection verifies that X-XSS-Protection header is set.
func TestSecurityHeaders_XSSProtection(t *testing.T) {
	e := newSecurityEcho()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("X-XSS-Protection"))
}
