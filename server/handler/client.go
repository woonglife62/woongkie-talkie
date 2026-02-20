package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func ChatHTMLRender(c echo.Context) error {
	return c.Render(
		http.StatusOK,
		"chat.html",
		map[string]interface{}{},
	)
}

func LoginPageRender(c echo.Context) error {
	return c.Render(
		http.StatusOK,
		"login.html",
		map[string]interface{}{},
	)
}
