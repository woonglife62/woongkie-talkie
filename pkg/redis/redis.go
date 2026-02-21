package redisclient

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var client *redis.Client

// Initialize creates and tests a Redis connection with a connection pool.
func Initialize(addr, password string, db int) error {
	c := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     10,
		MinIdleConns: 3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := c.Ping(ctx).Result(); err != nil {
		c.Close()
		return fmt.Errorf("redis: ping failed: %w", err)
	}

	client = c
	return nil
}

// Close shuts down the Redis client.
func Close() {
	if client != nil {
		client.Close()
		client = nil
	}
}

// IsAvailable returns true when the client is initialized.
func IsAvailable() bool {
	return client != nil
}

// Ping checks the connection liveness.
func Ping(ctx context.Context) error {
	if client == nil {
		return fmt.Errorf("redis: client not initialized")
	}
	_, err := client.Ping(ctx).Result()
	return err
}

// Client returns the underlying redis.Client for use in sub-packages.
func Client() *redis.Client {
	return client
}
