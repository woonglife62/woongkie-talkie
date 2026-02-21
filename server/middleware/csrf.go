package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// CSRFProtect rejects mutating requests (POST/PUT/DELETE/PATCH) that do not
// carry the X-Requested-With: XMLHttpRequest header. GET/HEAD/OPTIONS and
// WebSocket upgrade requests are always allowed through.
func CSRFProtect() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			method := req.Method

			// Safe methods: pass through unconditionally.
			if method == http.MethodGet ||
				method == http.MethodHead ||
				method == http.MethodOptions {
				return next(c)
			}

			// WebSocket upgrade: pass through unconditionally.
			if req.Header.Get("Upgrade") == "websocket" {
				return next(c)
			}

			// Mutating request: require the AJAX sentinel header.
			if req.Header.Get("X-Requested-With") != "XMLHttpRequest" {
				return echo.NewHTTPError(http.StatusForbidden, "CSRF check failed: missing X-Requested-With header")
			}

			return next(c)
		}
	}
}
