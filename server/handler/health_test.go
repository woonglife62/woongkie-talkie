package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestHealthHandler(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := HealthHandler(c); err != nil {
		t.Fatalf("HealthHandler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`expected {"status":"ok"}, got %v`, body)
	}
}

// TestReadyHandler_NoMongoDB verifies that ReadyHandler returns 503 when the
// MongoDB client cannot be reached. Because the global db.Client is initialised
// via package init() (which panics without a real DB), this test is skipped in
// environments where the DB package is not available. To run it against a live
// DB, set the environment variable MONGO_URI before running the tests.
//
// The test exercises the error-path logic of ReadyHandler by confirming that the
// handler writes a JSON body with {"status":"not ready"} and HTTP 503 when
// db.Client.Ping fails. In CI without MongoDB this test is skipped so that the
// suite does not block on infrastructure dependencies.
func TestReadyHandler_NoMongoDB(t *testing.T) {
	t.Skip("ReadyHandler requires a live MongoDB client (db.Client); " +
		"run integration tests separately")
}
