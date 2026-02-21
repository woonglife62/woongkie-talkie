package redisclient

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/woonglife62/woongkie-talkie/pkg/logger"
)

const channelPrefix = "chat:room:"

// Broker manages Redis pub/sub subscriptions with automatic fallback and recovery.
type Broker struct {
	client        *redis.Client
	mu            sync.RWMutex
	subscriptions map[string]*redis.PubSub
	handlers      map[string]func([]byte)
	fallback      bool
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewBroker creates a Broker backed by the given client.
func NewBroker(c *redis.Client) *Broker {
	ctx, cancel := context.WithCancel(context.Background())
	b := &Broker{
		client:        c,
		subscriptions: make(map[string]*redis.PubSub),
		handlers:      make(map[string]func([]byte)),
		ctx:           ctx,
		cancel:        cancel,
	}
	go b.monitorConnection()
	return b
}

// channelName returns the Redis channel name for a room.
func channelName(roomID string) string {
	return channelPrefix + roomID
}

// Publish sends data to the channel for roomID.
func (b *Broker) Publish(ctx context.Context, roomID string, data []byte) error {
	b.mu.RLock()
	fb := b.fallback
	b.mu.RUnlock()

	if fb || b.client == nil {
		return fmt.Errorf("redis: broker in fallback mode, publish skipped")
	}

	return b.client.Publish(ctx, channelName(roomID), data).Err()
}

// Subscribe registers handler for messages on roomID and starts a listener goroutine.
func (b *Broker) Subscribe(roomID string, handler func([]byte)) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.fallback || b.client == nil {
		b.handlers[roomID] = handler
		return fmt.Errorf("redis: broker in fallback mode, subscription stored but not active")
	}

	if _, exists := b.subscriptions[roomID]; exists {
		b.handlers[roomID] = handler
		return nil
	}

	ps := b.client.Subscribe(b.ctx, channelName(roomID))
	b.subscriptions[roomID] = ps
	b.handlers[roomID] = handler

	go b.listenRoom(roomID, ps)
	return nil
}

// listenRoom reads messages from a PubSub and dispatches them to the handler.
func (b *Broker) listenRoom(roomID string, ps *redis.PubSub) {
	ch := ps.Channel()
	for msg := range ch {
		b.mu.RLock()
		handler := b.handlers[roomID]
		b.mu.RUnlock()

		if handler != nil {
			handler([]byte(msg.Payload))
		}
	}
}

// Unsubscribe removes the subscription for roomID.
func (b *Broker) Unsubscribe(roomID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	ps, exists := b.subscriptions[roomID]
	if !exists {
		delete(b.handlers, roomID)
		return nil
	}

	if err := ps.Close(); err != nil {
		logger.Logger.Warnw("redis: failed to close pubsub", "roomID", roomID, "error", err)
	}

	delete(b.subscriptions, roomID)
	delete(b.handlers, roomID)
	return nil
}

// IsFallback reports whether the broker is operating in fallback mode.
func (b *Broker) IsFallback() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.fallback
}

// SetFallback manually sets the fallback mode.
func (b *Broker) SetFallback(v bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.fallback = v
}

// Close shuts down all subscriptions and stops the monitor goroutine.
func (b *Broker) Close() {
	b.cancel()

	b.mu.Lock()
	defer b.mu.Unlock()

	for roomID, ps := range b.subscriptions {
		if err := ps.Close(); err != nil {
			logger.Logger.Warnw("redis: error closing subscription", "roomID", roomID, "error", err)
		}
	}
	b.subscriptions = make(map[string]*redis.PubSub)
}

// monitorConnection periodically pings Redis, switches to fallback on failure,
// and re-subscribes all rooms when the connection recovers.
func (b *Broker) monitorConnection() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			b.mu.RLock()
			c := b.client
			b.mu.RUnlock()

			if c == nil {
				continue
			}

			pingCtx, cancel := context.WithTimeout(b.ctx, 3*time.Second)
			err := c.Ping(pingCtx).Err()
			cancel()

			b.mu.Lock()
			wasDown := b.fallback
			if err != nil {
				if !b.fallback {
					logger.Logger.Warnw("redis: connection lost, switching to fallback", "error", err)
					b.fallback = true
					// Close all existing subscriptions; they are now stale.
					for roomID, ps := range b.subscriptions {
						ps.Close()
						delete(b.subscriptions, roomID)
					}
				}
			} else if wasDown {
				// Connection recovered — restore subscriptions.
				logger.Logger.Infow("redis: connection recovered, restoring subscriptions")
				b.fallback = false
				handlers := make(map[string]func([]byte), len(b.handlers))
				for k, v := range b.handlers {
					handlers[k] = v
				}
				b.mu.Unlock()

				for roomID, handler := range handlers {
					ps := b.client.Subscribe(b.ctx, channelName(roomID))
					b.mu.Lock()
					b.subscriptions[roomID] = ps
					b.mu.Unlock()
					go b.listenRoom(roomID, ps)
					logger.Logger.Infow("redis: re-subscribed room", "roomID", roomID)
					_ = handler
				}
				continue
			}
			b.mu.Unlock()
		}
	}
}
