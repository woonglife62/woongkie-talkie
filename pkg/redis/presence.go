package redisclient

import (
	"context"
	"fmt"
	"time"
)

const (
	presenceSetPrefix  = "presence:room:"          // SET key: presence:room:{roomID}
	presenceUserPrefix = "presence:user:"           // STRING key: presence:user:{roomID}:{username} (TTL)
	typingSetPrefix    = "typing:room:"             // SET key: typing:room:{roomID}
	typingUserPrefix   = "typing:user:"             // STRING key: typing:user:{roomID}:{username} (TTL)
	presenceTTL        = 5 * time.Minute
	typingTTL          = 10 * time.Second
)

// presenceSetKey returns the Redis Set key tracking all online users in a room.
func presenceSetKey(roomID string) string {
	return fmt.Sprintf("%s%s", presenceSetPrefix, roomID)
}

// presenceUserKey returns the per-user TTL key for presence.
func presenceUserKey(roomID, username string) string {
	return fmt.Sprintf("%s%s:%s", presenceUserPrefix, roomID, username)
}

// typingSetKey returns the Redis Set key tracking all typing users in a room.
func typingSetKey(roomID string) string {
	return fmt.Sprintf("%s%s", typingSetPrefix, roomID)
}

// typingUserKey returns the per-user TTL key for typing status.
func typingUserKey(roomID, username string) string {
	return fmt.Sprintf("%s%s:%s", typingUserPrefix, roomID, username)
}

// SetOnline marks a user as online in a room.
// Uses SADD to add to the room Set, and SET with TTL for expiry tracking.
func SetOnline(ctx context.Context, roomID, username string) error {
	if client == nil {
		return fmt.Errorf("redis: client not initialized")
	}
	pipe := client.Pipeline()
	pipe.SAdd(ctx, presenceSetKey(roomID), username)
	pipe.Set(ctx, presenceUserKey(roomID, username), "1", presenceTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// SetOffline removes a user's online presence from a room.
func SetOffline(ctx context.Context, roomID, username string) error {
	if client == nil {
		return fmt.Errorf("redis: client not initialized")
	}
	pipe := client.Pipeline()
	pipe.SRem(ctx, presenceSetKey(roomID), username)
	pipe.Del(ctx, presenceUserKey(roomID, username))
	_, err := pipe.Exec(ctx)
	return err
}

// SetTyping marks a user as typing in a room with a short TTL.
func SetTyping(ctx context.Context, roomID, username string) error {
	if client == nil {
		return fmt.Errorf("redis: client not initialized")
	}
	pipe := client.Pipeline()
	pipe.SAdd(ctx, typingSetKey(roomID), username)
	pipe.Set(ctx, typingUserKey(roomID, username), "1", typingTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// ClearTyping removes a user's typing status from a room.
func ClearTyping(ctx context.Context, roomID, username string) error {
	if client == nil {
		return fmt.Errorf("redis: client not initialized")
	}
	pipe := client.Pipeline()
	pipe.SRem(ctx, typingSetKey(roomID), username)
	pipe.Del(ctx, typingUserKey(roomID, username))
	_, err := pipe.Exec(ctx)
	return err
}

// GetOnlineUsers returns all online usernames in a room.
// Uses SMEMBERS and filters out users whose TTL key has expired.
func GetOnlineUsers(ctx context.Context, roomID string) ([]string, error) {
	if client == nil {
		return nil, fmt.Errorf("redis: client not initialized")
	}
	members, err := client.SMembers(ctx, presenceSetKey(roomID)).Result()
	if err != nil {
		return nil, err
	}

	active := make([]string, 0, len(members))
	for _, username := range members {
		exists, err := client.Exists(ctx, presenceUserKey(roomID, username)).Result()
		if err != nil {
			continue
		}
		if exists > 0 {
			active = append(active, username)
		} else {
			// TTL key expired; clean up stale Set member
			client.SRem(ctx, presenceSetKey(roomID), username)
		}
	}
	return active, nil
}

// GetTypingUsers returns all usernames currently typing in a room.
// Uses SMEMBERS and filters out users whose TTL key has expired.
func GetTypingUsers(ctx context.Context, roomID string) ([]string, error) {
	if client == nil {
		return nil, fmt.Errorf("redis: client not initialized")
	}
	members, err := client.SMembers(ctx, typingSetKey(roomID)).Result()
	if err != nil {
		return nil, err
	}

	active := make([]string, 0, len(members))
	for _, username := range members {
		exists, err := client.Exists(ctx, typingUserKey(roomID, username)).Result()
		if err != nil {
			continue
		}
		if exists > 0 {
			active = append(active, username)
		} else {
			// TTL key expired; clean up stale Set member
			client.SRem(ctx, typingSetKey(roomID), username)
		}
	}
	return active, nil
}
