package db

import (
	"context"
	"fmt"
	"time"

	"github.com/woonglife62/woongkie-talkie/pkg/config"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var DB *mongo.Database
var Client *mongo.Client

// Initialize connects to MongoDB and sets up the database.
// Returns an error instead of silently failing.
func Initialize() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// MongoDB 클라이언트 생성
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.DBConfig.URI).SetAuth(options.Credential{
		Username: config.DBConfig.User,
		Password: config.DBConfig.Password,
	}).SetMaxPoolSize(50).SetMinPoolSize(5).SetMaxConnIdleTime(30*time.Second).SetServerSelectionTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("MongoDB connect error: %w", err)
	}

	// Ping
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		return fmt.Errorf("MongoDB ping failed: %w", err)
	}

	Client = client

	// MongoDB DB 선택 설정
	DB = client.Database(config.DBConfig.Database)
	return nil
}

func Disconnect() {
	if Client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		Client.Disconnect(ctx)
	}
}
