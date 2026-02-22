package mongodb

import "go.mongodb.org/mongo-driver/mongo"

// InitAll initializes all MongoDB collections. Call this after db.Initialize().
func InitAll(database *mongo.Database) error {
	if database == nil {
		return nil
	}
	if err := InitChatCollection(database); err != nil {
		return err
	}
	if err := InitLogCollection(database); err != nil {
		return err
	}
	if err := InitUserCollection(database); err != nil {
		return err
	}
	if err := InitRoomCollection(database); err != nil {
		return err
	}
	if err := InitFileCollection(database); err != nil {
		return err
	}
	return nil
}
