package handler

import (
	"fmt"
	"log"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
)

var (
	upgrader  = websocket.Upgrader{}             // 웹소캣을 생성함.
	clients   = make(map[*websocket.Conn]string) // 접속중인 client 관리
	broadcast = make(chan mongodb.ChatMessage)   // channel 로 개방된 소캣에 전달 할 message
)

func MsgReceiver(c echo.Context) error {
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer ws.Close()

	clientNm, _, _ := c.Request().BasicAuth()
	clients[ws] = clientNm // client의 접속으로 열린 웹소켓 저장

	chatList, err := mongodb.FindChat()
	if err == nil {
		for _, pastChat := range chatList {
			var tmpMsg mongodb.ChatMessage
			tmpMsg.User = pastChat.User

			tmpMsg.Message = pastChat.Message

			if pastChat.User == clientNm {
				tmpMsg.Owner = true
			} else {
				tmpMsg.Owner = false
			}
			tmpMsg.Event = "CHATLOG"
			ws.WriteJSON(&tmpMsg)
		}
	}

	for {
		var msg mongodb.ChatMessage
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("error1: %v", err)
			delete(clients, ws)
			msg.Event = "CLOSE"
			msg.User = clientNm
			broadcast <- msg
			break
		}
		msg.User = clientNm

		// 입력한 글 mongoDB 에 저장
		if msg.Event != "OPEN" {
			chatMessage := mongodb.ChatMessage{
				User:    msg.User,
				Message: msg.Message,
			}
			err = mongodb.InsertChat(chatMessage)
			if err != nil {
				log.Print(err)
			}
		}

		broadcast <- msg
	}
	return nil
}

func msgDeliverer() {
	for {
		msg := <-broadcast
		var msgFulltxt string = msg.Message

		for ws := range clients {
			msg.Message = msgFulltxt
			if clients[ws] == msg.User {
				msg.Owner = true
			} else {
				msg.Owner = false
			}
			if msg.Event == "OPEN" {
				msg.Message = fmt.Sprintf("---- %s님이 입장하셨습니다. ----", msg.User)
			} else if msg.Event == "CLOSE" {
				msg.Message = fmt.Sprintf("---- %s님이 퇴장하셨습니다. ----", msg.User)
			}
			err := ws.WriteJSON(msg)
			if err != nil {
				log.Printf("error2: %v", err)
				ws.Close()
				delete(clients, ws)
			}
		}
	}
}

func init() {
	go msgDeliverer()
}
