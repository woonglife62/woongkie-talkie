package middleware

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
	"github.com/woonglife62/woongkie-talkie/pkg/config"
)

func jwtAuth(e *echo.Echo) {
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path

			// 인증 불필요 경로 스킵
			if strings.HasPrefix(path, "/auth/") ||
				strings.HasPrefix(path, "/view/") ||
				path == "/login" ||
				path == "/" {
				return next(c)
			}

			if path == "/health" || path == "/ready" {
				return next(c)
			}

			tokenString := ""

			// Authorization: Bearer <token> 헤더에서 추출
			auth := c.Request().Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				tokenString = strings.TrimPrefix(auth, "Bearer ")
			}

			// WebSocket용 query param 폴백
			if tokenString == "" {
				tokenString = c.QueryParam("token")
			}

			if tokenString == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "인증 토큰이 필요합니다"})
			}

			// JWT 검증
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, echo.NewHTTPError(http.StatusUnauthorized, "잘못된 토큰입니다")
				}
				return []byte(config.JWTConfig.Secret), nil
			})

			if err != nil || !token.Valid {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "유효하지 않은 토큰입니다"})
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "유효하지 않은 토큰입니다"})
			}

			username, ok := claims["username"].(string)
			if !ok || username == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "유효하지 않은 토큰입니다"})
			}

			c.Set("username", username)
			return next(c)
		}
	})
}
