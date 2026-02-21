package mongodb

import (
	"context"
	"errors"
	"time"

	"github.com/woonglife62/woongkie-talkie/pkg/config/db"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

type Room struct {
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Name        string             `json:"name" bson:"name"`
	Description string             `json:"description" bson:"description"`
	IsPublic    bool               `json:"is_public" bson:"is_public"`
	Password    string             `json:"-" bson:"password,omitempty"`
	MaxMembers  int                `json:"max_members" bson:"max_members"`
	CreatedBy   string             `json:"created_by" bson:"created_by"`
	CreatedAt   time.Time          `json:"created_at" bson:"created_at"`
	IsDefault   bool               `json:"is_default" bson:"is_default"`
	Members     []string           `json:"members" bson:"members"`
}

var roomCollection *mongo.Collection

func init() {
	if db.DB == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	collection := "rooms"
	db.DB.CreateCollection(ctx, collection)
	roomCollection = db.DB.Collection(collection)

	// name 필드에 유니크 인덱스 생성
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "name", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	roomCollection.Indexes().CreateOne(ctx, indexModel)

	// is_public 인덱스 생성
	roomCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "is_public", Value: 1}},
	})

	// is_default 인덱스 생성
	roomCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "is_default", Value: 1}},
	})

	// 기본 "general" 채팅방 생성
	ensureDefaultRoom(ctx)

	// 기존 메시지를 general 방으로 마이그레이션
	defaultRoom, err := FindDefaultRoom()
	if err == nil {
		MigrateChatsToRoom(defaultRoom.ID.Hex())
	}
}

func ensureDefaultRoom(ctx context.Context) {
	filter := bson.D{{Key: "is_default", Value: true}}
	var existing Room
	err := roomCollection.FindOne(ctx, filter).Decode(&existing)
	if err == mongo.ErrNoDocuments {
		room := Room{
			Name:        "general",
			Description: "기본 채팅방",
			IsPublic:    true,
			MaxMembers:  0,
			CreatedBy:   "system",
			CreatedAt:   time.Now(),
			IsDefault:   true,
			Members:     []string{},
		}
		roomCollection.InsertOne(ctx, room)
	}
}

func CreateRoom(room Room) (*Room, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	room.CreatedAt = time.Now()
	if room.Members == nil {
		room.Members = []string{}
	}
	result, err := roomCollection.InsertOne(ctx, room)
	if err != nil {
		return nil, err
	}
	room.ID = result.InsertedID.(primitive.ObjectID)
	return &room, nil
}

func FindRooms() ([]Room, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.D{{Key: "is_public", Value: true}}
	cur, err := roomCollection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var rooms []Room
	for cur.Next(ctx) {
		var room Room
		if err := cur.Decode(&room); err != nil {
			return nil, err
		}
		rooms = append(rooms, room)
	}
	return rooms, nil
}

func FindRoomByID(id string) (*Room, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	filter := bson.D{{Key: "_id", Value: objID}}
	var room Room
	err = roomCollection.FindOne(ctx, filter).Decode(&room)
	if err != nil {
		return nil, err
	}
	return &room, nil
}

func FindDefaultRoom() (*Room, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.D{{Key: "is_default", Value: true}}
	var room Room
	err := roomCollection.FindOne(ctx, filter).Decode(&room)
	if err != nil {
		return nil, err
	}
	return &room, nil
}

func DeleteRoom(id string, username string) error {
	room, err := FindRoomByID(id)
	if err != nil {
		return err
	}
	if room.IsDefault {
		return errors.New("기본 채팅방은 삭제할 수 없습니다")
	}
	if room.CreatedBy != username {
		return errors.New("채팅방 생성자만 삭제할 수 있습니다")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objID, _ := primitive.ObjectIDFromHex(id)
	_, err = roomCollection.DeleteOne(ctx, bson.D{{Key: "_id", Value: objID}})
	return err
}

func JoinRoom(roomID string, username string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(roomID)
	if err != nil {
		return err
	}

	// 이미 멤버인 경우 필터: 멤버에 포함되어 있으면 바로 성공
	filterAlready := bson.D{
		{Key: "_id", Value: objID},
		{Key: "members", Value: username},
	}
	count, err := roomCollection.CountDocuments(ctx, filterAlready)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	// 원자적 업데이트: max_members == 0 (무제한) 이거나 현재 멤버 수가 max_members 미만일 때만 추가
	filter := bson.D{
		{Key: "_id", Value: objID},
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "max_members", Value: 0}},
			bson.D{{Key: "$expr", Value: bson.D{
				{Key: "$lt", Value: bson.A{
					bson.D{{Key: "$size", Value: "$members"}},
					"$max_members",
				}},
			}}},
		}},
	}
	update := bson.D{{Key: "$addToSet", Value: bson.D{{Key: "members", Value: username}}}}

	result, err := roomCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return errors.New("채팅방 인원이 가득 찼습니다")
	}
	return nil
}

// HashRoomPassword hashes a plaintext room password using bcrypt.
func HashRoomPassword(password string) (string, error) {
	if password == "" {
		return "", nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckRoomPassword compares a plaintext password against a bcrypt hash.
func CheckRoomPassword(room *Room, password string) bool {
	if room.Password == "" {
		return true
	}
	return bcrypt.CompareHashAndPassword([]byte(room.Password), []byte(password)) == nil
}

func LeaveRoom(roomID string, username string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(roomID)
	if err != nil {
		return err
	}
	_, err = roomCollection.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: objID}},
		bson.D{{Key: "$pull", Value: bson.D{{Key: "members", Value: username}}}},
	)
	return err
}
