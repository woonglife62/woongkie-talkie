package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type LogMessage struct {
	Level   string `bson:"level"`
	Message string `bson:"message"`
}

// Log is the MongoDB storage structure (flat, snake_case)
type Log struct {
	CreatedAt time.Time `bson:"created_at,omitempty"`
	Level     string    `bson:"level,omitempty"`
	Message   string    `bson:"message,omitempty"`
}

var logCollection *mongo.Collection

// InitLogCollection initializes the log collection and runs migrations.
func InitLogCollection(database *mongo.Database) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	collection := "log"
	database.CreateCollection(ctx, collection)
	logCollection = database.Collection(collection)

	// Migrate old schema documents to flat structure
	if err := migrateLogSchema(ctx); err != nil {
		return err
	}

	return nil
}

// migrateLogSchema flattens existing documents from old nested schema to flat schema.
func migrateLogSchema(ctx context.Context) error {
	// Check if old schema documents exist
	oldFilter := bson.D{{Key: "Date Time", Value: bson.D{{Key: "$exists", Value: true}}}}
	count, err := logCollection.CountDocuments(ctx, oldFilter)
	if err != nil || count == 0 {
		return err
	}

	cursor, err := logCollection.Find(ctx, oldFilter)
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

		// Flatten "Log Message" fields
		if lm, ok := raw["Log Message"].(bson.M); ok {
			for k, v := range lm {
				key := k
				if k == "Level" {
					key = "level"
				}
				if k == "Message" {
					key = "message"
				}
				sets[key] = v
			}
			unsets["Log Message"] = ""
		}

		if len(sets) > 0 {
			logCollection.UpdateOne(ctx, bson.M{"_id": id}, update)
		}
	}
	return nil
}

// log 저장
func InsertLog(logMessage LogMessage) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log := Log{
		CreatedAt: time.Now(),
		Level:     logMessage.Level,
		Message:   logMessage.Message,
	}

	_, err = logCollection.InsertOne(ctx, log)
	if err != nil {
		return err
	}

	return nil
}
