package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// newTestWSLimiter creates an isolated wsConnLimiter for testing (not the global one).
func newTestWSLimiter(maxConn int64) *wsConnLimiter {
	return &wsConnLimiter{
		counts:  make(map[string]*ipEntry),
		maxConn: maxConn,
	}
}

// buildWSLimitMiddleware returns an Echo middleware backed by the given limiter.
func buildWSLimitMiddleware(l *wsConnLimiter) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Header.Get("Upgrade") != "websocket" {
				return next(c)
			}
			ip := c.RealIP()
			e := l.entry(ip)
			e.lastSeen.Store(time.Now().Unix())

			if e.count.Load() >= l.maxConn {
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "WebSocket 연결 한도를 초과했습니다.",
				})
			}
			e.count.Add(1)
			defer func() {
				e.count.Add(-1)
				e.lastSeen.Store(time.Now().Unix())
			}()
			return next(c)
		}
	}
}

// TestWSConnLimiter_AllowsUnderLimit verifies that connections up to the limit
// are accepted (HTTP 101 Switching Protocols is simulated by returning 200 from
// the stub handler, since real WS upgrade requires a live TCP conn).
func TestWSConnLimiter_AllowsUnderLimit(t *testing.T) {
	limiter := newTestWSLimiter(3)

	e := echo.New()
	e.Use(buildWSLimitMiddleware(limiter))
	e.GET("/ws", func(c echo.Context) error {
		return c.String(http.StatusOK, "connected")
	})

	ip := "10.1.1.1"
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/ws", nil)
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("X-Real-IP", ip)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "connection %d should be allowed", i+1)
	}
}

// TestWSConnLimiter_BlocksOverLimit verifies that once the per-IP connection
// limit is reached, further WebSocket upgrade requests receive 429.
func TestWSConnLimiter_BlocksOverLimit(t *testing.T) {
	limiter := newTestWSLimiter(2)
	ip := "10.2.2.2"

	// Pre-load the entry with count == maxConn to simulate "limit reached".
	entry := limiter.entry(ip)
	entry.count.Store(2)

	e := echo.New()
	e.Use(buildWSLimitMiddleware(limiter))
	e.GET("/ws", func(c echo.Context) error {
		return c.String(http.StatusOK, "connected")
	})

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("X-Real-IP", ip)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code,
		"request over limit should be rejected with 429")
}

// TestWSConnLimiter_NonWSRequestPassesThrough verifies that the middleware
// does NOT apply limits to non-WebSocket HTTP requests.
func TestWSConnLimiter_NonWSRequestPassesThrough(t *testing.T) {
	limiter := newTestWSLimiter(0) // maxConn=0 would block all WS, but not plain HTTP

	e := echo.New()
	e.Use(buildWSLimitMiddleware(limiter))
	e.GET("/api/rooms", func(c echo.Context) error {
		return c.String(http.StatusOK, "rooms")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/rooms", nil)
	// No "Upgrade: websocket" header — plain HTTP request.
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code,
		"non-WebSocket requests must bypass connection limiter")
}

// TestWSConnLimiter_Cleanup verifies that cleanup() removes IP entries that
// have zero active connections and a lastSeen timestamp older than 10 minutes.
func TestWSConnLimiter_Cleanup(t *testing.T) {
	l := newTestWSLimiter(10)

	// Create a stale entry (count=0, lastSeen > 10 min ago).
	staleEntry := l.entry("192.0.2.1")
	staleEntry.count.Store(0)
	staleEntry.lastSeen.Store(time.Now().Add(-11 * time.Minute).Unix())

	// Create a recent entry (count=0, lastSeen < 10 min ago).
	recentEntry := l.entry("192.0.2.2")
	recentEntry.count.Store(0)
	recentEntry.lastSeen.Store(time.Now().Unix())

	// Create an active entry (count>0, old lastSeen) — must NOT be removed.
	activeEntry := l.entry("192.0.2.3")
	activeEntry.count.Store(1)
	activeEntry.lastSeen.Store(time.Now().Add(-11 * time.Minute).Unix())

	l.cleanup()

	l.mu.Lock()
	defer l.mu.Unlock()

	_, staleExists := l.counts["192.0.2.1"]
	assert.False(t, staleExists, "stale zero-count entry should be removed by cleanup")

	_, recentExists := l.counts["192.0.2.2"]
	assert.True(t, recentExists, "recently-seen zero-count entry should be retained")

	_, activeExists := l.counts["192.0.2.3"]
	assert.True(t, activeExists, "entry with active connections should never be removed")
}

// TestWSConnLimiter_ConcurrentAccess verifies the limiter is safe under concurrent
// goroutine access (no data races, no panics).
func TestWSConnLimiter_ConcurrentAccess(t *testing.T) {
	limiter := newTestWSLimiter(100)

	e := echo.New()
	e.Use(buildWSLimitMiddleware(limiter))
	e.GET("/ws", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			req.Header.Set("Upgrade", "websocket")
			req.Header.Set("X-Real-IP", "10.0.0.99")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			// Just checking it doesn't panic; status may be 200 or 429.
		}()
	}
	wg.Wait()
}
