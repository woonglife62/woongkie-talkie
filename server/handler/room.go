package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
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

type RoomResponse struct {
	mongodb.Room
	OnlineMembers []string `json:"online_members"`
	HasPassword   bool     `json:"has_password"`
}

// POST /rooms
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
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "채팅방 생성에 실패했습니다"})
	}

	return c.JSON(http.StatusCreated, created)
}

// GET /rooms
func ListRoomsHandler(c echo.Context) error {
	rooms, err := mongodb.FindRooms()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "채팅방 목록 조회에 실패했습니다"})
	}
	if rooms == nil {
		rooms = []mongodb.Room{}
	}

	var response []RoomResponse
	for _, room := range rooms {
		online := RoomMgr.GetOnlineMembers(room.ID.Hex())
		response = append(response, RoomResponse{Room: room, OnlineMembers: online, HasPassword: room.Password != ""})
	}
	if response == nil {
		response = []RoomResponse{}
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

	return c.JSON(http.StatusOK, RoomResponse{Room: *room, OnlineMembers: online, HasPassword: room.Password != ""})
}

// DELETE /rooms/:id
func DeleteRoomHandler(c echo.Context) error {
	id := c.Param("id")
	username := GetUsername(c)

	err := mongodb.DeleteRoom(id, username)
	if err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "채팅방 삭제 권한이 없습니다"})
	}

	RoomMgr.RemoveHub(id)

	return c.JSON(http.StatusOK, map[string]string{"message": "채팅방이 삭제되었습니다"})
}

// POST /rooms/:id/join
func JoinRoomHandler(c echo.Context) error {
	id := c.Param("id")
	username := GetUsername(c)

	room, err := mongodb.FindRoomByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "채팅방을 찾을 수 없습니다"})
	}

	// 비공개 방 비밀번호 확인 (bcrypt)
	if !room.IsPublic && room.Password != "" {
		var req JoinRoomRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "잘못된 요청입니다"})
		}
		if !mongodb.CheckRoomPassword(room, req.Password) {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "비밀번호가 올바르지 않습니다"})
		}
	}

	err = mongodb.JoinRoom(id, username)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "채팅방 참여에 실패했습니다"})
	}

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
		return c.JSON(http.StatusOK, MessagesResponse{Messages: chats, HasMore: len(chats) == int(limit)})
	}

	chats, err := mongodb.FindChatByRoom(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "메시지 조회에 실패했습니다"})
	}
	if chats == nil {
		chats = []mongodb.Chat{}
	}
	return c.JSON(http.StatusOK, MessagesResponse{Messages: chats, HasMore: len(chats) >= 100})
}

// GET /rooms/default
func GetDefaultRoomHandler(c echo.Context) error {
	room, err := mongodb.FindDefaultRoom()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "기본 채팅방을 찾을 수 없습니다"})
	}
	online := RoomMgr.GetOnlineMembers(room.ID.Hex())
	return c.JSON(http.StatusOK, RoomResponse{Room: *room, OnlineMembers: online, HasPassword: room.Password != ""})
}
