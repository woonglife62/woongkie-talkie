package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Username     string             `json:"username" bson:"username"`
	PasswordHash string             `json:"-" bson:"password_hash"`
	DisplayName  string             `json:"display_name" bson:"display_name"`
	AvatarURL     string             `json:"avatar_url" bson:"avatar_url"`
	StatusMessage string             `json:"status_message" bson:"status_message"`
	CreatedAt    time.Time          `json:"created_at" bson:"created_at"`
}

var userCollection *mongo.Collection

// InitUserCollection initializes the user collection with indexes.
func InitUserCollection(database *mongo.Database) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	collection := "users"
	database.CreateCollection(ctx, collection)
	userCollection = database.Collection(collection)

	// username 필드에 유니크 인덱스 생성
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "username", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	_, err := userCollection.Indexes().CreateOne(ctx, indexModel)
	return err
}

func CreateUser(username, password, displayName string) (*User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := User{
		Username:     username,
		PasswordHash: string(hash),
		DisplayName:  displayName,
		CreatedAt:    time.Now(),
	}

	result, err := userCollection.InsertOne(ctx, user)
	if err != nil {
		return nil, err
	}
	user.ID = result.InsertedID.(primitive.ObjectID)
	return &user, nil
}

func FindUserByUsername(username string) (*User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.D{{Key: "username", Value: username}}
	var user User
	err := userCollection.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func CheckPassword(user *User, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	return err == nil
}

// UpdateUserProfile updates display_name, status_message, and avatar_url for the given username.
func UpdateUserProfile(username, displayName, statusMessage, avatarURL string) (*User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "display_name", Value: displayName},
		{Key: "status_message", Value: statusMessage},
		{Key: "avatar_url", Value: avatarURL},
	}}}
	filter := bson.D{{Key: "username", Value: username}}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var user User
	err := userCollection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
