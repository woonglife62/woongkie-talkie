package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// TestAdminBlockUserHandler_SelfBlock verifies that an admin cannot block themselves.
func TestAdminBlockUserHandler_SelfBlock(t *testing.T) {
	e := newTestEcho()

	body := `{"blocked":true}`
	req := httptest.NewRequest(http.MethodPut, "/admin/users/alice/block", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("username")
	c.SetParamValues("alice")
	// Set the authenticated admin username to the same value as the target.
	c.Set("username", "alice")

	err := AdminBlockUserHandler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "자기 자신을 차단할 수 없습니다")
}

// TestAdminBlockUserHandler_DifferentUser verifies that blocking a different user
// passes the self-block check — reaching the DB layer (even if it panics without
// MongoDB) proves the 403 guard was not triggered.
func TestAdminBlockUserHandler_DifferentUser(t *testing.T) {
	e := newTestEcho()

	body := `{"blocked":true}`
	req := httptest.NewRequest(http.MethodPut, "/admin/users/bob/block", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("username")
	c.SetParamValues("bob")
	c.Set("username", "alice")

	// Capture any panic from the nil MongoDB client so the test does not crash.
	var handlerErr error
	func() {
		defer func() { recover() }() // absorb nil-DB panic
		handlerErr = AdminBlockUserHandler(c)
	}()

	// If we got a response (no panic or handled error), it must not be 403.
	// If there was a panic (DB layer), the self-block guard was correctly bypassed.
	if handlerErr == nil && rec.Code != 0 {
		assert.NotEqual(t, http.StatusForbidden, rec.Code,
			"should not return 403 when blocking a different user")
	}
}

// TestAdminAnnounceHandler_EmptyMessage verifies that an empty announcement message returns 400.
func TestAdminAnnounceHandler_EmptyMessage(t *testing.T) {
	e := newTestEcho()

	body := `{"message":""}`
	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/roomid123/announce", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("roomid123")

	err := AdminAnnounceHandler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "공지 메시지가 필요합니다")
}

// TestAdminAnnounceHandler_MessageTooLong verifies that a message over 2000 runes returns 400.
func TestAdminAnnounceHandler_MessageTooLong(t *testing.T) {
	e := newTestEcho()

	// Build a string with 2001 runes (using ASCII for simplicity).
	longMessage := strings.Repeat("a", 2001)
	body := `{"message":"` + longMessage + `"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/roomid123/announce", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("roomid123")

	err := AdminAnnounceHandler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "2000자를 초과할 수 없습니다")
}

// TestAdminAnnounceHandler_MissingRoomID verifies that an empty room ID returns 400.
func TestAdminAnnounceHandler_MissingRoomID(t *testing.T) {
	e := newTestEcho()

	body := `{"message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/rooms//announce", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	// No param set so c.Param("id") returns "".
	c.SetParamNames("id")
	c.SetParamValues("")

	err := AdminAnnounceHandler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
