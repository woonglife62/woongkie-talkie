package mongodb

import (
	"context"
	"time"

	"github.com/woonglife62/woongkie-talkie/pkg/config/db"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ChatMessage struct {
	Event   string `json:"Event,omitempty" bson:"Event,omitempty"`
	User    string `json:"User" bson:"User"`
	Message string `json:"message" bson:"message"`
	Owner   bool   `json:"owner,omitempty" bson:"owner,omitempty"`
	RoomID  string `json:"room_id,omitempty" bson:"-"`
}

type Chat struct {
	DateTime    time.Time `bson:"Date Time,omitempty"`
	RoomID      string    `bson:"room_id,omitempty"`
	ChatMessage `bson:"Chat Message,omitempty"`
}

var chatCollection *mongo.Collection

func init() {
	if db.DB == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	collection := "chats"
	db.DB.CreateCollection(ctx, collection)
	chatCollection = db.DB.Collection(collection)

	// 복합 인덱스: room_id ASC, Date Time DESC
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "room_id", Value: 1},
			{Key: "Date Time", Value: -1},
		},
	}
	chatCollection.Indexes().CreateOne(ctx, indexModel)
}

// chat message 저장 (room_id 포함)
func InsertChat(chatMessage ChatMessage) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	chat := Chat{
		DateTime:    time.Now(),
		RoomID:      chatMessage.RoomID,
		ChatMessage: chatMessage,
	}

	_, err = chatCollection.InsertOne(ctx, chat)
	if err != nil {
		return err
	}

	return nil
}

// room별 chat 내용 가져오기 (최근 100건)
func FindChatByRoom(roomID string) (chat []Chat, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	chat = []Chat{}

	filter := bson.D{{Key: "room_id", Value: roomID}}
	opts := options.Find().SetSort(bson.D{{Key: "Date Time", Value: -1}}).SetLimit(100)

	cur, err := chatCollection.Find(ctx, filter, opts)
	if err != nil {
		return chat, err
	}
	defer cur.Close(ctx)

	for cur.Next(ctx) {
		var result Chat
		err := cur.Decode(&result)
		if err != nil {
			return chat, err
		}
		chat = append(chat, result)
	}

	// 시간순 정렬 (역순 -> 정순)
	for i, j := 0, len(chat)-1; i < j; i, j = i+1, j-1 {
		chat[i], chat[j] = chat[j], chat[i]
	}

	return chat, nil
}

// 이전 메시지 페이징 조회
func FindChatByRoomBefore(roomID string, before time.Time, limit int64) (chat []Chat, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	chat = []Chat{}

	filter := bson.D{
		{Key: "room_id", Value: roomID},
		{Key: "Date Time", Value: bson.D{{Key: "$lt", Value: before}}},
	}
	opts := options.Find().SetSort(bson.D{{Key: "Date Time", Value: -1}}).SetLimit(limit)

	cur, err := chatCollection.Find(ctx, filter, opts)
	if err != nil {
		return chat, err
	}
	defer cur.Close(ctx)

	for cur.Next(ctx) {
		var result Chat
		err := cur.Decode(&result)
		if err != nil {
			return chat, err
		}
		chat = append(chat, result)
	}

	for i, j := 0, len(chat)-1; i < j; i, j = i+1, j-1 {
		chat[i], chat[j] = chat[j], chat[i]
	}

	return chat, nil
}

// 재연결 시 놓친 메시지 조회
func FindChatByRoomAfter(roomID string, after time.Time) ([]Chat, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.D{
		{Key: "room_id", Value: roomID},
		{Key: "Date Time", Value: bson.D{{Key: "$gt", Value: after}}},
	}
	opts := options.Find().SetSort(bson.D{{Key: "Date Time", Value: 1}}).SetLimit(200)

	cur, err := chatCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var chats []Chat
	for cur.Next(ctx) {
		var result Chat
		if err := cur.Decode(&result); err != nil {
			return nil, err
		}
		chats = append(chats, result)
	}
	if chats == nil {
		chats = []Chat{}
	}
	return chats, nil
}

// 기존 호환 - 모든 chat 가져오기
func FindChat() (chat []Chat, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	chat = []Chat{}

	cur, err := chatCollection.Find(ctx, bson.D{})
	if err != nil {
		return chat, err
	}
	defer cur.Close(ctx)

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

// 기존 메시지를 특정 room으로 마이그레이션
func MigrateChatsToRoom(roomID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.D{
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "room_id", Value: ""}},
			bson.D{{Key: "room_id", Value: bson.D{{Key: "$exists", Value: false}}}},
		}},
	}
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "room_id", Value: roomID}}}}
	_, err := chatCollection.UpdateMany(ctx, filter, update)
	return err
}
