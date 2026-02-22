package handler

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
)

type UploadPublicKeyRequest struct {
	PublicKey string `json:"public_key"`
}

// validateJWK performs basic structural validation of a JWK JSON string.
// #256/#231/#192: ensure public key is valid JWK format before storing.
func validateJWK(key string) bool {
	var jwk map[string]interface{}
	if err := json.Unmarshal([]byte(key), &jwk); err != nil {
		return false
	}
	// A JWK must have at minimum a "kty" field.
	kty, ok := jwk["kty"].(string)
	if !ok || kty == "" {
		return false
	}
	return true
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

	// #256/#231/#192: validate JWK format
	if !validateJWK(req.PublicKey) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "유효하지 않은 JWK 형식입니다"})
	}

	if err := mongodb.UpdateUserPublicKey(username, req.PublicKey); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "공개 키 저장에 실패했습니다"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "공개 키가 저장되었습니다"})
}

// GetPublicKeyHandler handles GET /crypto/keys/:username
// Returns the RSA public key for the given username.
// #191: public keys are intentionally public for E2E encryption key exchange.
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
// #254: only room members can access the keys
// #245: batch-fetch public keys to avoid N+1 queries
func GetRoomKeysHandler(c echo.Context) error {
	roomID := c.Param("id")
	if roomID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "방 ID가 필요합니다"})
	}

	// #254: verify caller is a member of the room
	callerUsername := GetUsername(c)
	if callerUsername == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "인증이 필요합니다"})
	}

	room, err := mongodb.FindRoomByID(roomID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "채팅방을 찾을 수 없습니다"})
	}

	// Check membership
	isMember := false
	for _, member := range room.Members {
		if member == callerUsername {
			isMember = true
			break
		}
	}
	if !isMember {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "채팅방 멤버만 접근할 수 있습니다"})
	}

	// #245: batch-fetch all member public keys in one query
	keys, err := mongodb.GetBatchPublicKeys(room.Members)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "공개 키 조회에 실패했습니다"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"keys": keys,
	})
}
