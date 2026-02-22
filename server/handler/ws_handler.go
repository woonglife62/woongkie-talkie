package handler

import (
	"github.com/labstack/echo/v4"
	"github.com/woonglife62/woongkie-talkie/pkg/logger"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
)

// RoomWebSocket handles WebSocket connections for a specific room
func RoomWebSocket(c echo.Context) error {
	if err := requireRoomMember(c); err != nil {
		return err
	}
	roomID := c.Param("id")

	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}

	clientNm := GetUsername(c)
	hub := RoomMgr.GetOrCreateHub(roomID)
	client := newClient(hub, ws, clientNm, roomID)

	logger.Logger.Infow("client connected",
		"room_id", roomID,
		"username", clientNm,
	)

	// Queue history into the Send buffer before registering with the hub.
	// This guarantees history messages are ordered before any live broadcasts.
	// Limit to last 256 messages to avoid overflowing the Send buffer (#138).
	chatList, err := mongodb.FindChatByRoom(roomID)
	if err == nil {
		if len(chatList) > 256 {
			chatList = chatList[len(chatList)-256:]
		}
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
			// Non-blocking send: drop history if buffer is full (#265)
			select {
			case client.Send <- tmpMsg:
			default:
			}
		}
	}

	// Start writePump before registering so the channel is being drained
	// before the hub can send any live messages.
	go client.writePump()
	hub.Register <- client
	client.readPump() // blocking
	return nil
}

// MsgReceiver handles the legacy /server WebSocket endpoint (backward compatible)
func MsgReceiver(c echo.Context) error {
	// Require authentication (#266)
	clientNm := GetUsername(c)
	if clientNm == "" {
		return echo.NewHTTPError(401, "인증이 필요합니다")
	}

	defaultRoom, err := mongodb.FindDefaultRoom()
	if err != nil {
		return echo.NewHTTPError(500, "기본 채팅방을 찾을 수 없습니다")
	}

	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}

	roomID := defaultRoom.ID.Hex()

	hub := RoomMgr.GetOrCreateHub(roomID)
	client := newClient(hub, ws, clientNm, roomID)

	logger.Logger.Infow("client connected",
		"room_id", roomID,
		"username", clientNm,
	)

	// Queue history before registering to avoid race with live broadcasts.
	// Limit to last 256 messages to avoid overflowing the Send buffer (#138).
	chatList, err := mongodb.FindChatByRoom(roomID)
	if err == nil {
		if len(chatList) > 256 {
			chatList = chatList[len(chatList)-256:]
		}
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
			// Non-blocking send: drop history if buffer is full (#265)
			select {
			case client.Send <- tmpMsg:
			default:
			}
		}
	}

	go client.writePump()
	hub.Register <- client
	client.readPump() // blocking
	return nil
}
