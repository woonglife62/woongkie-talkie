package handler

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/woonglife62/woongkie-talkie/pkg/config"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
)

var usernameRegexp = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,30}$`)

type RegisterRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string       `json:"token"`
	User  mongodb.User `json:"user"`
}

func RegisterHandler(c echo.Context) error {
	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "잘못된 요청입니다"})
	}

	if !usernameRegexp.MatchString(req.Username) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "사용자 이름은 3자 이상 30자 이하의 영문자, 숫자, _, -만 사용 가능합니다"})
	}
	if len(req.Password) < 6 || len(req.Password) > 72 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "비밀번호는 6자 이상 72자 이하이어야 합니다"})
	}

	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Username
	}

	user, err := mongodb.CreateUser(req.Username, req.Password, displayName)
	if err != nil {
		return c.JSON(http.StatusConflict, map[string]string{"error": "이미 존재하는 사용자 이름입니다"})
	}

	token, err := generateToken(user.Username)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "토큰 생성에 실패했습니다"})
	}

	return c.JSON(http.StatusCreated, AuthResponse{Token: token, User: *user})
}

func LoginHandler(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "잘못된 요청입니다"})
	}

	user, err := mongodb.FindUserByUsername(req.Username)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "사용자 이름 또는 비밀번호가 올바르지 않습니다"})
	}

	if !mongodb.CheckPassword(user, req.Password) {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "사용자 이름 또는 비밀번호가 올바르지 않습니다"})
	}

	token, err := generateToken(user.Username)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "토큰 생성에 실패했습니다"})
	}

	return c.JSON(http.StatusOK, AuthResponse{Token: token, User: *user})
}

func MeHandler(c echo.Context) error {
	username := GetUsername(c)

	user, err := mongodb.FindUserByUsername(username)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "사용자를 찾을 수 없습니다"})
	}

	return c.JSON(http.StatusOK, user)
}

func RefreshHandler(c echo.Context) error {
	auth := c.Request().Header.Get("Authorization")
	tokenString := ""
	if strings.HasPrefix(auth, "Bearer ") {
		tokenString = strings.TrimPrefix(auth, "Bearer ")
	}
	if tokenString == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "토큰이 필요합니다"})
	}

	// Parse and verify signature but skip claims validation (expiry etc.)
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(config.JWTConfig.Secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithoutClaimsValidation())
	if err != nil || !token.Valid {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "유효하지 않은 토큰입니다"})
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || claims.Subject == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "유효하지 않은 토큰입니다"})
	}

	// If token has expiry, check that it is within the refresh grace period
	if claims.ExpiresAt != nil && time.Since(claims.ExpiresAt.Time) > config.RefreshGracePeriod {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "토큰이 만료되었습니다. 다시 로그인하세요"})
	}

	newToken, err := generateToken(claims.Subject)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "토큰 생성에 실패했습니다"})
	}
	return c.JSON(http.StatusOK, map[string]string{"token": newToken})
}

func generateToken(username string) (string, error) {
	expiry, err := time.ParseDuration(config.JWTConfig.Expiry)
	if err != nil {
		expiry = 24 * time.Hour
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   username,
		ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		IssuedAt:  jwt.NewNumericDate(now),
		Issuer:    "woongkie-talkie",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.JWTConfig.Secret))
}
