package middleware

import (
	"github.com/labstack/echo/v4"
	echoMiddle "github.com/labstack/echo/v4/middleware"
)

func Middleware(e *echo.Echo) {
	e.Use(echoMiddle.Logger())
	e.Use(echoMiddle.Recover())

	// auth
	auth(e)
}
