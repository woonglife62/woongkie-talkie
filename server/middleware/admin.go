package middleware

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
)

// AdminRequired returns middleware that checks if the user has role "admin".
// #221: DB errors return 503 Service Unavailable rather than silently returning 403,
//
//	preventing false "not an admin" responses when the DB is down.
//
// #250: Role is verified via DB lookup. For higher throughput, embed role in JWT claims.
func AdminRequired() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			username := c.Get("username")
			if username == nil || username == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "인증이 필요합니다"})
			}
			user, err := mongodb.FindUserByUsername(username.(string))
			if err != nil {
				// #221: distinguish "not found" (403) from DB failure (503)
				if errors.Is(err, mongodb.ErrNotFound) {
					return c.JSON(http.StatusForbidden, map[string]string{"error": "관리자 권한이 필요합니다"})
				}
				return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "서비스를 일시적으로 사용할 수 없습니다"})
			}
			if user.Role != "admin" {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "관리자 권한이 필요합니다"})
			}
			return next(c)
		}
	}
}
