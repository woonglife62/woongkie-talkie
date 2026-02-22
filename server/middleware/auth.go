package middleware

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/woonglife62/woongkie-talkie/pkg/config"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
)

func jwtAuth(e *echo.Echo) {
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path

			// 인증 불필요 경로 스킵 (명시적 목록)
			if path == "/auth/register" ||
				path == "/auth/login" ||
				path == "/auth/refresh" ||
				strings.HasPrefix(path, "/view/") ||
				path == "/login" ||
				path == "/" ||
				path == "/docs" ||
				strings.HasPrefix(path, "/docs/") {
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

			// Check if the user is blocked in DB (skip check if DB is unavailable)
			if user, dbErr := mongodb.FindUserByUsername(username); dbErr == nil && user.Role == "blocked" {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "차단된 계정입니다"})
			}

			c.Set("username", username)
			return next(c)
		}
	})
}
