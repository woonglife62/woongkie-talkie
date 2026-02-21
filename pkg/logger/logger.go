package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.SugaredLogger
var RawLogger *zap.Logger

func Initialize(isDev bool) {
	var encConfig zapcore.EncoderConfig
	var defaultLogLevel zapcore.Level
	var encoder zapcore.Encoder

	if isDev {
		encConfig = zap.NewDevelopmentEncoderConfig()
		defaultLogLevel = zapcore.DebugLevel
		encConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoder = zapcore.NewConsoleEncoder(encConfig)
	} else {
		encConfig = zap.NewProductionEncoderConfig()
		defaultLogLevel = zapcore.InfoLevel
		encConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encConfig.EncodeLevel = zapcore.CapitalLevelEncoder
		encoder = zapcore.NewJSONEncoder(encConfig)
	}

	core := zapcore.NewTee(
		zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), defaultLogLevel),
	)
	RawLogger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	Logger = RawLogger.Sugar()
}

func Sync() {
	if RawLogger != nil {
		RawLogger.Sync()
	}
}
