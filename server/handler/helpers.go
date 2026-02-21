package handler

import "github.com/labstack/echo/v4"

func GetUsername(c echo.Context) string {
	v, ok := c.Get("username").(string)
	if !ok {
		return ""
	}
	return v
}
