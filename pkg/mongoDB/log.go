package mongodb

import (
	"context"
	"time"

	"github.com/woonglife62/woongkie-talkie/pkg/config/db"
	"go.mongodb.org/mongo-driver/mongo"
)

type LogMessage struct {
	Level   string `bson:"Level,omitempty"`
	Message string `bson:"Message,omitempty"`
}

type Log struct {
	DateTime   time.Time `bson:"Date Time,omitempty"`
	LogMessage `bson:"Log Message,omitempty"`
}

var logCollection *mongo.Collection

func init() {
	if db.DB == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// log 컬렉션 만들기
	collection := "log"
	db.DB.CreateCollection(ctx, collection)
	logCollection = db.DB.Collection(collection)
}

// log 저장
func InsertLog(logMessage LogMessage) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log := Log{
		DateTime:   time.Now(),
		LogMessage: logMessage,
	}

	_, err = logCollection.InsertOne(ctx, log)
	if err != nil {
		return err
	}

	return nil
}
