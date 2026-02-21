package handler

import (
	"html"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/woonglife62/woongkie-talkie/pkg/config"
	"github.com/woonglife62/woongkie-talkie/pkg/logger"
	"github.com/woonglife62/woongkie-talkie/pkg/metrics"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
	"github.com/woonglife62/woongkie-talkie/server/middleware"
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
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     config.CheckOrigin,
}

// insertQueue is a buffered channel for async MongoDB chat inserts.
var insertQueue = make(chan mongodb.ChatMessage, insertQueueSize)

func init() {
	for i := 0; i < insertWorkers; i++ {
		go func() {
			for msg := range insertQueue {
				if _, err := mongodb.InsertChat(msg); err != nil {
					logger.Logger.Errorw("async InsertChat failed",
						"room_id", msg.RoomID,
						"username", msg.User,
						"error", err,
					)
				}
			}
		}()
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
		}
	}
}

// readPump pumps messages from the WebSocket to the hub's Broadcast channel.
func (c *Client) readPump() {
	defer func() {
		logger.Logger.Infow("client disconnected",
			"room_id", c.RoomID,
			"username", c.Username,
		)
		metrics.ActiveWSConnections.Dec()
		// Send CLOSE event to notify other clients.
		msg := mongodb.ChatMessage{
			Event:  "CLOSE",
			User:   c.Username,
			RoomID: c.RoomID,
		}
		c.Hub.Broadcast <- msg
		c.Hub.Unregister <- c
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

		if msg.Event != "OPEN" {
			// Typing events: update Redis presence and broadcast without saving to MongoDB
			if msg.Event == "TYPING_START" || msg.Event == "TYPING_STOP" {
				c.Hub.Broadcast <- msg
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
			}
		}

		c.Hub.Broadcast <- msg
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
