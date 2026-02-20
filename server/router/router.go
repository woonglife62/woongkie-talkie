package router

import (
	"os"

	"github.com/labstack/echo/v4"
	"github.com/woonglife62/woongkie-talkie/server/handler"
	"github.com/woonglife62/woongkie-talkie/server/middleware"
)

func Router(e *echo.Echo) {

	e.Static("/", os.ExpandEnv("$GOPATH/src/woongkie-talkie"))

	middleware.Middleware(e)

	// 인증 엔드포인트 (미들웨어에서 스킵됨)
	e.POST("/auth/register", handler.RegisterHandler)
	e.POST("/auth/login", handler.LoginHandler)
	e.GET("/auth/me", handler.MeHandler)

	// 로그인 페이지
	e.GET("/login", handler.LoginPageRender)

	// 기존 호환 엔드포인트
	e.GET("/client", handler.ChatHTMLRender)
	e.GET("/server", handler.MsgReceiver)

	// 채팅방 REST API
	e.GET("/rooms/default", handler.GetDefaultRoomHandler)
	e.POST("/rooms", handler.CreateRoomHandler)
	e.GET("/rooms", handler.ListRoomsHandler)
	e.GET("/rooms/:id", handler.GetRoomHandler)
	e.DELETE("/rooms/:id", handler.DeleteRoomHandler)
	e.POST("/rooms/:id/join", handler.JoinRoomHandler)
	e.POST("/rooms/:id/leave", handler.LeaveRoomHandler)
	e.GET("/rooms/:id/ws", handler.RoomWebSocket)
	e.GET("/rooms/:id/messages", handler.GetRoomMessagesHandler)
}
