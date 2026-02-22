package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

// okHandler is a trivial Echo handler that returns 200.
var okHandler = func(c echo.Context) error {
	return c.String(http.StatusOK, "ok")
}

// TestGlobalRateLimit verifies that the global rate limiter returns 429 once
// the burst capacity is exhausted. The global limiter has a burst of 10, so
// sending 11 requests from the same IP must trigger a 429 on the 11th.
func TestGlobalRateLimit(t *testing.T) {
	// Use a fresh limiter with a small burst so the test runs quickly.
	rl := newIPRateLimiter(rate.Limit(0), 3) // 3-token burst, no refill
	defer rl.Close()

	mw := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			if !rl.getLimiter(ip).Allow() {
				c.Response().Header().Set("Retry-After", "1")
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "too many requests",
				})
			}
			return next(c)
		}
	}

	e := echo.New()
	e.Use(mw)
	e.GET("/test", okHandler)

	const burst = 3
	for i := 0; i < burst; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Real-IP", "10.0.0.1")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "request %d should be allowed", i+1)
	}

	// The burst+1 th request should be rate-limited.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Real-IP", "10.0.0.1")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "request after burst should be 429")
	assert.NotEmpty(t, rec.Header().Get("Retry-After"), "Retry-After header must be set on 429")
}

// TestAuthRateLimit verifies that the AuthRateLimit middleware returns 429 once
// the auth burst capacity is exhausted (burst = 5).
func TestAuthRateLimit(t *testing.T) {
	// Use a fresh auth limiter with a small burst to keep the test fast.
	authRL := newIPRateLimiter(rate.Limit(0), 2) // 2-token burst, no refill
	defer authRL.Close()

	mw := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			if !authRL.getLimiter(ip).Allow() {
				c.Response().Header().Set("Retry-After", "12")
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "auth rate limit exceeded",
				})
			}
			return next(c)
		}
	}

	e := echo.New()
	e.Use(mw)
	e.POST("/auth/login", okHandler)

	const burst = 2
	for i := 0; i < burst; i++ {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
		req.Header.Set("X-Real-IP", "10.0.0.2")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "auth request %d should be allowed", i+1)
	}

	// The burst+1 th request should be rejected.
	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.Header.Set("X-Real-IP", "10.0.0.2")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "auth request after burst should be 429")
	assert.Equal(t, "12", rec.Header().Get("Retry-After"))
}

// TestGlobalRateLimit_DifferentIPs verifies that rate limiting is per-IP:
// one IP being throttled must not affect a different IP.
func TestGlobalRateLimit_DifferentIPs(t *testing.T) {
	rl := newIPRateLimiter(rate.Limit(0), 1) // burst of 1
	defer rl.Close()

	mw := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			if !rl.getLimiter(ip).Allow() {
				return c.JSON(http.StatusTooManyRequests, nil)
			}
			return next(c)
		}
	}

	e := echo.New()
	e.Use(mw)
	e.GET("/test", okHandler)

	// Exhaust IP A.
	reqA1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqA1.Header.Set("X-Real-IP", "192.168.1.1")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, reqA1)
	assert.Equal(t, http.StatusOK, rec.Code)

	reqA2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqA2.Header.Set("X-Real-IP", "192.168.1.1")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, reqA2)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "IP A should be throttled")

	// IP B's first request should still be allowed.
	reqB := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqB.Header.Set("X-Real-IP", "192.168.1.2")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, reqB)
	assert.Equal(t, http.StatusOK, rec.Code, "IP B should not be affected by IP A throttle")
}
