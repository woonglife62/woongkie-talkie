package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/woonglife62/woongkie-talkie/pkg/config"
	"github.com/woonglife62/woongkie-talkie/pkg/logger"
	"github.com/woonglife62/woongkie-talkie/pkg/metrics"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
	redisclient "github.com/woonglife62/woongkie-talkie/pkg/redis"
)

// Hub manages WebSocket connections for a single room
type Hub struct {
	RoomID     string
	Clients    map[*Client]bool
	Broadcast  chan mongodb.ChatMessage
	Register   chan *Client
	Unregister chan *Client
	stop       chan struct{}
	mu         sync.RWMutex
	broker     *redisclient.Broker
}

func NewHub(roomID string, broker *redisclient.Broker) *Hub {
	return &Hub{
		RoomID:     roomID,
		Clients:    make(map[*Client]bool),
		Broadcast:  make(chan mongodb.ChatMessage, 256),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		stop:       make(chan struct{}),
		broker:     broker,
	}
}

// closeAllClients sends a graceful close frame to every client, then closes
// the connection. Must be called with h.mu held.
func (h *Hub) closeAllClients() {
	closeMsg := websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutting down")
	for client := range h.Clients {
		client.closeSend()
		if client.Conn != nil {
			_ = client.Conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(writeWait))
			client.Conn.Close()
		}
		delete(h.Clients, client)
	}
}

func (h *Hub) Run() {
	metrics.RoomsActive.Inc()
	defer metrics.RoomsActive.Dec()

	// Subscribe to Redis channel if broker is available; use a never-closing channel as fallback.
	redisCh := make(chan []byte) // blocks forever if Redis not used
	if h.broker != nil && !h.broker.IsFallback() {
		msgCh := make(chan []byte, 256)
		if err := h.broker.Subscribe(h.RoomID, func(data []byte) {
			select {
			case msgCh <- data:
			default:
				// slow consumer: drop message and record metrics
				logger.Logger.Warnw("Redis message dropped: slow consumer",
					"room_id", h.RoomID,
				)
				metrics.RedisMessagesDropped.Inc()
			}
		}); err != nil {
			logger.Logger.Warnw("Redis subscribe failed, using local fallback",
				"room_id", h.RoomID, "error", err)
		} else {
			redisCh = msgCh
			defer h.broker.Unsubscribe(h.RoomID)
		}
	}

	idleTimer := time.NewTimer(config.HubIdleTimeout)
	defer idleTimer.Stop()

	for {
		select {
		case <-h.stop:
			// Clean up all remaining clients before exiting.
			h.mu.Lock()
			h.closeAllClients()
			h.mu.Unlock()
			return

		case <-idleTimer.C:
			// Auto-shutdown hub after idle timeout with no clients.
			// Fix #97 (TOCTOU): first check without RoomMgr lock, then re-confirm
			// under RoomMgr.mu to make the emptiness check and hub removal atomic
			// with respect to GetOrCreateHub (which also locks RoomMgr.mu).
			h.mu.RLock()
			empty := len(h.Clients) == 0
			h.mu.RUnlock()
			if empty {
				RoomMgr.mu.Lock()
				h.mu.RLock()
				stillEmpty := len(h.Clients) == 0
				h.mu.RUnlock()
				if stillEmpty {
					delete(RoomMgr.hubs, h.RoomID)
					RoomMgr.mu.Unlock()
					logger.Logger.Infow("hub idle timeout, shutting down",
						"room_id", h.RoomID)
					// Signal stop to release any goroutines waiting on it (#169)
					close(h.stop)
					return
				}
				RoomMgr.mu.Unlock()
			}
			// Still has clients; reset the timer.
			idleTimer.Reset(config.HubIdleTimeout)

		case client := <-h.Register:
			// Reset idle timer on activity.
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(config.HubIdleTimeout)

			h.mu.Lock()
			h.Clients[client] = true
			h.mu.Unlock()
			metrics.ActiveWSConnections.Inc()
			logger.Logger.Infow("client registered",
				"room_id", client.RoomID,
				"username", client.Username,
			)
			// Update Redis presence and broadcast PRESENCE online event
			if redisclient.IsAvailable() {
				_ = redisclient.SetOnline(context.Background(), client.RoomID, client.Username)
			}
			h.broadcastLocal(mongodb.ChatMessage{
				Event:   "PRESENCE",
				User:    client.Username,
				RoomID:  client.RoomID,
				Message: "online",
			})

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				client.closeSend()
				logger.Logger.Infow("client unregistered",
					"room_id", client.RoomID,
					"username", client.Username,
				)
			}
			// Check if user still has other connections open (multi-tab, #171)
			remainingConns := 0
			for c := range h.Clients {
				if c.Username == client.Username {
					remainingConns++
				}
			}
			h.mu.Unlock()
			// Only mark offline if this was the last connection for this user
			if remainingConns == 0 {
				if redisclient.IsAvailable() {
					_ = redisclient.SetOffline(context.Background(), client.RoomID, client.Username)
					_ = redisclient.ClearTyping(context.Background(), client.RoomID, client.Username)
				}
				h.broadcastLocal(mongodb.ChatMessage{
					Event:   "PRESENCE",
					User:    client.Username,
					RoomID:  client.RoomID,
					Message: "offline",
				})
			}

		case msg := <-h.Broadcast:
			// Message from a local client: publish to Redis or fall back to local broadcast.
			if h.broker != nil && !h.broker.IsFallback() {
				data, err := json.Marshal(msg)
				if err == nil {
					if pubErr := h.broker.Publish(context.Background(), h.RoomID, data); pubErr != nil {
						logger.Logger.Warnw("Redis publish failed, using local broadcast",
							"room_id", h.RoomID, "error", pubErr)
						h.broadcastLocal(msg)
					}
					// When Redis is healthy we receive our own message back via redisCh.
				} else {
					h.broadcastLocal(msg)
				}
			} else {
				// Fallback mode: direct local broadcast (original behaviour).
				h.broadcastLocal(msg)
			}

		case data := <-redisCh:
			// Message received from Redis (from this or another server instance).
			var msg mongodb.ChatMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				logger.Logger.Warnw("Redis message unmarshal failed",
					"room_id", h.RoomID, "error", err)
				continue
			}
			// Event whitelist: only forward known, safe event types.
			if !isAllowedEvent(msg.Event) {
				logger.Logger.Warnw("Redis message rejected: unknown event type",
					"room_id", h.RoomID, "event", msg.Event)
				continue
			}
			h.broadcastLocal(msg)
		}
	}
}

