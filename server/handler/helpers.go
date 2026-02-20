package handler

import "github.com/labstack/echo/v4"

func GetUsername(c echo.Context) string {
	return c.Get("username").(string)
}
