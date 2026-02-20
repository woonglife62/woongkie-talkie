package handler

import (
	"fmt"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
)

var upgrader = websocket.Upgrader{}

// Client represents a connected WebSocket client
type Client struct {
	Conn     *websocket.Conn
	Username string
	RoomID   string
	mu       sync.Mutex
}

func (c *Client) WriteJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteJSON(v)
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
			}
			h.mu.Unlock()

		case msg := <-h.Broadcast:
			h.mu.RLock()
			msgFulltxt := msg.Message
			var failed []*Client
			for client := range h.Clients {
				msg.Message = msgFulltxt
				if client.Username == msg.User {
					msg.Owner = true
				} else {
					msg.Owner = false
				}
				if msg.Event == "OPEN" {
					msg.Message = fmt.Sprintf("---- %s님이 입장하셨습니다. ----", msg.User)
				} else if msg.Event == "CLOSE" {
					msg.Message = fmt.Sprintf("---- %s님이 퇴장하셨습니다. ----", msg.User)
				}
				err := client.WriteJSON(msg)
				if err != nil {
					log.Printf("error writing to client: %v", err)
					client.Conn.Close()
					failed = append(failed, client)
				}
			}
			h.mu.RUnlock()
			if len(failed) > 0 {
				h.mu.Lock()
				for _, c := range failed {
					delete(h.Clients, c)
				}
				h.mu.Unlock()
			}
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
	defer ws.Close()

	clientNm := GetUsername(c)

	hub := RoomMgr.GetOrCreateHub(roomID)
	client := &Client{
		Conn:     ws,
		Username: clientNm,
		RoomID:   roomID,
	}

	hub.Register <- client
	defer func() {
		hub.Unregister <- client
	}()

	// 채팅 이력 전송
	chatList, err := mongodb.FindChatByRoom(roomID)
	if err == nil {
		for _, pastChat := range chatList {
			var tmpMsg mongodb.ChatMessage
			tmpMsg.User = pastChat.User
			tmpMsg.Message = pastChat.Message
			tmpMsg.RoomID = roomID
			if pastChat.User == clientNm {
				tmpMsg.Owner = true
			} else {
				tmpMsg.Owner = false
			}
			tmpMsg.Event = "CHATLOG"
			client.WriteJSON(&tmpMsg)
		}
	}

	for {
		var msg mongodb.ChatMessage
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("error reading: %v", err)
			msg.Event = "CLOSE"
			msg.User = clientNm
			msg.RoomID = roomID
			hub.Broadcast <- msg
			break
		}
		msg.User = clientNm
		msg.RoomID = roomID

		if msg.Event != "OPEN" {
			chatMessage := mongodb.ChatMessage{
				User:    msg.User,
				Message: msg.Message,
				RoomID:  roomID,
			}
			err = mongodb.InsertChat(chatMessage)
			if err != nil {
				log.Print(err)
			}
		}

		hub.Broadcast <- msg
	}
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
	defer ws.Close()

	clientNm := GetUsername(c)
	roomID := defaultRoom.ID.Hex()

	hub := RoomMgr.GetOrCreateHub(roomID)
	client := &Client{
		Conn:     ws,
		Username: clientNm,
		RoomID:   roomID,
	}

	hub.Register <- client
	defer func() {
		hub.Unregister <- client
	}()

	chatList, err := mongodb.FindChatByRoom(roomID)
	if err == nil {
		for _, pastChat := range chatList {
			var tmpMsg mongodb.ChatMessage
			tmpMsg.User = pastChat.User
			tmpMsg.Message = pastChat.Message
			if pastChat.User == clientNm {
				tmpMsg.Owner = true
			} else {
				tmpMsg.Owner = false
			}
			tmpMsg.Event = "CHATLOG"
			client.WriteJSON(&tmpMsg)
		}
	}

	for {
		var msg mongodb.ChatMessage
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("error: %v", err)
			msg.Event = "CLOSE"
			msg.User = clientNm
			hub.Broadcast <- msg
			break
		}
		msg.User = clientNm
		msg.RoomID = roomID

		if msg.Event != "OPEN" {
			chatMessage := mongodb.ChatMessage{
				User:    msg.User,
				Message: msg.Message,
				RoomID:  roomID,
			}
			err = mongodb.InsertChat(chatMessage)
			if err != nil {
				log.Print(err)
			}
		}

		hub.Broadcast <- msg
	}
	return nil
}
