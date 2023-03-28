package db

import (
	"context"

	"github.com/woonglife62/woongkie-talkie/pkg/config"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var DB *mongo.Database

/*
make DB Connector
*/
func init() {

	// ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	ctx := context.Background()

	// MongoDB 클라이언트 생성
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.DBConfig.URI).SetAuth(options.Credential{
		Username: config.DBConfig.User,
		Password: config.DBConfig.Password,
	}))
	if err != nil {
		panic(err)
	}
	// defer client.Disconnect(ctx)

	// Ping
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		panic(err)
	}

	// MongoDB DB 선택 설정
	DB = client.Database(config.DBConfig.Database)
}
