package config

import (
	"fmt"

	"github.com/jinzhu/configor"
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

// init is intentionally left empty. Redis configuration is loaded exclusively
// via loadRedis() called from LoadAll(), avoiding double-initialization (#99).
func init() {}
