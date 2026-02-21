package mongodb

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ChatMessage is used for WebSocket message exchange (keep json tags as-is for compatibility)
type ChatMessage struct {
	Event     string `json:"Event,omitempty"`
	User      string `json:"User"`
	Message   string `json:"message"`
	Owner     bool   `json:"owner,omitempty"`
	RoomID    string `json:"room_id,omitempty"`
	MessageID string `json:"message_id,omitempty"`
	ReplyTo   string `json:"reply_to,omitempty"`
}

// Chat is the MongoDB storage structure (flat, snake_case)
type Chat struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	CreatedAt time.Time          `json:"created_at" bson:"created_at,omitempty"`
	EditedAt  *time.Time         `json:"edited_at,omitempty" bson:"edited_at,omitempty"`
	RoomID    string             `json:"room_id" bson:"room_id,omitempty"`
	Event     string             `json:"event,omitempty" bson:"event,omitempty"`
	User      string             `json:"user" bson:"user"`
	Message   string             `json:"message" bson:"message"`
	Owner     bool               `json:"owner,omitempty" bson:"owner,omitempty"`
	ReplyTo   string             `json:"reply_to,omitempty" bson:"reply_to,omitempty"`
	IsDeleted bool               `json:"is_deleted,omitempty" bson:"is_deleted,omitempty"`
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

// InsertManyChat bulk-inserts a slice of ChatMessages into MongoDB.
// Returns the count of inserted documents and any error.
func InsertManyChat(messages []ChatMessage) (int, error) {
	if len(messages) == 0 {
		return 0, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	docs := make([]interface{}, len(messages))
	for i, msg := range messages {
		docs[i] = Chat{
			CreatedAt: time.Now(),
			RoomID:    msg.RoomID,
			Event:     msg.Event,
			User:      msg.User,
			Message:   msg.Message,
			Owner:     msg.Owner,
		}
	}

	result, err := chatCollection.InsertMany(ctx, docs)
	if err != nil {
		return 0, err
	}
	return len(result.InsertedIDs), nil
}

// chat message 저장 (room_id 포함)
// Returns the inserted document ID as a hex string and any error.
func InsertChat(chatMessage ChatMessage) (string, error) {
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

	result, err := chatCollection.InsertOne(ctx, chat)
	if err != nil {
		return "", err
	}
	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		return oid.Hex(), nil
	}
	return "", nil
}

// room별 chat 내용 가져오기 (최근 100건, 시간 오름차순)
func FindChatByRoom(roomID string) (chat []Chat, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	chat = []Chat{}

	filter := bson.D{
		{Key: "room_id", Value: roomID},
		{Key: "is_deleted", Value: bson.D{{Key: "$ne", Value: true}}},
	}
	// Sort ascending by _id (ObjectID embeds timestamp) to get chronological order efficiently.
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(100)

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

	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(500)
	cur, err := chatCollection.Find(ctx, bson.D{}, opts)
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

// FindChatByID finds a single chat message by its ObjectID hex string.
func FindChatByID(messageID string) (*Chat, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	oid, err := primitive.ObjectIDFromHex(messageID)
	if err != nil {
		return nil, err
	}

	var chat Chat
	err = chatCollection.FindOne(ctx, bson.D{{Key: "_id", Value: oid}}).Decode(&chat)
	if err != nil {
		return nil, err
	}
	return &chat, nil
}

// EditChat updates the message text for the given message ID, only if the requesting
// user is the owner and the message is within the 5-minute edit window.
func EditChat(messageID, username, newMessage string) (*Chat, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	oid, err := primitive.ObjectIDFromHex(messageID)
	if err != nil {
		return nil, err
	}

	var chat Chat
	err = chatCollection.FindOne(ctx, bson.D{{Key: "_id", Value: oid}}).Decode(&chat)
	if err != nil {
		return nil, err
	}

	if chat.User != username {
		return nil, ErrForbidden
	}
	if time.Since(chat.CreatedAt) > 5*time.Minute {
		return nil, ErrEditWindowExpired
	}
	if chat.IsDeleted {
		return nil, ErrMessageDeleted
	}

	now := time.Now()
	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "message", Value: newMessage},
		{Key: "edited_at", Value: now},
	}}}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updated Chat
	err = chatCollection.FindOneAndUpdate(ctx, bson.D{{Key: "_id", Value: oid}}, update, opts).Decode(&updated)
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

// DeleteChat soft-deletes a message by setting is_deleted=true and clearing message content.
func DeleteChat(messageID, username string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	oid, err := primitive.ObjectIDFromHex(messageID)
	if err != nil {
		return err
	}

	var chat Chat
	err = chatCollection.FindOne(ctx, bson.D{{Key: "_id", Value: oid}}).Decode(&chat)
	if err != nil {
		return err
	}

	if chat.User != username {
		return ErrForbidden
	}

	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "is_deleted", Value: true},
		{Key: "message", Value: ""},
	}}}
	_, err = chatCollection.UpdateOne(ctx, bson.D{{Key: "_id", Value: oid}}, update)
	return err
}

// InsertChatWithReply saves a new chat message that replies to another message.
func InsertChatWithReply(chatMessage ChatMessage) (*Chat, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	chat := Chat{
		CreatedAt: time.Now(),
		RoomID:    chatMessage.RoomID,
		Event:     chatMessage.Event,
		User:      chatMessage.User,
		Message:   chatMessage.Message,
		Owner:     chatMessage.Owner,
		ReplyTo:   chatMessage.ReplyTo,
	}

	result, err := chatCollection.InsertOne(ctx, chat)
	if err != nil {
		return nil, err
	}
	chat.ID = result.InsertedID.(primitive.ObjectID)
	return &chat, nil
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
