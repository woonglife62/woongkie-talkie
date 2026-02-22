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
	done     chan struct{}
}

func newIPRateLimiter(r rate.Limit, b int) *ipRateLimiter {
	rl := &ipRateLimiter{
		visitors: make(map[string]*visitor),
		r:        r,
		b:        b,
		done:     make(chan struct{}),
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

// Close stops the cleanup goroutine.
func (rl *ipRateLimiter) Close() {
	close(rl.done)
}

// cleanupLoop removes visitors that haven't been seen in 10 minutes.
func (rl *ipRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			for ip, v := range rl.visitors {
				if time.Since(v.lastSeen) > 10*time.Minute {
					delete(rl.visitors, ip)
				}
			}
			rl.mu.Unlock()
		case <-rl.done:
			return
		}
	}
}

// globalLimiter: 100 requests/minute per IP, burst of 10
var globalLimiter = newIPRateLimiter(rate.Every(time.Minute/100), 10)

// authLimiter: stricter limit for auth endpoints — 5 requests/minute per IP, burst of 5
var authLimiter = newIPRateLimiter(rate.Every(12*time.Second), 5)

// roomCreateLimiter: 5 room creations/minute per IP, burst of 5
var roomCreateLimiter = newIPRateLimiter(rate.Every(12*time.Second), 5)

// RateLimit returns a middleware that applies per-IP rate limiting (100 req/min).
func RateLimit() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			if !globalLimiter.getLimiter(ip).Allow() {
				c.Response().Header().Set("Retry-After", "1")
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
				c.Response().Header().Set("Retry-After", "12")
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "인증 요청이 너무 많습니다. 잠시 후 다시 시도해주세요",
				})
			}
			return next(c)
		}
	}
}

// RoomCreateRateLimit returns a middleware for room creation — 5 rooms/minute per IP.
func RoomCreateRateLimit() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			if !roomCreateLimiter.getLimiter(ip).Allow() {
				c.Response().Header().Set("Retry-After", "12")
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "채팅방 생성 요청이 너무 많습니다. 잠시 후 다시 시도해주세요",
				})
			}
			return next(c)
		}
	}
}

// NewWSMessageLimiter creates a per-client WebSocket message rate limiter.
// Rate: 1 message per 2 seconds (30 msg/min), burst of 5 (#196).
func NewWSMessageLimiter() *rate.Limiter {
	return rate.NewLimiter(rate.Every(2*time.Second), 5)
}