// allowedEvents is the whitelist of event types accepted from Redis Pub/Sub.
var allowedEvents = map[string]bool{
	"MSG":          true,
	"MSG_FILE":     true,
	"MSG_EDIT":     true,
	"MSG_DELETE":   true,
	"OPEN":         true,
	"CLOSE":        true,
	"CHATLOG":      true,
	"PRESENCE":     true,
	"TYPING_START": true,
	"TYPING_STOP":  true,
	"WARN":         true,
}

// isAllowedEvent returns true if the event type is in the whitelist.
func isAllowedEvent(event string) bool {
	return allowedEvents[event]
}

// broadcastLocal fans out msg to all locally connected clients.
// It takes a snapshot of the client list under a read lock to avoid holding
// the mutex while performing channel sends, preventing potential deadlocks.
func (h *Hub) broadcastLocal(msg mongodb.ChatMessage) {
	isTyping := msg.Event == "TYPING_START" || msg.Event == "TYPING_STOP"

	// Update Redis typing state (best-effort, errors are non-fatal)
	if redisclient.IsAvailable() {
		ctx := context.Background()
		if msg.Event == "TYPING_START" {
			_ = redisclient.SetTyping(ctx, h.RoomID, msg.User)
		} else if msg.Event == "TYPING_STOP" {
			_ = redisclient.ClearTyping(ctx, h.RoomID, msg.User)
		}
	}

	msgFulltxt := msg.Message

	// Take a snapshot of clients under a read lock to avoid holding the mutex
	// during channel sends, which could otherwise cause a deadlock if a client's
	// writePump goroutine is blocked waiting on the hub's run loop.
	h.mu.RLock()
	snapshot := make([]*Client, 0, len(h.Clients))
	for client := range h.Clients {
		snapshot = append(snapshot, client)
	}
	h.mu.RUnlock()

	// Collect slow clients that need to be evicted (Send channel full).
	var evict []*Client

	for _, client := range snapshot {
		// Do not send typing events back to the sender
		if isTyping && client.Username == msg.User {
			continue
		}

		clientMsg := msg
		clientMsg.Message = msgFulltxt
		if client.Username == msg.User {
			clientMsg.Owner = true
		} else {
			clientMsg.Owner = false
		}
		if clientMsg.Event == "OPEN" {
			clientMsg.Message = fmt.Sprintf("---- %s님이 입장하셨습니다. ----", clientMsg.User)
		} else if clientMsg.Event == "CLOSE" {
			clientMsg.Message = fmt.Sprintf("---- %s님이 퇴장하셨습니다. ----", clientMsg.User)
		}
		select {
		case client.Send <- clientMsg:
		default:
			// Slow client: mark for eviction.
			evict = append(evict, client)
		}
	}

	// Evict slow clients under a write lock.
	if len(evict) > 0 {
		h.mu.Lock()
		for _, client := range evict {
			if _, ok := h.Clients[client]; ok {
				client.closeSend()
				delete(h.Clients, client)
				// Clean up Redis presence for evicted client (#146)
				if redisclient.IsAvailable() {
					ctx := context.Background()
					_ = redisclient.SetOffline(ctx, client.RoomID, client.Username)
					_ = redisclient.ClearTyping(ctx, client.RoomID, client.Username)
				}
			}
		}
		h.mu.Unlock()
	}
}

// KickUser forcibly disconnects all connections for the given username (#242).
func (h *Hub) KickUser(username string) {
	h.mu.Lock()
	var toEvict []*Client
	for client := range h.Clients {
		if client.Username == username {
			toEvict = append(toEvict, client)
		}
	}
	for _, client := range toEvict {
		client.closeSend()
		delete(h.Clients, client)
	}
	h.mu.Unlock()

	// Clean up Redis presence after kick
	if redisclient.IsAvailable() && len(toEvict) > 0 {
		ctx := context.Background()
		_ = redisclient.SetOffline(ctx, h.RoomID, username)
		_ = redisclient.ClearTyping(ctx, h.RoomID, username)
	}
}

// GetMemberNames returns unique usernames of connected clients
func (h *Hub) GetMemberNames() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	names := make(map[string]bool)
	for client := range h.Clients {
		names[client.Username] = true
	}
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	return result
}
