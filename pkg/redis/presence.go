package redisclient

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	presenceSetPrefix  = "presence:room:"          // SET key: presence:room:{roomID}
	presenceUserPrefix = "presence:user:"           // STRING key: presence:user:{roomID}:{username} (TTL)
	typingSetPrefix    = "typing:room:"             // SET key: typing:room:{roomID}
	typingUserPrefix   = "typing:user:"             // STRING key: typing:user:{roomID}:{username} (TTL)
	presenceTTL        = 5 * time.Minute
	typingTTL          = 5 * time.Second
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

// RefreshOnline resets the TTL for a user's presence key, keeping them marked online.
// Call this periodically (e.g. on WebSocket ping) to prevent stale expiry.
func RefreshOnline(ctx context.Context, roomID, username string) error {
	if client == nil {
		return fmt.Errorf("redis: client not initialized")
	}
	return client.Expire(ctx, presenceUserKey(roomID, username), presenceTTL).Err()
}

// GetOnlineUsers returns all online usernames in a room.
// Uses SMEMBERS then a single Pipeline of EXISTS checks to avoid N+1 round-trips.
func GetOnlineUsers(ctx context.Context, roomID string) ([]string, error) {
	if client == nil {
		return nil, fmt.Errorf("redis: client not initialized")
	}
	members, err := client.SMembers(ctx, presenceSetKey(roomID)).Result()
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return []string{}, nil
	}

	pipe := client.Pipeline()
	cmds := make([]*redis.IntCmd, len(members))
	for i, username := range members {
		cmds[i] = pipe.Exists(ctx, presenceUserKey(roomID, username))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}

	active := make([]string, 0, len(members))
	stale := make([]interface{}, 0)
	for i, username := range members {
		if cmds[i].Val() > 0 {
			active = append(active, username)
		} else {
			stale = append(stale, username)
		}
	}
	if len(stale) > 0 {
		client.SRem(ctx, presenceSetKey(roomID), stale...)
	}
	return active, nil
}

// GetTypingUsers returns all usernames currently typing in a room.
// Uses SMEMBERS then a single Pipeline of EXISTS checks to avoid N+1 round-trips (#162).
func GetTypingUsers(ctx context.Context, roomID string) ([]string, error) {
	if client == nil {
		return nil, fmt.Errorf("redis: client not initialized")
	}
	members, err := client.SMembers(ctx, typingSetKey(roomID)).Result()
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return []string{}, nil
	}

	pipe := client.Pipeline()
	cmds := make([]*redis.IntCmd, len(members))
	for i, username := range members {
		cmds[i] = pipe.Exists(ctx, typingUserKey(roomID, username))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}

	active := make([]string, 0, len(members))
	stale := make([]interface{}, 0)
	for i, username := range members {
		if cmds[i].Val() > 0 {
			active = append(active, username)
		} else {
			stale = append(stale, username)
		}
	}
	if len(stale) > 0 {
		client.SRem(ctx, typingSetKey(roomID), stale...)
	}
	return active, nil
}
