package middleware

import (
	"github.com/labstack/echo/v4"
	"github.com/woonglife62/woongkie-talkie/pkg/config"
)

func securityHeaders(e *echo.Echo) {
	tlsEnabled := config.TLSConfig.CertFile != "" && config.TLSConfig.KeyFile != ""
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("X-Content-Type-Options", "nosniff")
			c.Response().Header().Set("X-Frame-Options", "DENY")
			c.Response().Header().Set("X-XSS-Protection", "1; mode=block")
			c.Response().Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			c.Response().Header().Set("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self' 'unsafe-inline' https://code.jquery.com https://cdn.jsdelivr.net; "+
					"style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; "+
					"img-src 'self' data:; "+
					"connect-src 'self' ws: wss:; "+
					"font-src 'self' https://cdn.jsdelivr.net")
			if tlsEnabled {
				c.Response().Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			return next(c)
		}
	})
}
