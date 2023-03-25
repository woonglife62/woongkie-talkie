package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.SugaredLogger

func Initialize(isDev bool) {
	var config zapcore.EncoderConfig
	var defaultLogLevel zapcore.Level

	if isDev {
		config = zap.NewDevelopmentEncoderConfig()
		defaultLogLevel = zapcore.InfoLevel
	} else {
		config = zap.NewProductionEncoderConfig()
		defaultLogLevel = zapcore.DebugLevel
	}
	config.EncodeTime = zapcore.ISO8601TimeEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(config)
	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), defaultLogLevel),
	)
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.WarnLevel))
	Logger = logger.Sugar()
}
