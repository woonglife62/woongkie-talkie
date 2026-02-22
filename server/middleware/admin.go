package middleware

import (
	"github.com/labstack/echo/v4"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
)

// AdminRequired returns middleware that checks if the user has role "admin".
func AdminRequired() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			username := c.Get("username")
			if username == nil || username == "" {
				return c.JSON(401, map[string]string{"error": "인증이 필요합니다"})
			}
			user, err := mongodb.FindUserByUsername(username.(string))
			if err != nil || user.Role != "admin" {
				return c.JSON(403, map[string]string{"error": "관리자 권한이 필요합니다"})
			}
			return next(c)
		}
	}
}
