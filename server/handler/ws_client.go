package handler

import (
	"context"
	"html"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/woonglife62/woongkie-talkie/pkg/config"
	"github.com/woonglife62/woongkie-talkie/pkg/logger"
	"github.com/woonglife62/woongkie-talkie/pkg/metrics"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
	redisclient "github.com/woonglife62/woongkie-talkie/pkg/redis"
	"github.com/woonglife62/woongkie-talkie/server/middleware"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/time/rate"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10 // 54s
	maxMessageSize = 64 * 1024

	insertWorkers   = 4
	insertQueueSize = 1024
	insertTimeout   = 5 * time.Second

	insertBatchSize    = 50
	insertBatchTimeout = 100 * time.Millisecond
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:    1024,
	WriteBufferSize:   1024,
	HandshakeTimeout:  10 * time.Second,
	CheckOrigin:       config.CheckOrigin,
	EnableCompression: true,
}

// insertQueue is a buffered channel for async MongoDB chat inserts.
var insertQueue = make(chan mongodb.ChatMessage, insertQueueSize)

// insertWg tracks live insert worker goroutines for graceful shutdown.
var insertWg sync.WaitGroup

func init() {
	for i := 0; i < insertWorkers; i++ {
		insertWg.Add(1)
		go func() {
			defer insertWg.Done()
			runInsertWorker()
		}()
	}
}

// ShutdownInsertQueue closes the insertQueue channel and blocks until all
// insert workers have flushed their in-flight batches to MongoDB. Call this
// during graceful server shutdown to prevent message loss (#98).
func ShutdownInsertQueue() {
	close(insertQueue)
	insertWg.Wait()
}

// runInsertWorker batches messages from insertQueue and bulk-inserts them.
// It flushes when the batch reaches insertBatchSize or insertBatchTimeout elapses.
func runInsertWorker() {
	batch := make([]mongodb.ChatMessage, 0, insertBatchSize)
	ticker := time.NewTicker(insertBatchTimeout)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if _, err := mongodb.InsertManyChat(batch); err != nil {
			logger.Logger.Errorw("batch InsertManyChat failed",
				"count", len(batch),
				"error", err,
			)
		}
		batch = batch[:0]
	}

	for {
		select {
		case msg, ok := <-insertQueue:
			if !ok {
				flush()
				return
			}
			batch = append(batch, msg)
			if len(batch) >= insertBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// Client represents a connected WebSocket client
type Client struct {
	Hub       *Hub
	Conn      *websocket.Conn
	Send      chan mongodb.ChatMessage // per-client send buffer
	Username  string
	RoomID    string
	msgLimit  *rate.Limiter
	closeOnce sync.Once
}

// closeSend closes the Send channel exactly once, preventing double-close panics.
func (c *Client) closeSend() {
	c.closeOnce.Do(func() { close(c.Send) })
}

// writePump pumps messages from the Send channel to the WebSocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel — send a clean close frame.
				closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "connection closed")
				_ = c.Conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(writeWait))
				return
			}
			if err := c.Conn.WriteJSON(msg); err != nil {
				logger.Logger.Warnw("writePump WriteJSON error",
					"username", c.Username,
					"error", err,
				)
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
			// Refresh Redis presence TTL so the user stays marked online (#100).
			if err := redisclient.RefreshOnline(context.Background(), c.RoomID, c.Username); err != nil {
				logger.Logger.Warnw("writePump: RefreshOnline failed",
					"room_id", c.RoomID,
					"username", c.Username,
					"error", err,
				)
			}
		}
	}
}

// hubSendTimeout is the maximum time readPump will wait to deliver a message
// to the hub's Broadcast or Unregister channels. If the hub has already
// exited, the send would block forever without this guard.
const hubSendTimeout = 5 * time.Second

