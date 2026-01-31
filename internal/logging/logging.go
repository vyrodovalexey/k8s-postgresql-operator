package logging

import (
	"log"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ParseLogLevel parses a string log level to zapcore.Level
// Supported levels: debug, info, warn, error
// Returns info level if the string is not recognized
func ParseLogLevel(level string) zapcore.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// NewLogging creates a new SugaredLogger with the specified log level
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

// NewLoggingFromString creates a new SugaredLogger from a string log level
// Supported levels: debug, info, warn, error
func NewLoggingFromString(level string) *zap.SugaredLogger {
	return NewLogging(ParseLogLevel(level))
}
