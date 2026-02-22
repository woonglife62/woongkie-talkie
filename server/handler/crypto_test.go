package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidateJWK_ValidRSA verifies that a well-formed RSA JWK returns true.
func TestValidateJWK_ValidRSA(t *testing.T) {
	// Minimal RSA public JWK with required "kty" field.
	validJWK := `{"kty":"RSA","n":"sampleModulus","e":"AQAB","use":"enc"}`
	assert.True(t, validateJWK(validJWK), "valid RSA JWK should return true")
}

// TestValidateJWK_ValidEC verifies that a well-formed EC JWK returns true.
func TestValidateJWK_ValidEC(t *testing.T) {
	validJWK := `{"kty":"EC","crv":"P-256","x":"sampleX","y":"sampleY"}`
	assert.True(t, validateJWK(validJWK), "valid EC JWK should return true")
}

// TestValidateJWK_InvalidJSON verifies that invalid JSON returns false.
func TestValidateJWK_InvalidJSON(t *testing.T) {
	assert.False(t, validateJWK("{bad json}"), "invalid JSON should return false")
	assert.False(t, validateJWK("not json at all"), "non-JSON string should return false")
	assert.False(t, validateJWK("[1,2,3]"), "JSON array (not object) should return false")
}

// TestValidateJWK_MissingKty verifies that a JWK without the "kty" field returns false.
func TestValidateJWK_MissingKty(t *testing.T) {
	noKty := `{"n":"sampleModulus","e":"AQAB","use":"enc"}`
	assert.False(t, validateJWK(noKty), "JWK without kty should return false")
}

// TestValidateJWK_EmptyKty verifies that a JWK with an empty "kty" returns false.
func TestValidateJWK_EmptyKty(t *testing.T) {
	emptyKty := `{"kty":"","n":"sampleModulus"}`
	assert.False(t, validateJWK(emptyKty), "JWK with empty kty should return false")
}

// TestValidateJWK_EmptyInput verifies that an empty string returns false.
func TestValidateJWK_EmptyInput(t *testing.T) {
	assert.False(t, validateJWK(""), "empty string should return false")
}

// TestValidateJWK_NullKty verifies that a JWK with null kty returns false.
func TestValidateJWK_NullKty(t *testing.T) {
	nullKty := `{"kty":null,"n":"sampleModulus"}`
	assert.False(t, validateJWK(nullKty), "JWK with null kty should return false")
}
