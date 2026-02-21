package redisclient

import (
	"context"
	"fmt"
	"time"
)

const (
	presenceKeyPrefix = "presence:room:"
	typingKeyPrefix   = "typing:room:"
	presenceTTL       = 5 * time.Minute
	typingTTL         = 10 * time.Second
)

// presenceKey returns the Redis key for a user's presence in a room.
func presenceKey(roomID, username string) string {
	return fmt.Sprintf("%s%s:user:%s", presenceKeyPrefix, roomID, username)
}

// typingKey returns the Redis key for a user's typing status in a room.
func typingKey(roomID, username string) string {
	return fmt.Sprintf("%s%s:user:%s", typingKeyPrefix, roomID, username)
}

// SetOnline marks a user as online in a room with a TTL.
func SetOnline(ctx context.Context, roomID, username string) error {
	if client == nil {
		return fmt.Errorf("redis: client not initialized")
	}
	return client.Set(ctx, presenceKey(roomID, username), "online", presenceTTL).Err()
}

// SetOffline removes a user's online presence from a room.
func SetOffline(ctx context.Context, roomID, username string) error {
	if client == nil {
		return fmt.Errorf("redis: client not initialized")
	}
	return client.Del(ctx, presenceKey(roomID, username)).Err()
}

// SetTyping marks a user as typing in a room with a short TTL.
func SetTyping(ctx context.Context, roomID, username string) error {
	if client == nil {
		return fmt.Errorf("redis: client not initialized")
	}
	return client.Set(ctx, typingKey(roomID, username), "typing", typingTTL).Err()
}

// ClearTyping removes a user's typing status from a room.
func ClearTyping(ctx context.Context, roomID, username string) error {
	if client == nil {
		return fmt.Errorf("redis: client not initialized")
	}
	return client.Del(ctx, typingKey(roomID, username)).Err()
}

// GetOnlineUsers returns all online usernames in a room.
func GetOnlineUsers(ctx context.Context, roomID string) ([]string, error) {
	if client == nil {
		return nil, fmt.Errorf("redis: client not initialized")
	}
	pattern := fmt.Sprintf("%s%s:user:*", presenceKeyPrefix, roomID)
	keys, err := client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	prefix := fmt.Sprintf("%s%s:user:", presenceKeyPrefix, roomID)
	users := make([]string, 0, len(keys))
	for _, key := range keys {
		if len(key) > len(prefix) {
			users = append(users, key[len(prefix):])
		}
	}
	return users, nil
}

// GetTypingUsers returns all usernames currently typing in a room.
func GetTypingUsers(ctx context.Context, roomID string) ([]string, error) {
	if client == nil {
		return nil, fmt.Errorf("redis: client not initialized")
	}
	pattern := fmt.Sprintf("%s%s:user:*", typingKeyPrefix, roomID)
	keys, err := client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	prefix := fmt.Sprintf("%s%s:user:", typingKeyPrefix, roomID)
	users := make([]string, 0, len(keys))
	for _, key := range keys {
		if len(key) > len(prefix) {
			users = append(users, key[len(prefix):])
		}
	}
	return users, nil
}
