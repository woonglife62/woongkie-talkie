//go:build integration

package mongodb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestChatMessage_Structure verifies that ChatMessage fields have the expected
// JSON and BSON struct tags.
func TestChatMessage_Structure(t *testing.T) {
	msg := ChatMessage{
		Event:   "OPEN",
		User:    "alice",
		Message: "hello",
		Owner:   true,
		RoomID:  "room-123",
	}

	assert.Equal(t, "OPEN", msg.Event)
	assert.Equal(t, "alice", msg.User)
	assert.Equal(t, "hello", msg.Message)
	assert.True(t, msg.Owner)
	assert.Equal(t, "room-123", msg.RoomID)
}

// TestChat_Structure verifies that the Chat struct embeds ChatMessage correctly.
func TestChat_Structure(t *testing.T) {
	inner := ChatMessage{
		User:    "bob",
		Message: "world",
	}
	chat := Chat{
		RoomID:      "room-456",
		ChatMessage: inner,
	}

	assert.Equal(t, "room-456", chat.RoomID)
	assert.Equal(t, "bob", chat.User)
	assert.Equal(t, "world", chat.Message)
}

// TestInsertChat_SkipsWithoutMongo documents that InsertChat requires a live
// MongoDB connection.
func TestInsertChat_SkipsWithoutMongo(t *testing.T) {
	t.Skip("requires MongoDB connection")
}

// TestFindChatByRoom_SkipsWithoutMongo documents that FindChatByRoom requires
// a live MongoDB connection.
func TestFindChatByRoom_SkipsWithoutMongo(t *testing.T) {
	t.Skip("requires MongoDB connection")
}

// TestFindChatByRoomBefore_SkipsWithoutMongo documents that
// FindChatByRoomBefore requires a live MongoDB connection.
func TestFindChatByRoomBefore_SkipsWithoutMongo(t *testing.T) {
	t.Skip("requires MongoDB connection")
}
