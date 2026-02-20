package mongodb

// Tests for pure bcrypt helper functions HashRoomPassword and CheckRoomPassword.
// These functions do not touch the database, but they live in the mongodb package
// whose init() connects to MongoDB. Running this file therefore requires the same
// MongoDB connectivity that the other tests in this package need.
//
// To run only these tests:
//   go test ./pkg/mongoDB/ -run TestHashRoomPassword -run TestCheckRoomPassword
//
// The assertions use the standard testing package so there is no extra dependency.

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// ---------------------------------------------------------------------------
// HashRoomPassword
// ---------------------------------------------------------------------------

func TestHashRoomPassword_Empty(t *testing.T) {
	hash, err := HashRoomPassword("")
	if err != nil {
		t.Fatalf("unexpected error for empty password: %v", err)
	}
	if hash != "" {
		t.Errorf("expected empty hash for empty password, got %q", hash)
	}
}

func TestHashRoomPassword_Valid(t *testing.T) {
	hash, err := HashRoomPassword("secret123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash, got empty string")
	}
	// A bcrypt hash always starts with "$2a$" or "$2b$".
	if !strings.HasPrefix(hash, "$2") {
		t.Errorf("expected bcrypt hash prefix, got %q", hash)
	}
	// Sanity-check: the returned hash must actually verify against the original.
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("secret123")); err != nil {
		t.Errorf("hash does not verify against original password: %v", err)
	}
}

func TestHashRoomPassword_UniqueHashes(t *testing.T) {
	// bcrypt generates a random salt, so two hashes of the same password differ.
	h1, _ := HashRoomPassword("same")
	h2, _ := HashRoomPassword("same")
	if h1 == h2 {
		t.Error("expected two hashes of the same password to differ (random salt)")
	}
}

// ---------------------------------------------------------------------------
// CheckRoomPassword
// ---------------------------------------------------------------------------

func TestCheckRoomPassword_Correct(t *testing.T) {
	hash, err := HashRoomPassword("mypassword")
	if err != nil {
		t.Fatalf("HashRoomPassword error: %v", err)
	}
	room := &Room{Password: hash}
	if !CheckRoomPassword(room, "mypassword") {
		t.Error("CheckRoomPassword should return true for the correct password")
	}
}

func TestCheckRoomPassword_Wrong(t *testing.T) {
	hash, err := HashRoomPassword("correctpass")
	if err != nil {
		t.Fatalf("HashRoomPassword error: %v", err)
	}
	room := &Room{Password: hash}
	if CheckRoomPassword(room, "wrongpass") {
		t.Error("CheckRoomPassword should return false for a wrong password")
	}
}

func TestCheckRoomPassword_EmptyHash(t *testing.T) {
	// When Room.Password is empty the room has no password set; any caller is allowed.
	room := &Room{Password: ""}
	if !CheckRoomPassword(room, "anything") {
		t.Error("CheckRoomPassword should return true when the room has no password set")
	}
	if !CheckRoomPassword(room, "") {
		t.Error("CheckRoomPassword should return true when the room has no password set (empty input)")
	}
}
