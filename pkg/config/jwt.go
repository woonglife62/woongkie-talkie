package config

import (
	"github.com/jinzhu/configor"
	"github.com/labstack/gommon/log"
	"gopkg.in/go-playground/validator.v9"
)

type jwtConfig struct {
	Secret string `env:"JWT_SECRET" validate:"required"`
	Expiry string `env:"JWT_EXPIRY" default:"24h"`
}

var JWTConfig = jwtConfig{}

func init() {
	validate := validator.New()

	if err := configor.Load(&JWTConfig); err != nil {
		log.Panic(err)
	}
	if err := validate.Struct(JWTConfig); err != nil {
		log.Panic(err)
	}
}
