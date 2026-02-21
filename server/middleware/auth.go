package middleware

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
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

			// httpOnly 쿠키에서 추출 (헤더 없을 때 폴백)
			if tokenString == "" {
				if cookie, err := c.Cookie("auth_token"); err == nil && cookie.Value != "" {
					tokenString = cookie.Value
				}
			}

			// WebSocket용 query param 폴백
			if tokenString == "" {
				tokenString = c.QueryParam("token")
			}

			if tokenString == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "인증 토큰이 필요합니다"})
			}

			// JWT 검증 (v5: WithValidMethods로 알고리즘 제한)
			token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
				return []byte(config.JWTConfig.Secret), nil
			}, jwt.WithValidMethods([]string{"HS256"}))

			if err != nil || !token.Valid {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "유효하지 않은 토큰입니다"})
			}

			claims, ok := token.Claims.(*jwt.RegisteredClaims)
			if !ok {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "유효하지 않은 토큰입니다"})
			}

			username := claims.Subject
			if username == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "유효하지 않은 토큰입니다"})
			}

			c.Set("username", username)
			return next(c)
		}
	})
}
