package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// newCSRFEcho creates an Echo instance with CSRFProtect middleware applied.
func newCSRFEcho() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.Use(CSRFProtect())
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	e.POST("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	e.PUT("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	e.DELETE("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	return e
}

// TestCSRFProtect_POST_WithoutHeader verifies that POST without X-Requested-With returns 403.
func TestCSRFProtect_POST_WithoutHeader(t *testing.T) {
	e := newCSRFEcho()

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("body"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestCSRFProtect_POST_WithHeader verifies that POST with X-Requested-With: XMLHttpRequest passes.
func TestCSRFProtect_POST_WithHeader(t *testing.T) {
	e := newCSRFEcho()

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("body"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestCSRFProtect_PUT_WithoutHeader verifies that PUT without the header returns 403.
func TestCSRFProtect_PUT_WithoutHeader(t *testing.T) {
	e := newCSRFEcho()

	req := httptest.NewRequest(http.MethodPut, "/test", strings.NewReader("body"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestCSRFProtect_DELETE_WithoutHeader verifies that DELETE without the header returns 403.
func TestCSRFProtect_DELETE_WithoutHeader(t *testing.T) {
	e := newCSRFEcho()

	req := httptest.NewRequest(http.MethodDelete, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestCSRFProtect_GET_NotChecked verifies that GET requests pass without the header.
func TestCSRFProtect_GET_NotChecked(t *testing.T) {
	e := newCSRFEcho()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestCSRFProtect_POST_WrongHeaderValue verifies that a wrong header value returns 403.
func TestCSRFProtect_POST_WrongHeaderValue(t *testing.T) {
	e := newCSRFEcho()

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("body"))
	req.Header.Set("X-Requested-With", "fetch") // not "XMLHttpRequest"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestCSRFProtect_WebSocketUpgrade verifies that WebSocket upgrade requests pass without header.
func TestCSRFProtect_WebSocketUpgrade(t *testing.T) {
	e := newCSRFEcho()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Upgrade", "websocket")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// GET is already a safe method so it should always pass.
	assert.Equal(t, http.StatusOK, rec.Code)
}
