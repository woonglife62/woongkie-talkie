package router

import (
	"os"

	"github.com/labstack/echo/v4"
	"github.com/woonglife62/woongkie-talkie/server/handler"
	"github.com/woonglife62/woongkie-talkie/server/middleware"
)

func Router(e *echo.Echo) {

	e.Static("/", os.ExpandEnv("$GOPATH/src/woongkie-talkie")) // echo root Path

	middleware.Middleware(e)

	e.GET("/client", handler.ChatHTMLRender)

	e.GET("/server", handler.MsgReceiver)
}
