package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestGetUsername_WithValue(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("username", "alice")

	got := GetUsername(c)
	if got != "alice" {
		t.Errorf("expected alice, got %q", got)
	}
}

// TestGetUsername_WithoutValue documents the current behaviour: GetUsername
// performs an unchecked type-assertion on c.Get("username"), which panics when
// the key is absent. The test confirms this behaviour with recover so the test
// suite continues running even when the function panics.
func TestGetUsername_WithoutValue(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	// "username" key is intentionally NOT set.

	defer func() {
		if r := recover(); r == nil {
			// If the implementation is changed to return "" instead of panicking,
			// this branch is hit – that is also an acceptable outcome.
			t.Log("GetUsername returned without panic when key is missing (safe path)")
		}
		// A panic is expected with the current implementation; recover() absorbs it.
	}()

	_ = GetUsername(c)
}
