package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// FileMetadata stores metadata for uploaded files.
type FileMetadata struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Filename  string             `json:"filename" bson:"filename"`
	MimeType  string             `json:"mime_type" bson:"mime_type"`
	Size      int64              `json:"size" bson:"size"`
	Path      string             `json:"-" bson:"path"`
	URL       string             `json:"url" bson:"url"`
	RoomID    string             `json:"room_id" bson:"room_id"`
	Uploader  string             `json:"uploader" bson:"uploader"`
	CreatedAt time.Time          `json:"created_at" bson:"created_at"`
}

var fileCollection *mongo.Collection

// InitFileCollection initializes the file_metadata collection with an index on room_id.
func InitFileCollection(database *mongo.Database) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	collection := "file_metadata"
	database.CreateCollection(ctx, collection)
	fileCollection = database.Collection(collection)

	indexModel := mongo.IndexModel{
		Keys: bson.D{{Key: "room_id", Value: 1}},
	}
	_, err := fileCollection.Indexes().CreateOne(ctx, indexModel)
	return err
}

// InsertFileMetadata inserts a FileMetadata document and returns the saved record.
func InsertFileMetadata(meta FileMetadata) (*FileMetadata, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	meta.CreatedAt = time.Now()
	result, err := fileCollection.InsertOne(ctx, meta)
	if err != nil {
		return nil, err
	}
	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		meta.ID = oid
	}
	return &meta, nil
}

// FindFileByID retrieves a FileMetadata document by its ObjectID hex string.
func FindFileByID(fileID string) (*FileMetadata, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	oid, err := primitive.ObjectIDFromHex(fileID)
	if err != nil {
		return nil, err
	}

	var meta FileMetadata
	err = fileCollection.FindOne(ctx, bson.D{{Key: "_id", Value: oid}}).Decode(&meta)
	if err != nil {
		return nil, err
	}
	return &meta, nil
}
