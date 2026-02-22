package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// newTestEcho returns a minimal Echo instance for handler unit tests.
func newTestEcho() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	return e
}

// TestRegisterHandler_UsernameTooShort verifies that a username shorter than
// 3 characters is rejected with 400 before any DB call is made.
func TestRegisterHandler_UsernameTooShort(t *testing.T) {
	e := newTestEcho()

	body := `{"username":"ab","password":"secret123","display_name":"AB"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := RegisterHandler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "3자 이상")
}

// TestRegisterHandler_PasswordTooShort verifies that a password shorter than
// 8 characters is rejected with 400 before any DB call is made.
func TestRegisterHandler_PasswordTooShort(t *testing.T) {
	e := newTestEcho()

	body := `{"username":"alice","password":"abc","display_name":"Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := RegisterHandler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "8자 이상")
}

// TestRegisterHandler_InvalidJSON verifies that a malformed JSON body returns
// 400 without reaching the DB.
func TestRegisterHandler_InvalidJSON(t *testing.T) {
	e := newTestEcho()

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString("{bad json"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := RegisterHandler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestLoginHandler_InvalidJSON verifies that a malformed JSON body returns 400.
func TestLoginHandler_InvalidJSON(t *testing.T) {
	e := newTestEcho()

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString("not-json"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := LoginHandler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestRegisterHandler_WithMongoDB tests successful registration end-to-end.
// Requires a running MongoDB instance (run inside Docker via docker-compose).
func TestRegisterHandler_WithMongoDB(t *testing.T) {
	t.Skip("requires MongoDB: run with integration test setup (docker-compose)")
}

// TestLoginHandler_WithMongoDB tests login with valid credentials end-to-end.
// Requires a running MongoDB instance (run inside Docker via docker-compose).
func TestLoginHandler_WithMongoDB(t *testing.T) {
	t.Skip("requires MongoDB: run with integration test setup (docker-compose)")
}
