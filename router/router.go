package router

import (
	"github.com/gorilla/websocket"

	"github.com/labstack/echo/v4"
)

var (
	upgrader  = websocket.Upgrader{}             // 웹소캣을 생성함.
	clients   = make(map[*websocket.Conn]string) // 접속중인 client 관리
	broadcast = make(chan Message)               // channel 로 개방된 소캣에 전달 할 message
)

type Message struct {
	Event   string `json:"Event"` // entrance or message
	User    string `json:"User"`
	Message string `json:"message"`
	Owner   bool   `json:"owner"`
}

func Router(e *echo.Echo) {
	clientRouter(e)
	serverRouter(e)
}
