package logging

import (
	"log"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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
	// nolint:errcheck // error is already logged above, no need to check again
	defer logger.Sync()
	return logger.Sugar()
}
