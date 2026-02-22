package handler

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
)

// SwaggerUIHandler serves the Swagger UI page.
// #278: SRI (Subresource Integrity) hashes added for CDN resources to prevent supply-chain attacks.
func SwaggerUIHandler(c echo.Context) error {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Woongkie-Talkie API Docs</title>
    <link rel="stylesheet"
          href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui.css"
          integrity="sha256-bgh8FMeGGEtcmBJNMf5kzFtfMEzI6tVmqBFXr+VKHE="
          crossorigin="anonymous">
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"
            integrity="sha256-sO4nmjbTWJJoMZEJ0/biBSDPPFUF9N4b+VN/6GYy2Yw="
            crossorigin="anonymous"></script>
    <script>
        SwaggerUIBundle({
            url: '/docs/openapi.yaml',
            dom_id: '#swagger-ui',
            presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
            layout: "StandaloneLayout"
        });
    </script>
</body>
</html>`
	return c.HTML(http.StatusOK, html)
}

func OpenAPISpecHandler(c echo.Context) error {
	// Try multiple paths for the spec file
	paths := []string{
		"/app/docs/openapi.yaml",
		"docs/openapi.yaml",
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		paths = append(paths, gopath+"/src/woongkie-talkie/docs/openapi.yaml")
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return c.File(p)
		}
	}
	return c.JSON(http.StatusNotFound, map[string]string{"error": "API spec not found"})
}
