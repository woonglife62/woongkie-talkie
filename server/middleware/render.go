package middleware

import (
	"html/template"
	"io"

	"github.com/labstack/echo/v4"
)

type templateRenderer struct {
	templates *template.Template
}

// Render renders a template document
func (t *templateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func render(e *echo.Echo) {

	e.Renderer = &templateRenderer{
		templates: template.Must(template.ParseGlob("view/*.html")),
	}
}
