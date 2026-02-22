package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
)

type UploadPublicKeyRequest struct {
	PublicKey string `json:"public_key"`
}

// UploadPublicKeyHandler handles PUT /crypto/keys
// Stores the authenticated user's RSA public key (JWK JSON string).
func UploadPublicKeyHandler(c echo.Context) error {
	username := GetUsername(c)
	if username == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "인증이 필요합니다"})
	}

	var req UploadPublicKeyRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "잘못된 요청입니다"})
	}
	if req.PublicKey == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "공개 키가 필요합니다"})
	}
	if len(req.PublicKey) > 4096 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "공개 키가 너무 큽니다"})
	}

	if err := mongodb.UpdateUserPublicKey(username, req.PublicKey); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "공개 키 저장에 실패했습니다"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "공개 키가 저장되었습니다"})
}

// GetPublicKeyHandler handles GET /crypto/keys/:username
// Returns the RSA public key for the given username.
func GetPublicKeyHandler(c echo.Context) error {
	username := c.Param("username")
	if username == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "사용자 이름이 필요합니다"})
	}

	publicKey, err := mongodb.GetUserPublicKey(username)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "사용자를 찾을 수 없습니다"})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"username":   username,
		"public_key": publicKey,
	})
}

// GetRoomKeysHandler handles GET /rooms/:id/keys
// Returns the public keys for all members of the room.
func GetRoomKeysHandler(c echo.Context) error {
	roomID := c.Param("id")
	if roomID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "방 ID가 필요합니다"})
	}

	room, err := mongodb.FindRoomByID(roomID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "채팅방을 찾을 수 없습니다"})
	}

	keys := make(map[string]string)
	for _, member := range room.Members {
		pubKey, err := mongodb.GetUserPublicKey(member)
		if err == nil && pubKey != "" {
			keys[member] = pubKey
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"keys": keys,
	})
}
