package cmd

import (
	"context"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/spf13/cobra"
	"github.com/woonglife62/woongkie-talkie/pkg/config"
	"github.com/woonglife62/woongkie-talkie/pkg/config/db"
	"github.com/woonglife62/woongkie-talkie/pkg/logger"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
	redisclient "github.com/woonglife62/woongkie-talkie/pkg/redis"
	"github.com/woonglife62/woongkie-talkie/server/handler"
	"github.com/woonglife62/woongkie-talkie/server/router"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start serve",
	Long: `
1. Start Simple Chat Server.`,
	Run: func(cmd *cobra.Command, args []string) {
		// pprof profiling server (dev mode only)
		if config.Config.IsDev == "dev" || config.Config.IsDev == "DEV" || config.Config.IsDev == "develop" {
			go func() {
				logger.Logger.Info("pprof available at http://localhost:6060/debug/pprof/")
				http.ListenAndServe(":6060", nil)
			}()
		}

		// Explicit MongoDB initialization
		if err := db.Initialize(); err != nil {
			logger.Logger.Warnw("MongoDB initialization failed", "error", err)
			// Don't fatal - allow running without DB for development
		}
		if db.DB != nil {
			if err := mongodb.InitAll(db.DB); err != nil {
				logger.Logger.Errorw("MongoDB collections init failed", "error", err)
			}
		}

		// Redis initialization (optional - fallback to in-memory if unavailable)
		if err := redisclient.Initialize(config.RedisConfig.Addr, config.RedisConfig.Password, config.RedisConfig.DB); err != nil {
			logger.Logger.Warnw("Redis initialization failed - running in fallback mode", "error", err)
		} else {
			logger.Logger.Infow("Redis connected", "addr", config.RedisConfig.Addr)
			broker := redisclient.NewBroker(redisclient.Client())
			handler.RoomMgr.SetBroker(broker)
		}

		e := echo.New()

		router.Router(e)

		// Start server in goroutine
		go func() {
			var err error
			if config.TLSConfig.CertFile != "" && config.TLSConfig.KeyFile != "" {
				err = e.StartTLS(":8080", config.TLSConfig.CertFile, config.TLSConfig.KeyFile)
			} else {
				err = e.Start(":8080")
			}
			if err != nil && err != http.ErrServerClosed {
				e.Logger.Fatal("shutting down the server")
			}
		}()

		// Wait for interrupt signal
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		// Graceful shutdown with 30s timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Close all WebSocket connections
		handler.RoomMgr.ShutdownAll()

		if err := e.Shutdown(ctx); err != nil {
			e.Logger.Fatal(err)
		}

		// Close Redis
		redisclient.Close()

		// Disconnect MongoDB
		db.Disconnect()
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
