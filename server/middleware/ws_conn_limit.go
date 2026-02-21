package middleware

import (
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/labstack/echo/v4"
)

const defaultMaxWSConnsPerIP = 10

// wsConnLimiter tracks active WebSocket connections per IP address.
type wsConnLimiter struct {
	mu      sync.Mutex
	counts  map[string]*atomic.Int64
	maxConn int64
}

var globalWSLimiter = &wsConnLimiter{
	counts:  make(map[string]*atomic.Int64),
	maxConn: defaultMaxWSConnsPerIP,
}

func (l *wsConnLimiter) counter(ip string) *atomic.Int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	if c, ok := l.counts[ip]; ok {
		return c
	}
	c := &atomic.Int64{}
	l.counts[ip] = c
	return c
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
			counter := globalWSLimiter.counter(ip)

			if counter.Load() >= globalWSLimiter.maxConn {
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "WebSocket 연결 한도를 초과했습니다. 잠시 후 다시 시도해주세요.",
				})
			}

			counter.Add(1)
			defer counter.Add(-1)

			return next(c)
		}
	}
}
