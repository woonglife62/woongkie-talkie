package mongodb

import (
	"fmt"
	"strconv"
	"testing"
)

func TestXxx(t *testing.T) {
	tmp := LogMessage{
		Level:   "Debug",
		Message: "test123",
	}
	err := InsertLog(tmp)
	fmt.Println(err)
}

func TestDeleteCollection(t *testing.T) {
	for i := 10; i < 20; i++ {
		tmp := ChatMessage{
			User:    "wclee",
			Message: strconv.Itoa(i),
		}

		InsertChat(tmp)
	}
}

func TestFindChat(t *testing.T) {

	chat, err := FindChat()
	fmt.Println(err)
	for _, c := range chat {
		fmt.Println(c)
	}

}
