package mongodb

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ChatMessage is used for WebSocket message exchange (keep json tags as-is for compatibility)
type ChatMessage struct {
	Event   string `json:"Event,omitempty"`
	User    string `json:"User"`
	Message string `json:"message"`
	Owner   bool   `json:"owner,omitempty"`
	RoomID  string `json:"room_id,omitempty"`
}

// Chat is the MongoDB storage structure (flat, snake_case)
type Chat struct {
	CreatedAt time.Time `json:"created_at" bson:"created_at,omitempty"`
	RoomID    string    `json:"room_id" bson:"room_id,omitempty"`
	Event     string    `json:"event,omitempty" bson:"event,omitempty"`
	User      string    `json:"user" bson:"user"`
	Message   string    `json:"message" bson:"message"`
	Owner     bool      `json:"owner,omitempty" bson:"owner,omitempty"`
}

var chatCollection *mongo.Collection

// InitChatCollection initializes the chat collection with indexes and runs migrations.
func InitChatCollection(database *mongo.Database) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	collection := "chats"
	database.CreateCollection(ctx, collection)
	chatCollection = database.Collection(collection)

	// 복합 인덱스: room_id ASC, created_at DESC
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "room_id", Value: 1},
			{Key: "created_at", Value: -1},
		},
	}
	_, err := chatCollection.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		return err
	}

	// Migrate old schema documents to flat structure
	if err := migrateChatSchema(ctx); err != nil {
		return err
	}

	return nil
}

// migrateChatSchema flattens existing documents from old nested schema to flat schema.
func migrateChatSchema(ctx context.Context) error {
	// Check if old schema documents exist
	oldFilter := bson.D{{Key: "Date Time", Value: bson.D{{Key: "$exists", Value: true}}}}
	count, err := chatCollection.CountDocuments(ctx, oldFilter)
	if err != nil || count == 0 {
		return err
	}

	cursor, err := chatCollection.Find(ctx, oldFilter)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			continue
		}
		id := raw["_id"]

		update := bson.M{"$set": bson.M{}, "$unset": bson.M{}}
		sets := update["$set"].(bson.M)
		unsets := update["$unset"].(bson.M)

		// Rename "Date Time" to "created_at"
		if dt, ok := raw["Date Time"]; ok {
			sets["created_at"] = dt
			unsets["Date Time"] = ""
		}

		// Flatten "Chat Message" fields
		if cm, ok := raw["Chat Message"].(bson.M); ok {
			for k, v := range cm {
				key := strings.ToLower(k)
				if k == "User" {
					key = "user"
				}
				if k == "Event" {
					key = "event"
				}
				sets[key] = v
			}
			unsets["Chat Message"] = ""
		}

		if len(sets) > 0 {
			chatCollection.UpdateOne(ctx, bson.M{"_id": id}, update)
		}
	}
	return nil
}

// chat message 저장 (room_id 포함)
func InsertChat(chatMessage ChatMessage) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	chat := Chat{
		CreatedAt: time.Now(),
		RoomID:    chatMessage.RoomID,
		Event:     chatMessage.Event,
		User:      chatMessage.User,
		Message:   chatMessage.Message,
		Owner:     chatMessage.Owner,
	}

	_, err := chatCollection.InsertOne(ctx, chat)
	return err
}

// room별 chat 내용 가져오기 (최근 100건)
func FindChatByRoom(roomID string) (chat []Chat, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	chat = []Chat{}

	filter := bson.D{{Key: "room_id", Value: roomID}}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(100)

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
		{Key: "created_at", Value: bson.D{{Key: "$lt", Value: before}}},
	}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(limit)

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
		{Key: "created_at", Value: bson.D{{Key: "$gt", Value: after}}},
	}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}).SetLimit(200)

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
