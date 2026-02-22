package handler

import (
	"net/http"
	"os"
	"runtime"
	"strconv"

	"github.com/labstack/echo/v4"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
)

// AdminStatsHandler handles GET /admin/stats
func AdminStatsHandler(c echo.Context) error {
	userCount, err := mongodb.CountUsers()
	if err != nil {
		userCount = 0
	}
	roomCount, err := mongodb.CountRooms()
	if err != nil {
		roomCount = 0
	}
	todayMessages, err := mongodb.CountTodayMessages()
	if err != nil {
		todayMessages = 0
	}

	// Count online users across all hubs
	onlineUsers := 0
	RoomMgr.mu.RLock()
	for _, hub := range RoomMgr.hubs {
		onlineUsers += len(hub.GetMemberNames())
	}
	RoomMgr.mu.RUnlock()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"online_users":   onlineUsers,
		"active_rooms":   roomCount,
		"today_messages": todayMessages,
		"goroutines":     runtime.NumGoroutine(),
		"total_users":    userCount,
	})
}

// AdminUsersHandler handles GET /admin/users?page=1&limit=20
func AdminUsersHandler(c echo.Context) error {
	page, _ := strconv.ParseInt(c.QueryParam("page"), 10, 64)
	limit, _ := strconv.ParseInt(c.QueryParam("limit"), 10, 64)
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	users, total, err := mongodb.FindAllUsers(page, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "사용자 목록을 불러올 수 없습니다"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"users": users,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// AdminBlockUserHandler handles PUT /admin/users/:username/block
func AdminBlockUserHandler(c echo.Context) error {
	username := c.Param("username")
	if username == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "사용자 이름이 필요합니다"})
	}

	var req struct {
		Blocked bool `json:"blocked"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "잘못된 요청입니다"})
	}

	var err error
	if req.Blocked {
		err = mongodb.BlockUser(username)
	} else {
		err = mongodb.UnblockUser(username)
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "사용자 상태 변경에 실패했습니다"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "사용자 상태가 변경되었습니다"})
}

// AdminRoomsHandler handles GET /admin/rooms?page=1&limit=20
func AdminRoomsHandler(c echo.Context) error {
	page, _ := strconv.ParseInt(c.QueryParam("page"), 10, 64)
	limit, _ := strconv.ParseInt(c.QueryParam("limit"), 10, 64)
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	rooms, total, err := mongodb.FindAllRooms(page, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "채팅방 목록을 불러올 수 없습니다"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"rooms": rooms,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// AdminDeleteRoomHandler handles DELETE /admin/rooms/:id
func AdminDeleteRoomHandler(c echo.Context) error {
	roomID := c.Param("id")
	if roomID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "채팅방 ID가 필요합니다"})
	}

	if err := mongodb.AdminDeleteRoom(roomID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "채팅방 삭제에 실패했습니다"})
	}

	// Also remove the hub if it exists
	RoomMgr.RemoveHub(roomID)

	return c.JSON(http.StatusOK, map[string]string{"message": "채팅방이 삭제되었습니다"})
}

// AdminAnnounceHandler handles POST /admin/rooms/:id/announce
func AdminAnnounceHandler(c echo.Context) error {
	roomID := c.Param("id")
	if roomID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "채팅방 ID가 필요합니다"})
	}

	var req struct {
		Message string `json:"message"`
	}
	if err := c.Bind(&req); err != nil || req.Message == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "공지 메시지가 필요합니다"})
	}

	hub := RoomMgr.GetHub(roomID)
	if hub == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "활성화된 채팅방을 찾을 수 없습니다"})
	}

	hub.Broadcast <- mongodb.ChatMessage{
		Event:   "MSG",
		User:    "system",
		Message: "[공지] " + req.Message,
		RoomID:  roomID,
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "공지가 전송되었습니다"})
}

// AdminDashboardPage handles GET /admin
func AdminDashboardPage(c echo.Context) error {
	return c.File(adminViewPath())
}

func adminViewPath() string {
	if _, err := os.Stat("/app/view/admin.html"); err == nil {
		return "/app/view/admin.html"
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		return gopath + "/src/woongkie-talkie/view/admin.html"
	}
	return "./view/admin.html"
}
