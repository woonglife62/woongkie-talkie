package config

import (
	"os"

	"github.com/jinzhu/configor"
	"github.com/joho/godotenv"
	"github.com/labstack/gommon/log"
	"github.com/woonglife62/woongkie-talkie/pkg/logger"
	"gopkg.in/go-playground/validator.v9"
)

type config struct {
	// DEV, dev, develop
	// PROD, prod, product
	IsDev string `env:"IS_DEV" validate:"required"`
}

var Config = config{}

func init() {

	var err error

	validate := validator.New()

	envFilePathsCandidates := []string{
		".env",
		os.ExpandEnv("$GOPATH/src/woongkie-talkie/.env"),
	}

	envFilePath := ""
	for _, envFilePathsCandidate := range envFilePathsCandidates {
		if _, ok := os.Stat(envFilePathsCandidate); ok == nil {
			envFilePath = envFilePathsCandidate
			break
		}
	}

	err = godotenv.Load(envFilePath)
	if err != nil {
		log.Error("Error loading .env file. " + err.Error())
	}

	// config load & validate
	if err := configor.Load(&Config); err != nil {
		log.Panic(err)
	}
	if err = validate.Struct(Config); err != nil {
		log.Panic(err)
	}

	// init logger
	if Config.IsDev == "DEV" || Config.IsDev == "dev" || Config.IsDev == "develop" {
		logger.Initialize(true)
	} else if Config.IsDev == "PROD" || Config.IsDev == "prod" || Config.IsDev == "product" {
		logger.Initialize(false)
	} else {
		log.Panic("IS_DEV value must be filled.")
	}

}
