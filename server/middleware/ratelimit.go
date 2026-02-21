package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type ipRateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	r        rate.Limit
	b        int
}

func newIPRateLimiter(r rate.Limit, b int) *ipRateLimiter {
	rl := &ipRateLimiter{
		visitors: make(map[string]*visitor),
		r:        r,
		b:        b,
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *ipRateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(rl.r, rl.b)
		rl.visitors[ip] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}
	v.lastSeen = time.Now()
	return v.limiter
}

// cleanupLoop removes visitors that haven't been seen in 10 minutes.
func (rl *ipRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > 10*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// globalLimiter: 20 requests/second per IP, burst of 40
var globalLimiter = newIPRateLimiter(20, 40)

// authLimiter: stricter limit for auth endpoints — 5 requests/minute per IP, burst of 5
var authLimiter = newIPRateLimiter(rate.Every(12*time.Second), 5)

// RateLimit returns a middleware that applies per-IP rate limiting.
func RateLimit() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			if !globalLimiter.getLimiter(ip).Allow() {
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "요청이 너무 많습니다. 잠시 후 다시 시도해주세요",
				})
			}
			return next(c)
		}
	}
}

// AuthRateLimit returns a stricter middleware for authentication endpoints.
func AuthRateLimit() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			if !authLimiter.getLimiter(ip).Allow() {
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "인증 요청이 너무 많습니다. 잠시 후 다시 시도해주세요",
				})
			}
			return next(c)
		}
	}
}
