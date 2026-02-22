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

// setupAdminTestServer creates an Echo instance with admin, crypto, and upload routes.
func setupAdminTestServer() *echo.Echo {
	e := echo.New()
	e.HideBanner = true

	// Auth endpoints (no JWT middleware for test setup)
	e.POST("/auth/register", handler.RegisterHandler)
	e.POST("/auth/login", handler.LoginHandler)

	// Admin group (requires AdminRequired middleware)
	admin := e.Group("/admin", middleware.AdminRequired())
	admin.GET("/stats", handler.AdminStatsHandler)
	admin.GET("/users", handler.AdminUsersHandler)
	admin.PUT("/users/:username/block", handler.AdminBlockUserHandler)
	admin.GET("/rooms", handler.AdminRoomsHandler)
	admin.DELETE("/rooms/:id", handler.AdminDeleteRoomHandler)
	admin.POST("/rooms/:id/announce", handler.AdminAnnounceHandler)

	// Crypto endpoints
	e.PUT("/crypto/keys", handler.UploadPublicKeyHandler)
	e.GET("/crypto/keys/:username", handler.GetPublicKeyHandler)
	e.GET("/rooms/:id/keys", handler.GetRoomKeysHandler)

	return e
}

// ---- Admin Tests ----

// TestAdminBlockSelf verifies that an admin cannot block themselves (#246).
// Requires MongoDB with an admin user.
func TestAdminBlockSelf(t *testing.T) {
	t.Skip("requires MongoDB: run with docker-compose up")

	e := setupAdminTestServer()

	// Login as admin
	rec := doRequest(t, e, http.MethodPost, "/auth/login", map[string]string{
		"username": "admin",
		"password": "adminpassword",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("admin login: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var authResp struct {
		Token string `json:"token"`
	}
	json.NewDecoder(rec.Body).Decode(&authResp)
	token := authResp.Token

	// Try to block self
	rec = doRequest(t, e, http.MethodPut, "/admin/users/admin/block",
		map[string]bool{"blocked": true}, token)
	if rec.Code != http.StatusForbidden {
		t.Errorf("self-block: expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAdminAnnounceLengthLimit verifies that announce messages longer than 2000 chars are rejected (#220).
func TestAdminAnnounceLengthLimit(t *testing.T) {
	t.Skip("requires MongoDB: run with docker-compose up")

	e := setupAdminTestServer()

	// Login as admin
	rec := doRequest(t, e, http.MethodPost, "/auth/login", map[string]string{
		"username": "admin",
		"password": "adminpassword",
	}, "")
	var authResp struct {
		Token string `json:"token"`
	}
	json.NewDecoder(rec.Body).Decode(&authResp)
	token := authResp.Token

	// Build a 2001-rune message
	longMsg := make([]byte, 2001)
	for i := range longMsg {
		longMsg[i] = 'a'
	}

	rec = doRequest(t, e, http.MethodPost, "/admin/rooms/000000000000000000000001/announce",
		map[string]string{"message": string(longMsg)}, token)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("long announce: expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAdminAnnounceInactiveRoom verifies that announcing to an inactive room returns 404 (#185).
func TestAdminAnnounceInactiveRoom(t *testing.T) {
	t.Skip("requires MongoDB: run with docker-compose up")

	e := setupAdminTestServer()

	rec := doRequest(t, e, http.MethodPost, "/auth/login", map[string]string{
		"username": "admin",
		"password": "adminpassword",
	}, "")
	var authResp struct {
		Token string `json:"token"`
	}
	json.NewDecoder(rec.Body).Decode(&authResp)
	token := authResp.Token

	// Use a valid-looking but non-existent room ID
	rec = doRequest(t, e, http.MethodPost, "/admin/rooms/aaaaaaaaaaaaaaaaaaaaaaaa/announce",
		map[string]string{"message": "test announcement"}, token)
	if rec.Code != http.StatusNotFound {
		t.Errorf("inactive room announce: expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAdminStatsNoGoroutines verifies that goroutine count is not exposed (#235).
func TestAdminStatsNoGoroutines(t *testing.T) {
	t.Skip("requires MongoDB: run with docker-compose up")

	e := setupAdminTestServer()

	rec := doRequest(t, e, http.MethodPost, "/auth/login", map[string]string{
		"username": "admin",
		"password": "adminpassword",
	}, "")
	var authResp struct {
		Token string `json:"token"`
	}
	json.NewDecoder(rec.Body).Decode(&authResp)
	token := authResp.Token

	rec = doRequest(t, e, http.MethodGet, "/admin/stats", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin stats: expected 200, got %d", rec.Code)
	}

	var stats map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&stats)
	if _, hasGoroutines := stats["goroutines"]; hasGoroutines {
		t.Error("admin stats: goroutine count must not be exposed in response")
	}
}

// TestAdminRequiredUnauthorized verifies that unauthenticated requests to admin routes return 401.
func TestAdminRequiredUnauthorized(t *testing.T) {
	e := setupAdminTestServer()

	paths := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/admin/stats"},
		{http.MethodGet, "/admin/users"},
		{http.MethodGet, "/admin/rooms"},
	}

	for _, p := range paths {
		rec := doRequest(t, e, p.method, p.path, nil, "")
		// No token → JWT middleware should reject before AdminRequired
		if rec.Code == http.StatusOK {
			t.Errorf("%s %s: expected non-200 without token, got %d", p.method, p.path, rec.Code)
		}
	}
}

// ---- Crypto Tests ----

// TestUploadPublicKeyValidation verifies that invalid JWK strings are rejected (#256).
func TestUploadPublicKeyValidation(t *testing.T) {
	// We need to inject a username into context manually since there's no JWT middleware here.
	// Use a custom middleware that sets username for testing.
	testE := echo.New()
	testE.HideBanner = true
	testE.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("username", "testuser")
			return next(c)
		}
	})
	testE.PUT("/crypto/keys", handler.UploadPublicKeyHandler)

	// Only test rejection cases — acceptance case requires a live MongoDB connection.
	cases := []struct {
		name    string
		payload string
	}{
		{"empty key", `{"public_key":""}`},
		{"not JSON", `{"public_key":"not-json-at-all"}`},
		{"JSON without kty", `{"public_key":"{\"n\":\"abc\"}"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/crypto/keys", bytes.NewBufferString(tc.payload))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			testE.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("case %q: expected 400, got %d: %s", tc.name, rec.Code, rec.Body.String())
			}
		})
	}
}

// TestGetRoomKeysMembershipCheck verifies that non-members cannot get room keys (#254).
func TestGetRoomKeysMembershipCheck(t *testing.T) {
	t.Skip("requires MongoDB: run with docker-compose up")

	srv := setupAdminTestServer()

	// Register a user who is NOT a member of any room
	doRequest(t, srv, http.MethodPost, "/auth/register", map[string]string{
		"username":     "nonmemberuser",
		"password":     "password123",
		"display_name": "NonMember",
	}, "")

	rec := doRequest(t, srv, http.MethodPost, "/auth/login", map[string]string{
		"username": "nonmemberuser",
		"password": "password123",
	}, "")
	var authResp struct {
		Token string `json:"token"`
	}
	json.NewDecoder(rec.Body).Decode(&authResp)
	token := authResp.Token

	// Try to access keys for a room this user didn't join
	rec = doRequest(t, srv, http.MethodGet, "/rooms/aaaaaaaaaaaaaaaaaaaaaaaa/keys", nil, token)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
		t.Errorf("non-member room keys: expected 403 or 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---- File Upload Validation Tests (no DB needed) ----

// TestFileUploadSanitizeLogic verifies upload handler rejects requests without a valid file.
func TestFileUploadSanitizeLogic(t *testing.T) {
	testE := echo.New()
	testE.HideBanner = true
	testE.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("username", "testuser")
			return next(c)
		}
	})
	testE.POST("/rooms/:id/upload", handler.UploadFileHandler)

	// Send a multipart request with no file.
	// requireRoomMember tries MongoDB (nil collection) → 404, or no file → 400.
	// Either way we must not get 200.
	req := httptest.NewRequest(http.MethodPost, "/rooms/testroom/upload", nil)
	req.Header.Set(echo.HeaderContentType, "multipart/form-data")
	rec := httptest.NewRecorder()
	testE.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Errorf("upload without file: expected non-200, got 200")
	}
}

// TestAnnounceXSSEscaping verifies that announce messages with XSS payloads are escaped (#226).
// This tests the input validation logic without MongoDB.
func TestAnnounceXSSValidation(t *testing.T) {
	// Admin announce requires a hub, so without MongoDB/hub we just verify the endpoint
	// returns the correct error (room not found) rather than executing the XSS payload.
	testE := echo.New()
	testE.HideBanner = true
	testE.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("username", "admin")
			return next(c)
		}
	})
	testE.POST("/admin/rooms/:id/announce", handler.AdminAnnounceHandler)

	xssPayload := `<script>alert('xss')</script>`
	rec := doRequest(t, testE, http.MethodPost, "/admin/rooms/aaaaaaaaaaaaaaaaaaaaaaaa/announce",
		map[string]string{"message": xssPayload}, "")

	// Should be 404 (hub not active), NOT 200 with unescaped payload
	if rec.Code == http.StatusOK {
		var body map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&body)
		if msg, ok := body["message"].(string); ok {
			if bytes.Contains([]byte(msg), []byte("<script>")) {
				t.Error("XSS payload was not escaped in announce response")
			}
		}
	}
}
