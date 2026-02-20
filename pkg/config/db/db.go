package db

import (
	"context"
	"log"
	"time"

	"github.com/woonglife62/woongkie-talkie/pkg/config"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var DB *mongo.Database
var Client *mongo.Client

/*
make DB Connector
*/
func init() {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// MongoDB 클라이언트 생성
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.DBConfig.URI).SetAuth(options.Credential{
		Username: config.DBConfig.User,
		Password: config.DBConfig.Password,
	}))
	if err != nil {
		log.Printf("warning: MongoDB connect error: %v", err)
		return
	}
	// defer client.Disconnect(ctx)

	// Ping
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		log.Printf("warning: MongoDB ping failed: %v", err)
		return
	}

	Client = client

	// MongoDB DB 선택 설정
	DB = client.Database(config.DBConfig.Database)
}

func Disconnect() {
	if Client != nil {
		Client.Disconnect(context.Background())
	}
}
