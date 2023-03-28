package mongodb

import (
	"time"

	"github.com/woonglife62/woongkie-talkie/pkg/config/db"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type ChatMessage struct {
	User    string `bson:"User,omitempty"`
	Message string `bson:"Message,omitempty"`
}

type Chat struct {
	DateTime    time.Time `bson:"Date Time,omitempty"`
	ChatMessage `bson:"Chat Message,omitempty"`
}

var chatCollection *mongo.Collection

func init() {
	// chat 컬렉션 만들기
	collection := "chat"
	db.DB.CreateCollection(ctx, collection)
	chatCollection = db.DB.Collection(collection)
}

// chat message 저장
func InsertChat(chatMessage ChatMessage) (err error) {

	chat := Chat{
		DateTime:    time.Now(),
		ChatMessage: chatMessage,
	}

	doc, err := toDoc(chat)
	if err != nil {
		return err
	}

	chatCollection.InsertOne(ctx, doc)
	if err != nil {
		return err
	}

	return nil
}

// chat 내용 전체 가져오기
func FindChat() (chat []Chat, err error) {

	chat = []Chat{}

	cur, err := chatCollection.Find(ctx, bson.D{})
	if err != nil {
		return chat, err
	}

	for cur.Next(ctx) {
		var result Chat
		err := cur.Decode(&result)
		if err != nil {
			return chat, err
		}
		chat = append(chat, result)
	}

	return chat, nil
}
