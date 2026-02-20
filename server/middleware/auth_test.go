package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/woonglife62/woongkie-talkie/pkg/config"
)

const testSecret = "test-secret-key-minimum-32-chars-long!!"

// TestMain sets required env vars before any init() in imported packages
// reads them. config.JWTConfig is a package-level var populated at init time,
// so we override it directly after init completes.
func TestMain(m *testing.M) {
	// Set env vars that config package init() needs so it does not panic.
	os.Setenv("IS_DEV", "dev")
	os.Setenv("JWT_SECRET", testSecret)
	os.Setenv("JWT_EXPIRY", "24h")
	os.Setenv("MONGODB_URI", "mongodb://localhost:27017")
	os.Setenv("MONGODB_USER", "test")
	os.Setenv("MONGODB_PASSWORD", "test")
	os.Setenv("MONGODB_DATABASE", "test")

	// config.JWTConfig is already populated by init(), but init() may have
	// run before our Setenv calls if this binary was already linked.
	// Override directly so the middleware reads the test secret at request time.
	config.JWTConfig.Secret = testSecret
	config.JWTConfig.Expiry = "24h"

	os.Exit(m.Run())
}

// setupEcho creates an Echo instance with jwtAuth middleware applied and a
// protected test handler that returns 200 with the username in the body.
func setupEcho() *echo.Echo {
	e := echo.New()
	jwtAuth(e)
	e.GET("/*", func(c echo.Context) error {
		username, _ := c.Get("username").(string)
		return c.String(http.StatusOK, username)
	})
	return e
}

// makeToken creates a signed HS256 JWT with the given subject and expiry.
func makeToken(subject string, expiry time.Duration, secret string) string {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   subject,
		ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		IssuedAt:  jwt.NewNumericDate(now),
		Issuer:    "woongkie-talkie",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(secret))
	return signed
}

// TestJWTAuth_SkipPaths verifies that requests to public paths bypass auth
// without any token and still receive a 200 response.
func TestJWTAuth_SkipPaths(t *testing.T) {
	config.JWTConfig.Secret = testSecret
	e := setupEcho()

	skipPaths := []string{
		"/auth/register",
		"/auth/login",
		"/auth/",
		"/view/index",
		"/view/",
		"/login",
		"/",
		"/health",
		"/ready",
	}

	for _, path := range skipPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code, "path %s should skip auth", path)
		})
	}
}

// TestJWTAuth_ValidBearerToken verifies that a valid JWT in the Authorization
// header grants access and sets the username in the context.
func TestJWTAuth_ValidBearerToken(t *testing.T) {
	config.JWTConfig.Secret = testSecret
	e := setupEcho()

	token := makeToken("alice", time.Hour, testSecret)

	req := httptest.NewRequest(http.MethodGet, "/api/rooms", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "alice", rec.Body.String())
}

// TestJWTAuth_ValidQueryParamToken verifies that a valid JWT in the ?token=
// query parameter grants access (WebSocket fallback path).
func TestJWTAuth_ValidQueryParamToken(t *testing.T) {
	config.JWTConfig.Secret = testSecret
	e := setupEcho()

	token := makeToken("bob", time.Hour, testSecret)

	req := httptest.NewRequest(http.MethodGet, "/ws?token="+token, nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "bob", rec.Body.String())
}

// TestJWTAuth_MissingToken verifies that a request with no token to a
// protected path returns 401.
func TestJWTAuth_MissingToken(t *testing.T) {
	config.JWTConfig.Secret = testSecret
	e := setupEcho()

	req := httptest.NewRequest(http.MethodGet, "/api/rooms", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestJWTAuth_InvalidToken verifies that a malformed token string returns 401.
func TestJWTAuth_InvalidToken(t *testing.T) {
	config.JWTConfig.Secret = testSecret
	e := setupEcho()

	req := httptest.NewRequest(http.MethodGet, "/api/rooms", nil)
	req.Header.Set("Authorization", "Bearer this.is.not.a.valid.jwt")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestJWTAuth_ExpiredToken verifies that a token whose ExpiresAt is in the
// past is rejected with 401.
func TestJWTAuth_ExpiredToken(t *testing.T) {
	config.JWTConfig.Secret = testSecret
	e := setupEcho()

	// Sign with a negative duration so the token is already expired.
	token := makeToken("carol", -time.Hour, testSecret)

	req := httptest.NewRequest(http.MethodGet, "/api/rooms", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestJWTAuth_WrongSigningMethod verifies that a token signed with a different
// algorithm (RS256 is not in the allowed list) returns 401.
// We simulate this by signing with a different HMAC secret, which causes
// signature verification to fail — equivalent to an untrusted token.
func TestJWTAuth_WrongSigningMethod(t *testing.T) {
	config.JWTConfig.Secret = testSecret
	e := setupEcho()

	// Token signed with a completely different secret simulates a foreign
	// token the server cannot verify.
	token := makeToken("eve", time.Hour, "wrong-secret-key-also-32-chars-long!!")

	req := httptest.NewRequest(http.MethodGet, "/api/rooms", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestJWTAuth_WrongAlgorithmToken verifies that a token signed with RS256
// (not in WithValidMethods(["HS256"])) is rejected with 401.
func TestJWTAuth_WrongAlgorithmToken(t *testing.T) {
	config.JWTConfig.Secret = testSecret
	e := setupEcho()

	// Manually craft a JWT header claiming RS256 but with HS256 body.
	// The easiest way is to use a raw token string with alg=RS256 in the header.
	// jwt/v5 with WithValidMethods(["HS256"]) will reject any other alg claim.
	header := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9" // {"alg":"RS256","typ":"JWT"}
	payload := "eyJzdWIiOiJldmUifQ"                   // {"sub":"eve"}
	fakeToken := header + "." + payload + ".fakesig"

	req := httptest.NewRequest(http.MethodGet, "/api/rooms", nil)
	req.Header.Set("Authorization", "Bearer "+fakeToken)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestJWTAuth_EmptySubject verifies that a valid HS256 token with an empty
// Subject claim is rejected with 401.
func TestJWTAuth_EmptySubject(t *testing.T) {
	config.JWTConfig.Secret = testSecret
	e := setupEcho()

	// Build a token with no Subject.
	claims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(testSecret))

	req := httptest.NewRequest(http.MethodGet, "/api/rooms", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestJWTAuth_BearerPrecedenceOverQueryParam verifies that when both the
// Authorization header and ?token= param are present, the header is used.
func TestJWTAuth_BearerPrecedenceOverQueryParam(t *testing.T) {
	config.JWTConfig.Secret = testSecret
	e := setupEcho()

	validToken := makeToken("header-user", time.Hour, testSecret)
	invalidToken := "bogus.query.token"

	req := httptest.NewRequest(http.MethodGet, "/api/rooms?token="+invalidToken, nil)
	req.Header.Set("Authorization", "Bearer "+validToken)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// Header token is valid, so request succeeds.
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "header-user", rec.Body.String())
}
