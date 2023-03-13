package router

import (
	"fmt"
	"log"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/woonglife62/woongkie-talkie/redisclient"
)

func msgReceiver(c echo.Context) error {
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer ws.Close()

	clientNm, _, _ := c.Request().BasicAuth()
	clients[ws] = clientNm // client의 접속으로 열린 웹소켓 저장

	reply, err := redisclient.ListAllLrange(c.Path())
	if err == nil {
		for pastMsg := range reply {
			var tmpMsg Message
			pastMsgSlice := strings.Split(reply[pastMsg], ":")
			tmpMsg.User = pastMsgSlice[0]
			// fmt.Println(pastMsgSlice[0], len(pastMsgSlice[0]))
			// fmt.Println(clientNm, len(clientNm))

			tmpMsg.Message = reply[pastMsg][len(pastMsgSlice[0])+1:]

			if pastMsgSlice[0] == clientNm {
				tmpMsg.Owner = true
				//tmpMsg.Message = reply[pastMsg][len(pastMsgSlice[0])+1:]
			} else {
				tmpMsg.Owner = false
				//tmpMsg.Message = pastMsgSlice[0] + " : " + reply[pastMsg][len(pastMsgSlice[0])+1:]
			}
			tmpMsg.Event = "CHATLOG"
			ws.WriteJSON(&tmpMsg)
		}
	}

	for {
		var msg Message
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

		// 입력한 글 redis에 저장
		if msg.Event != "OPEN" {
			_, err = redisclient.ListRpush(c.Path(), fmt.Sprintf("%s:%s", msg.User, msg.Message))
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
				// msg.Message = msg.User + " : " + msgFulltxt
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

func serverRouter(e *echo.Echo) {
	e.GET("/server", msgReceiver)
	go msgDeliverer()
}
