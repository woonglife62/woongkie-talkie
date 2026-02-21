package config

import (
	"fmt"

	"github.com/jinzhu/configor"
	"github.com/labstack/gommon/log"
	"gopkg.in/go-playground/validator.v9"
)

type jwtConfig struct {
	Secret string `env:"JWT_SECRET" validate:"required,min=32"`
	Expiry string `env:"JWT_EXPIRY" default:"24h"`
}

var JWTConfig = jwtConfig{}

// loadJWT loads and validates JWT configuration. Called from LoadAll().
func loadJWT() error {
	validate := validator.New()
	if err := configor.Load(&JWTConfig); err != nil {
		return fmt.Errorf("jwt configor load: %w", err)
	}
	if err := validate.Struct(JWTConfig); err != nil {
		return fmt.Errorf("jwt validation: %w", err)
	}
	return nil
}

func init() {
	validate := validator.New()

	if err := configor.Load(&JWTConfig); err != nil {
		log.Panic(err)
	}
	if err := validate.Struct(JWTConfig); err != nil {
		log.Panic(err)
	}
}
