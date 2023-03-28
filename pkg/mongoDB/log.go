package mongodb

import (
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

	// log 컬렉션 만들기
	collection := "log"
	db.DB.CreateCollection(ctx, collection)
	logCollection = db.DB.Collection(collection)
}

// log 저장
func InsertLog(logMessage LogMessage) (err error) {
	log := Log{
		DateTime:   time.Now(),
		LogMessage: logMessage,
	}

	doc, err := toDoc(log)
	if err != nil {
		return err
	}

	logCollection.InsertOne(ctx, doc)
	if err != nil {
		return err
	}

	return nil
}
