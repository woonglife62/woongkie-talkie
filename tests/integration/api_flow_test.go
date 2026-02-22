//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/woonglife62/woongkie-talkie/server/handler"
	"github.com/woonglife62/woongkie-talkie/server/middleware"
)

// setupTestServer creates an Echo instance with routing for integration tests.
func setupTestServer() *echo.Echo {
	e := echo.New()
	e.HideBanner = true

	// Auth endpoints
	e.POST("/auth/register", handler.RegisterHandler)
	e.POST("/auth/login", handler.LoginHandler)
	e.POST("/auth/logout", handler.LogoutHandler)
	e.GET("/auth/me", handler.MeHandler)

	// Room endpoints
	e.POST("/rooms", handler.CreateRoomHandler)
	e.GET("/rooms", handler.ListRoomsHandler)
	e.GET("/rooms/:id", handler.GetRoomHandler)
	e.POST("/rooms/:id/join", handler.JoinRoomHandler)
	e.GET("/rooms/:id/messages", handler.GetRoomMessagesHandler)
	e.GET("/rooms/:id/messages/search", handler.SearchMessagesHandler)

	return e
}

// doRequest is a helper that sends a JSON request and returns the response recorder.
func doRequest(t *testing.T, e *echo.Echo, method, path string, body interface{}, token string) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// TestFullUserFlow tests the complete user journey: register -> login -> create room -> list rooms -> join -> get messages.
// Requires a running MongoDB instance.
func TestFullUserFlow(t *testing.T) {
	t.Skip("requires MongoDB: run with docker-compose up")

	e := setupTestServer()

	// Step 1: Register a new user
	rec := doRequest(t, e, http.MethodPost, "/auth/register", map[string]string{
		"username":     "integuser1",
		"password":     "password123",
		"display_name": "IntegUser1",
	}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var authResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&authResp); err != nil {
		t.Fatalf("register: failed to decode response: %v", err)
	}
	token := authResp.Token
	if token == "" {
		t.Fatal("register: expected non-empty token")
	}

	// Step 2: Login with the registered user
	rec = doRequest(t, e, http.MethodPost, "/auth/login", map[string]string{
		"username": "integuser1",
		"password": "password123",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if err := json.NewDecoder(rec.Body).Decode(&authResp); err != nil {
		t.Fatalf("login: failed to decode response: %v", err)
	}
	token = authResp.Token

	// Step 3: Create a room
	rec = doRequest(t, e, http.MethodPost, "/rooms", map[string]interface{}{
		"name":        "Integration Test Room",
		"description": "Room created by integration test",
		"is_public":   true,
		"max_members": 10,
	}, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create room: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var roomResp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&roomResp); err != nil {
		t.Fatalf("create room: failed to decode response: %v", err)
	}
	roomID := roomResp.ID
	if roomID == "" {
		t.Fatal("create room: expected non-empty room ID")
	}

	// Step 4: List rooms — new room should appear
	rec = doRequest(t, e, http.MethodGet, "/rooms", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("list rooms: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var rooms []map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&rooms); err != nil {
		t.Fatalf("list rooms: failed to decode response: %v", err)
	}
	if len(rooms) == 0 {
		t.Fatal("list rooms: expected at least one room")
	}

	// Step 5: Join the room
	rec = doRequest(t, e, http.MethodPost, "/rooms/"+roomID+"/join", nil, token)
	if rec.Code != http.StatusOK && rec.Code != http.StatusBadRequest {
		// StatusBadRequest is acceptable if already a member
		t.Fatalf("join room: expected 200 or 400, got %d: %s", rec.Code, rec.Body.String())
	}

	// Step 6: Get messages from the room (public room, so accessible)
	rec = doRequest(t, e, http.MethodGet, "/rooms/"+roomID+"/messages", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("get messages: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestMessageSearchFlow tests login -> search messages.
// Requires a running MongoDB instance with data.
func TestMessageSearchFlow(t *testing.T) {
	t.Skip("requires MongoDB: run with docker-compose up")

	e := setupTestServer()

	// Login
	rec := doRequest(t, e, http.MethodPost, "/auth/login", map[string]string{
		"username": "integuser1",
		"password": "password123",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var authResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&authResp); err != nil {
		t.Fatalf("login: failed to decode response: %v", err)
	}
	token := authResp.Token

	// List rooms to get a room ID
	rec = doRequest(t, e, http.MethodGet, "/rooms", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("list rooms: expected 200, got %d", rec.Code)
	}
	var rooms []map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&rooms); err != nil || len(rooms) == 0 {
		t.Skip("no rooms available for search test")
	}

	roomID, _ := rooms[0]["id"].(string)
	if roomID == "" {
		t.Skip("could not determine room ID")
	}

	// Search for messages
	req := httptest.NewRequest(http.MethodGet, "/rooms/"+roomID+"/messages/search?q=hello&limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req)

	// Search returns 200 even if empty results
	if rec2.Code != http.StatusOK {
		t.Fatalf("search: expected 200, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

// TestAuthValidation tests input validation that does not require MongoDB.
func TestAuthValidation(t *testing.T) {
	e := setupTestServer()

	// Test 1: Register with too-short username
	rec := doRequest(t, e, http.MethodPost, "/auth/register", map[string]string{
		"username": "ab",
		"password": "password123",
	}, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("short username register: expected 400, got %d", rec.Code)
	}

	// Test 2: Register with too-short password
	rec = doRequest(t, e, http.MethodPost, "/auth/register", map[string]string{
		"username": "validuser",
		"password": "abc",
	}, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("short password register: expected 400, got %d", rec.Code)
	}
}

// TestAuthFlow tests login, token, and me endpoint scenarios.
// Requires a running MongoDB instance.
func TestAuthFlow(t *testing.T) {
	t.Skip("requires MongoDB: run with docker-compose up")

	e := setupTestServer()

	// Test 1: Invalid login credentials
	rec := doRequest(t, e, http.MethodPost, "/auth/login", map[string]string{
		"username": "nonexistentuser",
		"password": "wrongpassword",
	}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("invalid login: expected 401, got %d", rec.Code)
	}

	// Test 2: Missing token — protected endpoint without auth should not return 200
	rec = doRequest(t, e, http.MethodGet, "/auth/me", nil, "")
	if rec.Code == http.StatusOK {
		t.Error("me endpoint: should not return 200 without a valid token")
	}

	// Test 3: Invalid token format
	rec = doRequest(t, e, http.MethodGet, "/auth/me", nil, "invalidtoken")
	if rec.Code == http.StatusOK {
		t.Error("me endpoint: should not return 200 with invalid token")
	}
}

// TestRateLimiterBurst sends requests above the rate limit and verifies 429 responses.
// Uses a dummy handler to avoid MongoDB dependency.
func TestRateLimiterBurst(t *testing.T) {
	e := echo.New()
	e.HideBanner = true

	// Use AuthRateLimit with a dummy handler that always returns 200
	dummyHandler := func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
	e.POST("/auth/login", dummyHandler, middleware.AuthRateLimit())

	body := map[string]string{"username": "testuser", "password": "testpass"}

	got429 := false
	for i := 0; i < 20; i++ {
		rec := doRequest(t, e, http.MethodPost, "/auth/login", body, "")
		if rec.Code == http.StatusTooManyRequests {
			got429 = true
			// Verify Retry-After header is set
			if rec.Header().Get("Retry-After") == "" {
				t.Error("rate limit response missing Retry-After header")
			}
			break
		}
	}
	if !got429 {
		t.Error("expected at least one 429 Too Many Requests after burst, got none")
	}
}
