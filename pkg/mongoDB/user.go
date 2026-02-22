package mongodb

import (
	"context"
	"html"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID            primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Username      string             `json:"username" bson:"username"`
	PasswordHash  string             `json:"-" bson:"password_hash"`
	DisplayName   string             `json:"display_name" bson:"display_name"`
	AvatarURL     string             `json:"avatar_url" bson:"avatar_url"`
	StatusMessage string             `json:"status_message" bson:"status_message"`
	Role          string             `json:"role" bson:"role"`
	PublicKey     string             `json:"public_key,omitempty" bson:"public_key,omitempty"`
	CreatedAt     time.Time          `json:"created_at" bson:"created_at"`
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
		Role:         "user",
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
	if userCollection == nil {
		return nil, ErrNotFound
	}

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

// UpdateUserPublicKey stores the user's public key (JWK JSON string).
func UpdateUserPublicKey(username, publicKey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "public_key", Value: publicKey},
	}}}
	filter := bson.D{{Key: "username", Value: username}}
	_, err := userCollection.UpdateOne(ctx, filter, update)
	return err
}

// GetUserPublicKey returns the public key for a user.
func GetUserPublicKey(username string) (string, error) {
	user, err := FindUserByUsername(username)
	if err != nil {
		return "", err
	}
	return user.PublicKey, nil
}

// GetBatchPublicKeys fetches public keys for multiple usernames in a single query.
// #245: avoids N+1 queries in GetRoomKeysHandler.
func GetBatchPublicKeys(usernames []string) (map[string]string, error) {
	if len(usernames) == 0 {
		return map[string]string{}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.D{{Key: "username", Value: bson.D{{Key: "$in", Value: usernames}}}}
	projection := options.Find().SetProjection(bson.D{
		{Key: "username", Value: 1},
		{Key: "public_key", Value: 1},
	})

	cursor, err := userCollection.Find(ctx, filter, projection)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	keys := make(map[string]string, len(usernames))
	for cursor.Next(ctx) {
		var u User
		if err := cursor.Decode(&u); err == nil && u.PublicKey != "" {
			keys[u.Username] = u.PublicKey
		}
	}
	return keys, cursor.Err()
}

// UpdateUserProfile updates display_name, status_message, and avatar_url for the given username.
func UpdateUserProfile(username, displayName, statusMessage, avatarURL string) (*User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "display_name", Value: html.EscapeString(strings.TrimSpace(displayName))},
		{Key: "status_message", Value: html.EscapeString(strings.TrimSpace(statusMessage))},
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

// FindAllUsers returns paginated user list and total count.
func FindAllUsers(page, limit int64) ([]User, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	total, err := userCollection.CountDocuments(ctx, bson.D{})
	if err != nil {
		return nil, 0, err
	}

	skip := (page - 1) * limit
	opts := options.Find().SetSkip(skip).SetLimit(limit).SetSort(bson.D{{Key: "created_at", Value: -1}})
	cur, err := userCollection.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	var users []User
	for cur.Next(ctx) {
		var u User
		if err := cur.Decode(&u); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	if users == nil {
		users = []User{}
	}
	return users, total, nil
}

// SetUserRole updates the role for a user.
func SetUserRole(username, role string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.D{{Key: "username", Value: username}}
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "role", Value: role}}}}
	_, err := userCollection.UpdateOne(ctx, filter, update)
	return err
}

// BlockUser sets role to "blocked" for a user.
func BlockUser(username string) error {
	return SetUserRole(username, "blocked")
}

// UnblockUser resets role back to "user".
func UnblockUser(username string) error {
	return SetUserRole(username, "user")
}

// CountUsers returns total user count.
func CountUsers() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return userCollection.CountDocuments(ctx, bson.D{})
}
