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

// TestCreateRoomHandler_InvalidJSON verifies that a malformed JSON body returns 400.
func TestCreateRoomHandler_InvalidJSON(t *testing.T) {
	e := newTestEcho()

	req := httptest.NewRequest(http.MethodPost, "/rooms", bytes.NewBufferString("{bad json"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := CreateRoomHandler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestCreateRoomHandler_EmptyRoomName verifies that an empty room name returns 400.
func TestCreateRoomHandler_EmptyRoomName(t *testing.T) {
	e := newTestEcho()

	body := `{"name":"","description":"test","is_public":true}`
	req := httptest.NewRequest(http.MethodPost, "/rooms", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := CreateRoomHandler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "채팅방 이름")
}

// TestCreateRoomHandler_RoomNameTooLong verifies that a room name longer than 50 chars returns 400.
func TestCreateRoomHandler_RoomNameTooLong(t *testing.T) {
	e := newTestEcho()

	longName := strings.Repeat("a", 51)
	body := `{"name":"` + longName + `","description":"test","is_public":true}`
	req := httptest.NewRequest(http.MethodPost, "/rooms", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := CreateRoomHandler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "채팅방 이름")
}

// TestCreateRoomHandler_DescriptionTooLong verifies that a description longer than 200 chars returns 400.
func TestCreateRoomHandler_DescriptionTooLong(t *testing.T) {
	e := newTestEcho()

	longDesc := strings.Repeat("d", 201)
	body := `{"name":"valid","description":"` + longDesc + `","is_public":true}`
	req := httptest.NewRequest(http.MethodPost, "/rooms", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := CreateRoomHandler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "채팅방 설명")
}
