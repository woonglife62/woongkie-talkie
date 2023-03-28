package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
)

// type Log struct {
// 	gorm.Model
// 	Level     string `gorm:"not null"`
// 	Message   string `gorm:"not null"`
// 	Timestamp int64  `gorm:"not null"`
// }

// // 로그 출력 설정
// config := zap.NewProductionConfig()
// config.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
// logger, err := config.Build()
// if err != nil {
// 	panic(err)
// }
// defer logger.Sync()
// log := logger.Sugar()

// // 로그 작성 및 저장
// level := zapcore.InfoLevel
// message := "Sample log message"
// timestamp := zap.Int64("timestamp", zap.Now().UnixNano())

// log.Infow(message, timestamp)

// db.Create(&Log{
// 	Level:     level.String(),
// 	Message:   message,
// 	Timestamp: timestamp,
// })

var ctx context.Context = context.Background()

// https://stackoverflow.com/questions/53110020/mongodb-go-driver-bson-struct-to-bson-document-encoding
func toDoc(v interface{}) (doc *bson.D, err error) {
	data, err := bson.Marshal(v)
	if err != nil {
		return
	}

	err = bson.Unmarshal(data, &doc)
	return
}
