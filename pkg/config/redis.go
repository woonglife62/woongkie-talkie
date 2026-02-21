package config

import (
	"github.com/jinzhu/configor"
	"github.com/labstack/gommon/log"
)

type redisConfig struct {
	Addr     string `env:"REDIS_ADDR" default:"localhost:6379"`
	Password string `env:"REDIS_PASSWORD" default:""`
	DB       int    `env:"REDIS_DB" default:"0"`
}

var RedisConfig = redisConfig{}

func init() {
	if err := configor.Load(&RedisConfig); err != nil {
		log.Panic(err)
	}
}
