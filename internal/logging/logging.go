package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"log"
	"time"
)

func NewLogging(loglevel zapcore.Level) *zap.SugaredLogger {
	loggerConfig := zap.NewProductionConfig()
	loggerConfig.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC3339)
	loggerConfig.DisableCaller = true
	loggerConfig.Level.SetLevel(loglevel)

	logger, err := loggerConfig.Build()
	if err != nil {
		log.Printf("can't initialize zap logger: %v", err)
	}
	// nolint:errcheck
	defer logger.Sync()
	return logger.Sugar()
}
