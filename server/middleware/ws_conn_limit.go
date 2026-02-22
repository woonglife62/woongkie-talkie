package middleware

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/labstack/echo/v4"
)

const defaultMaxWSConnsPerIP = 10

// ipEntry holds a connection counter and the last-seen timestamp for cleanup.
type ipEntry struct {
	count    atomic.Int64
	lastSeen atomic.Int64 // Unix seconds
}

// wsConnLimiter tracks active WebSocket connections per IP address.
// A background goroutine periodically removes idle entries to prevent
// unbounded memory growth (#255).
type wsConnLimiter struct {
	mu      sync.Mutex
	counts  map[string]*ipEntry
	maxConn int64
}

var globalWSLimiter = &wsConnLimiter{
	counts:  make(map[string]*ipEntry),
	maxConn: defaultMaxWSConnsPerIP,
}

func init() {
	// Periodically remove IP entries with zero connections idle for >10 min (#255).
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			globalWSLimiter.cleanup()
		}
	}()
}

// cleanup removes IP entries that have no active connections and have not been
// seen in the last 10 minutes.
func (l *wsConnLimiter) cleanup() {
	cutoff := time.Now().Add(-10 * time.Minute).Unix()
	l.mu.Lock()
	defer l.mu.Unlock()
	for ip, e := range l.counts {
		if e.count.Load() == 0 && e.lastSeen.Load() < cutoff {
			delete(l.counts, ip)
		}
	}
}

func (l *wsConnLimiter) entry(ip string) *ipEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e, ok := l.counts[ip]; ok {
		return e
	}
	e := &ipEntry{}
	e.lastSeen.Store(time.Now().Unix())
	l.counts[ip] = e
	return e
}

// WSConnLimit returns an Echo middleware that limits concurrent WebSocket
// connections per client IP to maxConn (default 10).
func WSConnLimit() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Only apply to WebSocket upgrade requests.
			if c.Request().Header.Get("Upgrade") != "websocket" {
				return next(c)
			}

			ip := c.RealIP()
			e := globalWSLimiter.entry(ip)
			e.lastSeen.Store(time.Now().Unix())

			if e.count.Load() >= globalWSLimiter.maxConn {
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "WebSocket 연결 한도를 초과했습니다. 잠시 후 다시 시도해주세요.",
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
