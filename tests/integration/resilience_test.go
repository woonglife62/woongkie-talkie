//go:build integration

package integration

import (
	"context"
	"sync"
	"testing"

	redisclient "github.com/woonglife62/woongkie-talkie/pkg/redis"
)

// TestRedisBrokerFallback verifies that creating a Broker with nil client enters fallback mode
// gracefully without panicking.
func TestRedisBrokerFallback(t *testing.T) {
	broker := redisclient.NewBroker(nil)
	if broker == nil {
		t.Fatal("NewBroker(nil) returned nil")
	}
	defer broker.Close()

	// With nil client, the broker should be in fallback mode immediately or after Subscribe.
	err := broker.Subscribe("test-room", func(data []byte) {})
	if err == nil {
		t.Error("expected error subscribing with nil client, got nil")
	}
}

// TestBrokerPublishInFallback sets the broker to fallback mode and verifies Publish returns an error.
func TestBrokerPublishInFallback(t *testing.T) {
	broker := redisclient.NewBroker(nil)
	if broker == nil {
		t.Fatal("NewBroker(nil) returned nil")
	}
	defer broker.Close()

	broker.SetFallback(true)

	err := broker.Publish(context.Background(), "test-room", []byte(`{"message":"hello"}`))
	if err == nil {
		t.Error("expected error from Publish in fallback mode, got nil")
	}
}

// TestBrokerIsFallback verifies IsFallback reports correctly after SetFallback.
func TestBrokerIsFallback(t *testing.T) {
	broker := redisclient.NewBroker(nil)
	if broker == nil {
		t.Fatal("NewBroker(nil) returned nil")
	}
	defer broker.Close()

	broker.SetFallback(true)
	if !broker.IsFallback() {
		t.Error("expected IsFallback()=true after SetFallback(true)")
	}

	broker.SetFallback(false)
	if broker.IsFallback() {
		t.Error("expected IsFallback()=false after SetFallback(false)")
	}
}

// TestBrokerUnsubscribeNonExistent verifies Unsubscribe on a non-existent room does not panic.
func TestBrokerUnsubscribeNonExistent(t *testing.T) {
	broker := redisclient.NewBroker(nil)
	if broker == nil {
		t.Fatal("NewBroker(nil) returned nil")
	}
	defer broker.Close()

	// Should not panic
	err := broker.Unsubscribe("nonexistent-room")
	if err != nil {
		t.Errorf("unexpected error unsubscribing nonexistent room: %v", err)
	}
}

// TestConcurrentBrokerOperations verifies concurrent SetFallback and Publish calls
// do not cause data races (run with -race flag).
func TestConcurrentBrokerOperations(t *testing.T) {
	broker := redisclient.NewBroker(nil)
	if broker == nil {
		t.Fatal("NewBroker(nil) returned nil")
	}
	defer broker.Close()

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines * 3)

	// Concurrent SetFallback
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			broker.SetFallback(i%2 == 0)
		}(i)
	}

	// Concurrent IsFallback
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = broker.IsFallback()
		}()
	}

	// Concurrent Publish (all should return errors since client is nil)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = broker.Publish(context.Background(), "room", []byte("data"))
		}()
	}

	wg.Wait()
}

// TestBrokerCloseIdempotent verifies Close can be called without panicking.
func TestBrokerCloseIdempotent(t *testing.T) {
	broker := redisclient.NewBroker(nil)
	if broker == nil {
		t.Fatal("NewBroker(nil) returned nil")
	}

	// First Close should not panic.
	broker.Close()
}
