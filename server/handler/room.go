package handler

import (
	"errors"
	"html"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/woonglife62/woongkie-talkie/pkg/logger"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
	"go.uber.org/zap"
)

type CreateRoomRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
	Password    string `json:"password,omitempty"`
	MaxMembers  int    `json:"max_members"`
}

type JoinRoomRequest struct {
	Password string `json:"password,omitempty"`
}

// RoomResponse wraps a Room for the API response.
// #204: Members field is intentionally omitted to prevent leaking membership info.
type RoomResponse struct {
	ID          interface{} `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	IsPublic    bool        `json:"is_public"`
	MaxMembers  int         `json:"max_members"`
	CreatedBy   string      `json:"created_by"`
	CreatedAt   time.Time   `json:"created_at"`
	IsDefault   bool        `json:"is_default"`
	MemberCount int         `json:"member_count"`
	OnlineMembers []string  `json:"online_members"`
	HasPassword bool        `json:"has_password"`
}

func roomToResponse(room mongodb.Room, online []string) RoomResponse {
	return RoomResponse{
		ID:            room.ID,
		Name:          room.Name,
		Description:   room.Description,
		IsPublic:      room.IsPublic,
		MaxMembers:    room.MaxMembers,
		CreatedBy:     room.CreatedBy,
		CreatedAt:     room.CreatedAt,
		IsDefault:     room.IsDefault,
		MemberCount:   len(room.Members),
		OnlineMembers: online,
		HasPassword:   room.Password != "",
	}
}

// POST /rooms
// #190: JWT middleware already guards this route (by design).
func CreateRoomHandler(c echo.Context) error {
	var req CreateRoomRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "잘못된 요청입니다"})
	}
	if req.Name == "" || len(req.Name) > 50 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "채팅방 이름은 1자 이상 50자 이하이어야 합니다"})
	}
	if len(req.Description) > 200 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "채팅방 설명은 200자 이하이어야 합니다"})
	}
	if req.MaxMembers < 0 || req.MaxMembers > 1000 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "최대 인원은 0 이상 1000 이하이어야 합니다"})
	}

	// #188: XSS prevention - escape room name and description
	req.Name = html.EscapeString(req.Name)
	req.Description = html.EscapeString(req.Description)

	username := GetUsername(c)

	hashedPassword, err := mongodb.HashRoomPassword(req.Password)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "비밀번호 처리에 실패했습니다"})
	}

	room := mongodb.Room{
		Name:        req.Name,
		Description: req.Description,
		IsPublic:    req.IsPublic,
		Password:    hashedPassword,
		MaxMembers:  req.MaxMembers,
		CreatedBy:   username,
		IsDefault:   false,
		Members:     []string{username},
	}

	created, err := mongodb.CreateRoom(room)
	if err != nil {
		// #136: return meaningful error for duplicate room name
		if errors.Is(err, mongodb.ErrDuplicateRoomName) {
			return c.JSON(http.StatusConflict, map[string]string{"error": "이미 존재하는 채팅방 이름입니다"})
		}
		logger.AuditLog("room_create_failed", username, zap.String("room_name", req.Name), zap.String("ip", c.RealIP()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "채팅방 생성에 실패했습니다"})
	}

	logger.AuditLog("room_created", username, zap.String("room_id", created.ID.Hex()), zap.String("room_name", created.Name), zap.String("ip", c.RealIP()))
	return c.JSON(http.StatusCreated, roomToResponse(*created, []string{username}))
}

// GET /rooms
// #280: include private rooms where user is a member
// #204: response omits Members field
func ListRoomsHandler(c echo.Context) error {
	username := GetUsername(c)
	rooms, err := mongodb.FindRooms(username)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "채팅방 목록 조회에 실패했습니다"})
	}

	response := make([]RoomResponse, 0, len(rooms))
	for _, room := range rooms {
		online := RoomMgr.GetOnlineMembers(room.ID.Hex())
		response = append(response, roomToResponse(room, online))
	}

	return c.JSON(http.StatusOK, response)
}

// GET /rooms/:id
func GetRoomHandler(c echo.Context) error {
	id := c.Param("id")
	room, err := mongodb.FindRoomByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "채팅방을 찾을 수 없습니다"})
	}

	online := RoomMgr.GetOnlineMembers(id)
	return c.JSON(http.StatusOK, roomToResponse(*room, online))
}

// DELETE /rooms/:id
// #137: distinguish NotFound vs Forbidden errors
func DeleteRoomHandler(c echo.Context) error {
	id := c.Param("id")
	username := GetUsername(c)

	err := mongodb.DeleteRoom(id, username)
	if err != nil {
		if errors.Is(err, mongodb.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "채팅방을 찾을 수 없습니다"})
		}
		if errors.Is(err, mongodb.ErrForbidden) {
			logger.AuditLog("room_delete_forbidden", username, zap.String("room_id", id), zap.String("ip", c.RealIP()))
			return c.JSON(http.StatusForbidden, map[string]string{"error": "채팅방 삭제 권한이 없습니다"})
		}
		logger.AuditLog("room_delete_failed", username, zap.String("room_id", id), zap.String("ip", c.RealIP()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "채팅방 삭제에 실패했습니다"})
	}

	RoomMgr.RemoveHub(id)

	logger.AuditLog("room_deleted", username, zap.String("room_id", id), zap.String("ip", c.RealIP()))
	return c.JSON(http.StatusOK, map[string]string{"message": "채팅방이 삭제되었습니다"})
}

// POST /rooms/:id/join
// #209: strengthen password validation - require password for private rooms even if empty
func JoinRoomHandler(c echo.Context) error {
	id := c.Param("id")
	username := GetUsername(c)

	room, err := mongodb.FindRoomByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "채팅방을 찾을 수 없습니다"})
	}

	// #209/#134: for private rooms with a password, always require and verify the password
	if !room.IsPublic && room.Password != "" {
		var req JoinRoomRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "잘못된 요청입니다"})
		}
		// #209: reject empty password attempts for private password-protected rooms
		if req.Password == "" {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "비밀번호가 필요합니다"})
		}
		if !mongodb.CheckRoomPassword(room, req.Password) {
			logger.AuditLog("room_join_failed", username, zap.String("room_id", id), zap.String("reason", "wrong_password"), zap.String("ip", c.RealIP()))
			return c.JSON(http.StatusForbidden, map[string]string{"error": "비밀번호가 올바르지 않습니다"})
		}
	}

	err = mongodb.JoinRoom(id, username)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "채팅방 참여에 실패했습니다"})
	}

	logger.AuditLog("room_joined", username, zap.String("room_id", id), zap.String("ip", c.RealIP()))
	return c.JSON(http.StatusOK, map[string]string{"message": "채팅방에 참여했습니다"})
}

// POST /rooms/:id/leave
func LeaveRoomHandler(c echo.Context) error {
	id := c.Param("id")
	username := GetUsername(c)

	err := mongodb.LeaveRoom(id, username)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "채팅방 퇴장에 실패했습니다"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "채팅방에서 나갔습니다"})
}

type MessagesResponse struct {
	Messages []mongodb.Chat `json:"messages"`
	HasMore  bool           `json:"has_more"`
}

// GET /rooms/:id/messages
// #251/#160: fix HasMore calculation using count-based approach
func GetRoomMessagesHandler(c echo.Context) error {
	if err := requireRoomMember(c); err != nil {
		return err
	}
	id := c.Param("id")

	beforeStr := c.QueryParam("before")
	afterStr := c.QueryParam("after")
	limitStr := c.QueryParam("limit")

	limit := int64(50)
	if limitStr != "" {
		if l, err := strconv.ParseInt(limitStr, 10, 64); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	if afterStr != "" {
		after, err := time.Parse(time.RFC3339, afterStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "잘못된 시간 형식입니다"})
		}
		chats, err := mongodb.FindChatByRoomAfter(id, after)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "메시지 조회에 실패했습니다"})
		}
		if chats == nil {
			chats = []mongodb.Chat{}
		}
		return c.JSON(http.StatusOK, MessagesResponse{Messages: chats, HasMore: false})
	}

	if beforeStr != "" {
		before, err := time.Parse(time.RFC3339, beforeStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "잘못된 시간 형식입니다"})
		}
		chats, err := mongodb.FindChatByRoomBefore(id, before, limit)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "메시지 조회에 실패했습니다"})
		}
		if chats == nil {
			chats = []mongodb.Chat{}
		}
		// #251/#160: HasMore is true only if we got exactly limit results (meaning there may be more)
		return c.JSON(http.StatusOK, MessagesResponse{Messages: chats, HasMore: int64(len(chats)) == limit})
	}

	chats, err := mongodb.FindChatByRoom(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "메시지 조회에 실패했습니다"})
	}
	if chats == nil {
		chats = []mongodb.Chat{}
	}
	// #251: HasMore true only when we hit the hard limit of 100
	return c.JSON(http.StatusOK, MessagesResponse{Messages: chats, HasMore: len(chats) >= 100})
}

// GET /rooms/default
// #182: /rooms/default is registered before /rooms/:id in the router (by design).
func GetDefaultRoomHandler(c echo.Context) error {
	room, err := mongodb.FindDefaultRoom()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "기본 채팅방을 찾을 수 없습니다"})
	}
	online := RoomMgr.GetOnlineMembers(room.ID.Hex())
	return c.JSON(http.StatusOK, roomToResponse(*room, online))
}
