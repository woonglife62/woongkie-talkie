package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/woonglife62/woongkie-talkie/pkg/logger"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
	"go.uber.org/zap"
)

type EditMessageRequest struct {
	Message string `json:"message"`
}

type ReplyMessageRequest struct {
	Message string `json:"message"`
	ReplyTo string `json:"reply_to"`
}

// PUT /rooms/:id/messages/:msgId
func EditMessageHandler(c echo.Context) error {
	if err := requireRoomMember(c); err != nil {
		return err
	}
	roomID := c.Param("id")
	msgID := c.Param("msgId")
	username := GetUsername(c)
	if username == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "인증이 필요합니다"})
	}

	var req EditMessageRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "잘못된 요청입니다"})
	}
	if req.Message == "" || len([]rune(req.Message)) > 2000 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "메시지는 1자 이상 2000자 이하이어야 합니다"})
	}

	updated, err := mongodb.EditChat(msgID, username, req.Message)
	if err != nil {
		switch {
		case errors.Is(err, mongodb.ErrForbidden):
			logger.AuditLog("message_edit_forbidden", username, zap.String("msg_id", msgID), zap.String("room_id", roomID))
			return c.JSON(http.StatusForbidden, map[string]string{"error": "수정 권한이 없습니다"})
		case errors.Is(err, mongodb.ErrEditWindowExpired):
			return c.JSON(http.StatusForbidden, map[string]string{"error": "5분이 지나 수정할 수 없습니다"})
		case errors.Is(err, mongodb.ErrMessageDeleted):
			return c.JSON(http.StatusGone, map[string]string{"error": "삭제된 메시지입니다"})
		default:
			return c.JSON(http.StatusNotFound, map[string]string{"error": "메시지를 찾을 수 없습니다"})
		}
	}
	logger.AuditLog("message_edited", username, zap.String("msg_id", msgID), zap.String("room_id", roomID))

	// Broadcast MSG_EDIT event to room
	hub := RoomMgr.GetHub(roomID)
	if hub != nil {
		hub.Broadcast <- mongodb.ChatMessage{
			Event:     "MSG_EDIT",
			User:      username,
			Message:   updated.Message,
			RoomID:    roomID,
			MessageID: msgID,
		}
	}

	return c.JSON(http.StatusOK, updated)
}

// DELETE /rooms/:id/messages/:msgId
func DeleteMessageHandler(c echo.Context) error {
	if err := requireRoomMember(c); err != nil {
		return err
	}
	roomID := c.Param("id")
	msgID := c.Param("msgId")
	username := GetUsername(c)
	if username == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "인증이 필요합니다"})
	}

	err := mongodb.DeleteChat(msgID, username)
	if err != nil {
		switch {
		case errors.Is(err, mongodb.ErrForbidden):
			logger.AuditLog("message_delete_forbidden", username, zap.String("msg_id", msgID), zap.String("room_id", roomID))
			return c.JSON(http.StatusForbidden, map[string]string{"error": "삭제 권한이 없습니다"})
		default:
			return c.JSON(http.StatusNotFound, map[string]string{"error": "메시지를 찾을 수 없습니다"})
		}
	}

	logger.AuditLog("message_deleted", username, zap.String("msg_id", msgID), zap.String("room_id", roomID))

	// Broadcast MSG_DELETE event to room
	hub := RoomMgr.GetHub(roomID)
	if hub != nil {
		hub.Broadcast <- mongodb.ChatMessage{
			Event:     "MSG_DELETE",
			User:      username,
			RoomID:    roomID,
			MessageID: msgID,
		}
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "메시지가 삭제되었습니다"})
}

// POST /rooms/:id/messages/:msgId/reply
func ReplyMessageHandler(c echo.Context) error {
	if err := requireRoomMember(c); err != nil {
		return err
	}
	roomID := c.Param("id")
	msgID := c.Param("msgId")
	username := GetUsername(c)
	if username == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "인증이 필요합니다"})
	}

	var req ReplyMessageRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "잘못된 요청입니다"})
	}
	if req.Message == "" || len([]rune(req.Message)) > 2000 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "메시지는 1자 이상 2000자 이하이어야 합니다"})
	}

	// Verify the parent message exists
	_, err := mongodb.FindChatByID(msgID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "원본 메시지를 찾을 수 없습니다"})
	}

	chatMsg := mongodb.ChatMessage{
		User:    username,
		Message: req.Message,
		RoomID:  roomID,
		ReplyTo: msgID,
	}

	saved, err := mongodb.InsertChatWithReply(chatMsg)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "메시지 저장에 실패했습니다"})
	}

	// Broadcast the reply to the room
	hub := RoomMgr.GetHub(roomID)
	if hub != nil {
		hub.Broadcast <- mongodb.ChatMessage{
			Event:     "MSG",
			User:      username,
			Message:   req.Message,
			RoomID:    roomID,
			MessageID: saved.ID.Hex(),
			ReplyTo:   msgID,
		}
	}

	return c.JSON(http.StatusCreated, saved)
}
