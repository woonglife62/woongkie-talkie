package middleware

import (
	"crypto/subtle"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func auth(e *echo.Echo) {
	e.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
		// Be careful to use constant time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(username), []byte("user1")) == 1 &&
			subtle.ConstantTimeCompare([]byte(password), []byte("passwd1")) == 1 {
			return true, nil
		}

		if subtle.ConstantTimeCompare([]byte(username), []byte("user2")) == 1 &&
			subtle.ConstantTimeCompare([]byte(password), []byte("passwd2")) == 1 {
			return true, nil
		}

		if subtle.ConstantTimeCompare([]byte(username), []byte("user3")) == 1 &&
			subtle.ConstantTimeCompare([]byte(password), []byte("passwd3")) == 1 {
			return true, nil
		}
		if subtle.ConstantTimeCompare([]byte(username), []byte("user4")) == 1 &&
			subtle.ConstantTimeCompare([]byte(password), []byte("passwd4")) == 1 {
			return true, nil
		}
		if subtle.ConstantTimeCompare([]byte(username), []byte("user5")) == 1 &&
			subtle.ConstantTimeCompare([]byte(password), []byte("passwd5")) == 1 {
			return true, nil
		}
		return false, nil
	}))

}
