package router

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/woonglife62/woongkie-talkie/server/handler"
	"github.com/woonglife62/woongkie-talkie/server/middleware"
)

func getViewPath() string {
	// Check if running in container (view at /app/view)
	if _, err := os.Stat("/app/view"); err == nil {
		return "/app/view"
	}
	// Fallback: relative to executable or GOPATH
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		return gopath + "/src/woongkie-talkie/view"
	}
	return "./view"
}

func Router(e *echo.Echo) {
	// pprof는 ENABLE_PPROF=true 환경변수가 설정된 경우에만 활성화 (AdminRequired 적용)
	if os.Getenv("ENABLE_PPROF") == "true" {
		e.GET("/debug/pprof/*", echo.WrapHandler(http.DefaultServeMux), middleware.AdminRequired())
	}

	e.Static("/view", getViewPath())

	middleware.Middleware(e)

	// API 문서
	e.GET("/docs", handler.SwaggerUIHandler)
	e.GET("/docs/openapi.yaml", handler.OpenAPISpecHandler)

	// 헬스체크 (인증 불필요)
	e.GET("/health", handler.HealthHandler)
	e.GET("/ready", handler.ReadyHandler)

	// Prometheus 메트릭 (ENABLE_METRICS=true 환경변수가 설정된 경우에만 활성화, AdminRequired 적용)
	if os.Getenv("ENABLE_METRICS") == "true" {
		e.GET("/metrics", handler.MetricsHandler(), middleware.AdminRequired())
	}

	// 인증 엔드포인트 (미들웨어에서 스킵됨, 별도 rate limit 적용)
	e.POST("/auth/register", handler.RegisterHandler, middleware.AuthRateLimit())
	e.POST("/auth/login", handler.LoginHandler, middleware.AuthRateLimit())
	e.POST("/auth/logout", handler.LogoutHandler, middleware.AuthRateLimit())
	e.POST("/auth/refresh", handler.RefreshHandler, middleware.AuthRateLimit())
	e.GET("/auth/me", handler.MeHandler)

	// 로그인 페이지
	e.GET("/login", handler.LoginPageRender)

	// 기존 호환 엔드포인트
	e.GET("/client", handler.ChatHTMLRender)
	e.GET("/server", handler.MsgReceiver, middleware.WSConnLimit())

	// 채팅방 REST API
	e.GET("/rooms/default", handler.GetDefaultRoomHandler)
	e.POST("/rooms", handler.CreateRoomHandler, middleware.RoomCreateRateLimit())
	e.GET("/rooms", handler.ListRoomsHandler)
	e.GET("/rooms/:id", handler.GetRoomHandler)
	e.DELETE("/rooms/:id", handler.DeleteRoomHandler)
	e.POST("/rooms/:id/join", handler.JoinRoomHandler)
	e.POST("/rooms/:id/leave", handler.LeaveRoomHandler)
	e.GET("/rooms/:id/ws", handler.RoomWebSocket, middleware.WSConnLimit())
	e.GET("/rooms/:id/messages", handler.GetRoomMessagesHandler)
	e.GET("/rooms/:id/messages/search", handler.SearchMessagesHandler)
	e.PUT("/rooms/:id/messages/:msgId", handler.EditMessageHandler)
	e.DELETE("/rooms/:id/messages/:msgId", handler.DeleteMessageHandler)
	e.POST("/rooms/:id/messages/:msgId/reply", handler.ReplyMessageHandler)
	e.POST("/rooms/:id/upload", handler.UploadFileHandler)
	e.GET("/files/:fileId", handler.ServeFileHandler)

	// 유저 프로필 API
	e.GET("/users/:username/profile", handler.GetProfileHandler)
	e.PUT("/users/me/profile", handler.UpdateProfileHandler)

	// E2E 암호화 키 관리
	e.PUT("/crypto/keys", handler.UploadPublicKeyHandler)
	e.GET("/crypto/keys/:username", handler.GetPublicKeyHandler)
	e.GET("/rooms/:id/keys", handler.GetRoomKeysHandler)

	// 관리자 API
	admin := e.Group("/admin", middleware.AdminRequired())
	admin.GET("", handler.AdminDashboardPage)
	admin.GET("/stats", handler.AdminStatsHandler)
	admin.GET("/users", handler.AdminUsersHandler)
	admin.PUT("/users/:username/block", handler.AdminBlockUserHandler)
	admin.GET("/rooms", handler.AdminRoomsHandler)
	admin.DELETE("/rooms/:id", handler.AdminDeleteRoomHandler)
	admin.POST("/rooms/:id/announce", handler.AdminAnnounceHandler)

	// SPA fallback: serve frontend/dist/ if it exists
	if _, err := os.Stat("frontend/dist"); err == nil {
		e.Static("/assets", "frontend/dist/assets")
		e.File("/", "frontend/dist/index.html")
		e.GET("/*", func(c echo.Context) error {
			return c.File("frontend/dist/index.html")
		})
	}
}
