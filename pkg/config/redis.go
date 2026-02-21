package config

import (
	"fmt"

	"github.com/jinzhu/configor"
	"github.com/labstack/gommon/log"
)

type redisConfig struct {
	Addr     string `env:"REDIS_ADDR" default:"localhost:6379"`
	Password string `env:"REDIS_PASSWORD" default:""`
	DB       int    `env:"REDIS_DB" default:"0"`
}

var RedisConfig = redisConfig{}

// loadRedis loads Redis configuration. Called from LoadAll().
func loadRedis() error {
	if err := configor.Load(&RedisConfig); err != nil {
		return fmt.Errorf("redis configor load: %w", err)
	}
	return nil
}

func init() {
	if err := configor.Load(&RedisConfig); err != nil {
		log.Panic(err)
	}
}
