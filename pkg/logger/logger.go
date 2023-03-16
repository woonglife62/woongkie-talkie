package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger
var Logger *zap.SugaredLogger

func Initialize(production bool) {
	var config zapcore.EncoderConfig
	var defaultLogLevel zapcore.Level
	if production {
		config = zap.NewProductionEncoderConfig()
		defaultLogLevel = zapcore.DebugLevel
	} else {
		config = zap.NewDevelopmentEncoderConfig()
		defaultLogLevel = zapcore.InfoLevel
	}
	config.EncodeTime = zapcore.ISO8601TimeEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(config)
	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), defaultLogLevel),
	)
	logger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.WarnLevel))
	Logger = logger.Sugar()
}

func init() {
	Initialize(true)
}
