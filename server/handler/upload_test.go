package handler

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSanitizeFilename_PathTraversal verifies that path traversal sequences are replaced.
func TestSanitizeFilename_PathTraversal(t *testing.T) {
	cases := []struct {
		input    string
		notExpected string
	}{
		{"../../../etc/passwd", ".."},
		{"..\\windows\\system32\\cmd.exe", ".."},
		{"/absolute/path/file.txt", "/"},
	}

	for _, tc := range cases {
		result := sanitizeFilename(tc.input)
		assert.NotContains(t, result, tc.notExpected, "sanitizeFilename(%q) should not contain %q, got %q", tc.input, tc.notExpected, result)
	}
}

// TestSanitizeFilename_SpecialChars verifies that special characters are replaced with underscores.
func TestSanitizeFilename_SpecialChars(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"normal_file.txt", "normal_file.txt"},
		{"file with spaces.jpg", "file_with_spaces.jpg"},
		{"file<script>.png", "file_script_.png"},
		{"file;rm -rf.txt", "file_rm_-rf.txt"},
	}

	for _, tc := range cases {
		result := sanitizeFilename(tc.input)
		assert.Equal(t, tc.expected, result, "sanitizeFilename(%q)", tc.input)
	}
}

// TestIsPrivateIP_Loopback verifies that 127.0.0.1 is detected as private.
func TestIsPrivateIP_Loopback(t *testing.T) {
	ip := net.ParseIP("127.0.0.1")
	assert.True(t, isPrivateIP(ip), "127.0.0.1 should be private (loopback)")
}

// TestIsPrivateIP_IPv6Loopback verifies that ::1 is detected as private.
func TestIsPrivateIP_IPv6Loopback(t *testing.T) {
	ip := net.ParseIP("::1")
	assert.True(t, isPrivateIP(ip), "::1 should be private (IPv6 loopback)")
}

// TestIsPrivateIP_RFC1918_10 verifies that 10.x.x.x is detected as private.
func TestIsPrivateIP_RFC1918_10(t *testing.T) {
	cases := []string{"10.0.0.1", "10.255.255.255", "10.1.2.3"}
	for _, addr := range cases {
		ip := net.ParseIP(addr)
		assert.True(t, isPrivateIP(ip), "%s should be private", addr)
	}
}

// TestIsPrivateIP_RFC1918_192168 verifies that 192.168.x.x is detected as private.
func TestIsPrivateIP_RFC1918_192168(t *testing.T) {
	cases := []string{"192.168.0.1", "192.168.1.100", "192.168.255.255"}
	for _, addr := range cases {
		ip := net.ParseIP(addr)
		assert.True(t, isPrivateIP(ip), "%s should be private", addr)
	}
}

// TestIsPrivateIP_Public verifies that public IPs are not flagged as private.
func TestIsPrivateIP_Public(t *testing.T) {
	cases := []string{"8.8.8.8", "1.1.1.1", "203.0.113.1"}
	for _, addr := range cases {
		ip := net.ParseIP(addr)
		assert.False(t, isPrivateIP(ip), "%s should NOT be private", addr)
	}
}

// TestExtMatchesMime_Correct verifies valid extension/MIME pairings.
func TestExtMatchesMime_Correct(t *testing.T) {
	cases := []struct {
		ext  string
		mime string
	}{
		{".jpg", "image/jpeg"},
		{".jpeg", "image/jpeg"},
		{".png", "image/png"},
		{".gif", "image/gif"},
		{".webp", "image/webp"},
		{".pdf", "application/pdf"},
		{".txt", "text/plain"},
	}

	for _, tc := range cases {
		assert.True(t, extMatchesMime(tc.ext, tc.mime), "extMatchesMime(%q, %q) should be true", tc.ext, tc.mime)
	}
}

// TestExtMatchesMime_Incorrect verifies mismatched extension/MIME pairings return false.
func TestExtMatchesMime_Incorrect(t *testing.T) {
	cases := []struct {
		ext  string
		mime string
	}{
		{".exe", "image/jpeg"},
		{".jpg", "image/png"},
		{".png", "application/pdf"},
		{".php", "text/plain"},
		{".js", "image/gif"},
	}

	for _, tc := range cases {
		assert.False(t, extMatchesMime(tc.ext, tc.mime), "extMatchesMime(%q, %q) should be false", tc.ext, tc.mime)
	}
}
