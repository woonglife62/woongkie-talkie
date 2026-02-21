package config

import (
	"fmt"
	"os"
	"time"

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

// ShutdownTimeout is the duration to wait for graceful shutdown.
// Controlled by the SHUTDOWN_TIMEOUT env var (e.g. "30s", "1m"). Default: 30s.
var ShutdownTimeout = 30 * time.Second

// HubIdleTimeout is the duration a hub can be empty before it auto-shuts down.
// Controlled by the HUB_IDLE_TIMEOUT env var (e.g. "5m", "10m"). Default: 5m.
var HubIdleTimeout = 5 * time.Minute

// mongoDB config
type dbConfig struct {
	URI      string `env:"MONGODB_URI" validate:"required"`
	User     string `env:"MONGODB_USER" validate:"required"`
	Password string `env:"MONGODB_PASSWORD" validate:"required"`
	Database string `env:"MONGODB_DATABASE" validate:"required"`
}

var Config = config{}

var DBConfig = dbConfig{}

// LoadAll loads and validates all configuration. Returns an error on failure.
// Called explicitly from cmd/serve.go in addition to the package init().
func LoadAll() error {
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

	if err := godotenv.Load(envFilePath); err != nil {
		log.Error("Error loading .env file. " + err.Error())
	}

	if err := configor.Load(&Config); err != nil {
		return fmt.Errorf("config load error: %w", err)
	}
	if err := validate.Struct(Config); err != nil {
		return fmt.Errorf("config validation error: %w", err)
	}

	// init logger
	if Config.IsDev == "DEV" || Config.IsDev == "dev" || Config.IsDev == "develop" {
		logger.Initialize(true)
	} else if Config.IsDev == "PROD" || Config.IsDev == "prod" || Config.IsDev == "product" || Config.IsDev == "production" {
		logger.Initialize(false)
	} else {
		return fmt.Errorf("IS_DEV value must be filled")
	}

	if err := configor.Load(&DBConfig); err != nil {
		return fmt.Errorf("db config load error: %w", err)
	}
	if err := validate.Struct(DBConfig); err != nil {
		return fmt.Errorf("db config validation error: %w", err)
	}

	if err := loadJWT(); err != nil {
		return fmt.Errorf("jwt config error: %w", err)
	}

	if err := loadRedis(); err != nil {
		return fmt.Errorf("redis config error: %w", err)
	}

	if err := loadTLS(); err != nil {
		return fmt.Errorf("tls config error: %w", err)
	}

	if v := os.Getenv("SHUTDOWN_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			ShutdownTimeout = d
		}
	}

	if v := os.Getenv("HUB_IDLE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			HubIdleTimeout = d
		}
	}

	return nil
}

func init() {
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

	err := godotenv.Load(envFilePath)
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
	} else if Config.IsDev == "PROD" || Config.IsDev == "prod" || Config.IsDev == "product" || Config.IsDev == "production" {
		logger.Initialize(false)
	} else {
		log.Panic("IS_DEV value must be filled.")
	}

	// config load & validate
	if err := configor.Load(&DBConfig); err != nil {
		log.Panic(err)
	}
	if err = validate.Struct(DBConfig); err != nil {
		log.Panic(err)
	}
}
