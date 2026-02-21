package handler

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10 // 54s
	maxMessageSize = 64 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// Client represents a connected WebSocket client
type Client struct {
	Hub      *Hub
	Conn     *websocket.Conn
	Send     chan mongodb.ChatMessage // per-client send buffer
	Username string
	RoomID   string
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
				// Hub closed the channel.
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteJSON(msg); err != nil {
				log.Printf("writePump WriteJSON error for %s: %v", c.Username, err)
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
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("readPump error for %s: %v", c.Username, err)
			}
			return
		}
		msg.User = c.Username
		msg.RoomID = c.RoomID

		if msg.Event != "OPEN" {
			chatMessage := mongodb.ChatMessage{
				User:    msg.User,
				Message: msg.Message,
				RoomID:  c.RoomID,
			}
			if err := mongodb.InsertChat(chatMessage); err != nil {
				log.Print(err)
			}
		}

		c.Hub.Broadcast <- msg
	}
}

// Hub manages WebSocket connections for a single room
type Hub struct {
	RoomID     string
	Clients    map[*Client]bool
	Broadcast  chan mongodb.ChatMessage
	Register   chan *Client
	Unregister chan *Client
	stop       chan struct{}
	mu         sync.RWMutex
}

func NewHub(roomID string) *Hub {
	return &Hub{
		RoomID:     roomID,
		Clients:    make(map[*Client]bool),
		Broadcast:  make(chan mongodb.ChatMessage, 256),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		stop:       make(chan struct{}),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case <-h.stop:
			return

		case client := <-h.Register:
			h.mu.Lock()
			h.Clients[client] = true
			h.mu.Unlock()

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send)
			}
			h.mu.Unlock()

		case msg := <-h.Broadcast:
			msgFulltxt := msg.Message
			h.mu.Lock()
			for client := range h.Clients {
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
					// Slow client: close and remove.
					close(client.Send)
					delete(h.Clients, client)
				}
			}
			h.mu.Unlock()
		}
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

// RoomManager manages all room hubs
type roomManager struct {
	hubs map[string]*Hub
	mu   sync.RWMutex
}

// RoomMgr is the global room manager instance
var RoomMgr = &roomManager{
	hubs: make(map[string]*Hub),
}

func (rm *roomManager) GetOrCreateHub(roomID string) *Hub {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if hub, ok := rm.hubs[roomID]; ok {
		return hub
	}
	hub := NewHub(roomID)
	rm.hubs[roomID] = hub
	go hub.Run()
	return hub
}

func (rm *roomManager) GetHub(roomID string) *Hub {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.hubs[roomID]
}

// RemoveHub closes all connections and removes the hub for a room
func (rm *roomManager) RemoveHub(roomID string) {
	rm.mu.Lock()
	hub, ok := rm.hubs[roomID]
	if ok {
		delete(rm.hubs, roomID)
	}
	rm.mu.Unlock()
	if hub != nil {
		close(hub.stop)
		hub.mu.Lock()
		for client := range hub.Clients {
			close(client.Send)
			client.Conn.Close()
			delete(hub.Clients, client)
		}
		hub.mu.Unlock()
	}
}

func (rm *roomManager) ShutdownAll() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	for id, hub := range rm.hubs {
		close(hub.stop)
		hub.mu.Lock()
		for client := range hub.Clients {
			close(client.Send)
			client.Conn.Close()
			delete(hub.Clients, client)
		}
		hub.mu.Unlock()
		delete(rm.hubs, id)
	}
}

// GetOnlineMembers returns online usernames for a room
func (rm *roomManager) GetOnlineMembers(roomID string) []string {
	hub := rm.GetHub(roomID)
	if hub == nil {
		return []string{}
	}
	return hub.GetMemberNames()
}

// RoomWebSocket handles WebSocket connections for a specific room
func RoomWebSocket(c echo.Context) error {
	roomID := c.Param("id")

	_, err := mongodb.FindRoomByID(roomID)
	if err != nil {
		return echo.NewHTTPError(404, "채팅방을 찾을 수 없습니다")
	}

	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}

	clientNm := GetUsername(c)
	hub := RoomMgr.GetOrCreateHub(roomID)
	client := &Client{
		Hub:      hub,
		Conn:     ws,
		Send:     make(chan mongodb.ChatMessage, 256),
		Username: clientNm,
		RoomID:   roomID,
	}

	hub.Register <- client

	// Send chat history directly via client.Send before starting writePump goroutine
	chatList, err := mongodb.FindChatByRoom(roomID)
	if err == nil {
		for _, pastChat := range chatList {
			tmpMsg := mongodb.ChatMessage{
				User:    pastChat.User,
				Message: pastChat.Message,
				RoomID:  roomID,
				Event:   "CHATLOG",
			}
			if pastChat.User == clientNm {
				tmpMsg.Owner = true
			}
			client.Send <- tmpMsg
		}
	}

	go client.writePump()
	client.readPump() // blocking
	return nil
}

// MsgReceiver handles the legacy /server WebSocket endpoint (backward compatible)
func MsgReceiver(c echo.Context) error {
	defaultRoom, err := mongodb.FindDefaultRoom()
	if err != nil {
		return echo.NewHTTPError(500, "기본 채팅방을 찾을 수 없습니다")
	}

	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}

	clientNm := GetUsername(c)
	roomID := defaultRoom.ID.Hex()

	hub := RoomMgr.GetOrCreateHub(roomID)
	client := &Client{
		Hub:      hub,
		Conn:     ws,
		Send:     make(chan mongodb.ChatMessage, 256),
		Username: clientNm,
		RoomID:   roomID,
	}

	hub.Register <- client

	chatList, err := mongodb.FindChatByRoom(roomID)
	if err == nil {
		for _, pastChat := range chatList {
			tmpMsg := mongodb.ChatMessage{
				User:    pastChat.User,
				Message: pastChat.Message,
				Event:   "CHATLOG",
			}
			if pastChat.User == clientNm {
				tmpMsg.Owner = true
			}
			client.Send <- tmpMsg
		}
	}

	go client.writePump()
	client.readPump() // blocking
	return nil
}
