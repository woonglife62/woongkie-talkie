package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func ChatHTMLRender(c echo.Context) error {

	req := c.Request()
	user, _, _ := req.BasicAuth()

	return c.Render(
		http.StatusOK,
		"chat.html",
		map[string]interface{}{
			"user": user,
		},
	)
}