// readPump pumps messages from the WebSocket to the hub's Broadcast channel.
func (c *Client) readPump() {
	defer func() {
		logger.Logger.Infow("client disconnected",
			"room_id", c.RoomID,
			"username", c.Username,
		)
		metrics.ActiveWSConnections.Dec()
		// Send CLOSE event to notify other clients.
		// Use select+timeout to avoid blocking forever if the hub has exited.
		msg := mongodb.ChatMessage{
			Event:  "CLOSE",
			User:   c.Username,
			RoomID: c.RoomID,
		}
		select {
		case c.Hub.Broadcast <- msg:
		case <-c.Hub.stop:
		case <-time.After(hubSendTimeout):
			logger.Logger.Warnw("readPump: Broadcast send timed out, hub may be gone",
				"room_id", c.RoomID,
				"username", c.Username,
			)
		}
		select {
		case c.Hub.Unregister <- c:
		case <-c.Hub.stop:
		case <-time.After(hubSendTimeout):
			logger.Logger.Warnw("readPump: Unregister send timed out, hub may be gone",
				"room_id", c.RoomID,
				"username", c.Username,
			)
		}
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		var msg mongodb.ChatMessage
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseNormalClosure,
				websocket.CloseAbnormalClosure,
			) {
				logger.Logger.Warnw("readPump unexpected close error",
					"username", c.Username,
					"error", err,
				)
			}
			return
		}
		msg.User = c.Username
		msg.RoomID = c.RoomID

		// Whitelist: only accept MSG, MSG_FILE and TYPING_* events from clients.
		// Reject any other event type (e.g. OPEN, CLOSE, WARN) to prevent
		// clients from spoofing server-generated events (#102, #257).
		if msg.Event != "MSG" && msg.Event != "MSG_FILE" && msg.Event != "TYPING_START" && msg.Event != "TYPING_STOP" {
			logger.Logger.Warnw("readPump: rejected disallowed event type",
				"username", c.Username,
				"event", msg.Event,
			)
			continue
		}

		// Typing events: update Redis presence and broadcast without saving to MongoDB
		if msg.Event == "TYPING_START" || msg.Event == "TYPING_STOP" {
			select {
			case c.Hub.Broadcast <- msg:
			case <-c.Hub.stop:
				return
			}
			continue
		}

		// Validate: reject empty messages (#262)
		if strings.TrimSpace(msg.Message) == "" {
			continue
		}

		// Rate limit: 30 msg/min per client
		if !c.msgLimit.Allow() {
			warn := mongodb.ChatMessage{
				User:    "system",
				Message: "메시지 전송이 너무 빠릅니다. 잠시 후 다시 시도해주세요.",
				RoomID:  c.RoomID,
				Event:   "WARN",
			}
			select {
			case c.Send <- warn:
			default:
			}
			continue
		}

		// Validate message length (max 2000 chars)
		if len([]rune(msg.Message)) > 2000 {
			warn := mongodb.ChatMessage{
				User:    "system",
				Message: "메시지는 2000자를 초과할 수 없습니다.",
				RoomID:  c.RoomID,
				Event:   "WARN",
			}
			select {
			case c.Send <- warn:
			default:
			}
			continue
		}

		// Sanitize message content
		msg.Message = html.EscapeString(msg.Message)

		// Generate stable message ID and timestamp for the broadcast so clients
		// receive consistent metadata immediately (before the async DB insert).
		msg.MessageID = primitive.NewObjectID().Hex()
		msg.CreatedAt = time.Now().Format("2006-01-02T15:04:05Z07:00")

		if msg.Event == "MSG" {
			chatMessage := mongodb.ChatMessage{
				User:    msg.User,
				Message: msg.Message,
				RoomID:  c.RoomID,
			}
			select {
			case insertQueue <- chatMessage:
				metrics.MessagesTotal.Inc()
			case <-time.After(insertTimeout):
				logger.Logger.Warnw("insertQueue full after timeout, dropping message",
					"room_id", c.RoomID,
					"username", c.Username,
				)
				metrics.MessagesDropped.Inc()
				// Notify client that message was dropped (#260)
				warn := mongodb.ChatMessage{
					User:    "system",
					Message: "메시지 저장에 실패했습니다. 잠시 후 다시 시도해주세요.",
					RoomID:  c.RoomID,
					Event:   "WARN",
				}
				select {
				case c.Send <- warn:
				default:
				}
				continue
			}
		}

		select {
		case c.Hub.Broadcast <- msg:
		case <-c.Hub.stop:
			return
		}
	}
}

// newClient creates a Client for the given hub/connection/user.
func newClient(hub *Hub, ws *websocket.Conn, username, roomID string) *Client {
	return &Client{
		Hub:      hub,
		Conn:     ws,
		Send:     make(chan mongodb.ChatMessage, 256),
		Username: username,
		RoomID:   roomID,
		msgLimit: middleware.NewWSMessageLimiter(),
	}
}
