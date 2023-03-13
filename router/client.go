package router

import (
	"html/template"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

type templateRenderer struct {
	templates *template.Template
}

// Render renders a template document
func (t *templateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func chatHTMLRender(c echo.Context) error {

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

func clientRouter(e *echo.Echo) {
	e.Renderer = &templateRenderer{
		templates: template.Must(template.ParseGlob("/home/wclee/go/src/server/view/*.html")),
	}
	e.GET("/client", chatHTMLRender)
	//e.GET("/client/:group")
}
