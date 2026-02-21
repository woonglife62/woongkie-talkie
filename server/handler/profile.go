package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
)

type UpdateProfileRequest struct {
	DisplayName   string `json:"display_name"`
	StatusMessage string `json:"status_message"`
	AvatarURL     string `json:"avatar_url"`
}

// GET /users/:username/profile
func GetProfileHandler(c echo.Context) error {
	username := c.Param("username")
	user, err := mongodb.FindUserByUsername(username)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "사용자를 찾을 수 없습니다"})
	}
	return c.JSON(http.StatusOK, user)
}

// PUT /users/me/profile
func UpdateProfileHandler(c echo.Context) error {
	username := GetUsername(c)
	if username == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "인증이 필요합니다"})
	}

	var req UpdateProfileRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "잘못된 요청입니다"})
	}

	if len([]rune(req.DisplayName)) > 30 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "표시 이름은 30자 이하이어야 합니다"})
	}
	if len([]rune(req.StatusMessage)) > 100 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "상태 메시지는 100자 이하이어야 합니다"})
	}

	displayName := req.DisplayName
	if displayName == "" {
		displayName = username
	}

	user, err := mongodb.UpdateUserProfile(username, displayName, req.StatusMessage, req.AvatarURL)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "프로필 업데이트에 실패했습니다"})
	}

	return c.JSON(http.StatusOK, user)
}
