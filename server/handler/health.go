package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/woonglife62/woongkie-talkie/pkg/config/db"
	redisclient "github.com/woonglife62/woongkie-talkie/pkg/redis"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func HealthHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func ReadyHandler(c echo.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := db.Client.Ping(ctx, readpref.Primary())
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"status":  "not ready",
			"mongodb": "disconnected",
			"redis":   redisStatus(ctx),
		})
	}
	return c.JSON(http.StatusOK, map[string]string{
		"status":  "ok",
		"mongodb": "connected",
		"redis":   redisStatus(ctx),
	})
}

func redisStatus(ctx context.Context) string {
	if !redisclient.IsAvailable() {
		return "disconnected (fallback mode)"
	}
	if err := redisclient.Ping(ctx); err != nil {
		return "disconnected (fallback mode)"
	}
	return "connected"
}
