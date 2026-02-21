package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
)

func GetUsername(c echo.Context) string {
	v, ok := c.Get("username").(string)
	if !ok {
		return ""
	}
	return v
}

// requireRoomMember verifies the authenticated user is a member of the room
// identified by the ":id" path parameter.
// Public rooms (IsPublic=true) and the default room are always accessible.
// Returns a non-nil echo.HTTPError when access should be denied, nil otherwise.
func requireRoomMember(c echo.Context) error {
	roomID := c.Param("id")
	if roomID == "" {
		return nil
	}

	room, err := mongodb.FindRoomByID(roomID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "채팅방을 찾을 수 없습니다")
	}

	// Public rooms and the default room are open to everyone.
	if room.IsPublic || room.IsDefault {
		return nil
	}

	username := GetUsername(c)
	if username == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "인증이 필요합니다")
	}

	for _, member := range room.Members {
		if member == username {
			return nil
		}
	}

	return echo.NewHTTPError(http.StatusForbidden, "채팅방 멤버만 접근할 수 있습니다")
}
