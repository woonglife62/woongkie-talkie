package middleware

import (
	"time"

	"github.com/labstack/echo/v4"
	echoMiddle "github.com/labstack/echo/v4/middleware"
	"github.com/woonglife62/woongkie-talkie/pkg/logger"
)

func Middleware(e *echo.Echo) {
	e.Use(echoMiddle.Logger())
	e.Use(echoMiddle.Recover())

	// security headers
	securityHeaders(e)

	// global rate limiting
	e.Use(RateLimit())

	// CSRF protection: require X-Requested-With header on mutating requests
	e.Use(CSRFProtect())

	// auth
	jwtAuth(e)

	// request logging
	requestLogger(e)

	// render
	render(e)
}

func requestLogger(e *echo.Echo) {
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip health endpoints
			path := c.Request().URL.Path
			if path == "/health" || path == "/ready" {
				return next(c)
			}

			start := time.Now()
			err := next(c)
			latency := time.Since(start)

			req := c.Request()
			res := c.Response()

			logger.Logger.Infow("request",
				"method", req.Method,
				"path", path,
				"status", res.Status,
				"latency_ms", latency.Milliseconds(),
				"ip", c.RealIP(),
				"user_agent", req.UserAgent(),
			)

			return err
		}
	})
}
