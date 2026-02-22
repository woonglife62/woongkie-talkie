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

	if db.Client == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"status": "not ready",
		})
	}

	err := db.Client.Ping(ctx, readpref.Primary())
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"status": "not ready",
		})
	}

	// Also verify Redis if available
	if redisclient.IsAvailable() {
		if pingErr := redisclient.Ping(ctx); pingErr != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"status": "not ready",
			})
		}
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status": "ok",
	})
}

