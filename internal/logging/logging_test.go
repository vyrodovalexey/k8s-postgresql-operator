/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package logging

import (
	"sync"
	"testing"
	_ "unsafe"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

//go:linkname encoderNameToConstructor go.uber.org/zap._encoderNameToConstructor
var encoderNameToConstructor map[string]func(zapcore.EncoderConfig) (zapcore.Encoder, error)

//go:linkname encoderMutex go.uber.org/zap._encoderMutex
var encoderMutex sync.RWMutex

func TestNewLogging(t *testing.T) {
	logger := NewLogging(zapcore.InfoLevel)

	assert.NotNil(t, logger)
}

func TestNewLogging_DifferentLevels(t *testing.T) {
	levels := []zapcore.Level{
		zapcore.DebugLevel,
		zapcore.InfoLevel,
		zapcore.WarnLevel,
		zapcore.ErrorLevel,
		zapcore.DPanicLevel,
		zapcore.PanicLevel,
		zapcore.FatalLevel,
	}

	for _, level := range levels {
		t.Run(level.String(), func(t *testing.T) {
			logger := NewLogging(level)
			assert.NotNil(t, logger)
		})
	}
}

func TestNewLogging_CanLog(t *testing.T) {
	logger := NewLogging(zapcore.InfoLevel)

	// Test that we can actually log
	logger.Info("test message")
	logger.Infow("test structured log", "key", "value")
	logger.Error("test error")
	logger.Debug("test debug") // This might not appear due to level
}

func TestNewLogging_MultipleInstances(t *testing.T) {
	logger1 := NewLogging(zapcore.InfoLevel)
	logger2 := NewLogging(zapcore.DebugLevel)

	// Each call should return a new instance
	assert.NotSame(t, logger1, logger2)
}

func TestNewLogging_ReturnsSugaredLogger(t *testing.T) {
	// Arrange & Act
	logger := NewLogging(zapcore.InfoLevel)

	// Assert - verify the returned logger is a *zap.SugaredLogger
	assert.NotNil(t, logger)
	// Desugar should return the underlying *zap.Logger
	desugared := logger.Desugar()
	assert.NotNil(t, desugared)
}

func TestNewLogging_LogLevelRespected(t *testing.T) {
	tests := []struct {
		name  string
		level zapcore.Level
	}{
		{name: "debug level", level: zapcore.DebugLevel},
		{name: "info level", level: zapcore.InfoLevel},
		{name: "warn level", level: zapcore.WarnLevel},
		{name: "error level", level: zapcore.ErrorLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange & Act
			logger := NewLogging(tt.level)

			// Assert
			assert.NotNil(t, logger)
			desugared := logger.Desugar()
			core := desugared.Core()
			// Verify the core is enabled for the configured level
			assert.True(t, core.Enabled(tt.level))
		})
	}
}

func TestNewLogging_StructuredLogging(t *testing.T) {
	// Arrange
	logger := NewLogging(zapcore.DebugLevel)

	// Act & Assert - verify structured logging methods work without panic
	assert.NotPanics(t, func() {
		logger.Debugw("debug message", "key1", "value1")
		logger.Infow("info message", "key2", "value2", "key3", 42)
		logger.Warnw("warn message", "key4", true)
		logger.Errorw("error message", "key5", []string{"a", "b"})
	})
}

func TestNewLogging_WithFields(t *testing.T) {
	// Arrange
	logger := NewLogging(zapcore.InfoLevel)

	// Act & Assert - verify With method works
	assert.NotPanics(t, func() {
		childLogger := logger.With("component", "test")
		assert.NotNil(t, childLogger)
		childLogger.Info("message from child logger")
	})
}

func TestNewLogging_Named(t *testing.T) {
	// Arrange
	logger := NewLogging(zapcore.InfoLevel)

	// Act & Assert - verify Named method works
	assert.NotPanics(t, func() {
		namedLogger := logger.Named("test-logger")
		assert.NotNil(t, namedLogger)
		namedLogger.Info("message from named logger")
	})
}

func TestNewLogging_BuildError(t *testing.T) {
	// Arrange: temporarily remove the "json" encoder from zap's internal registry
	// to force loggerConfig.Build() to fail, covering the error handling path.
	encoderMutex.Lock()
	originalConstructor := encoderNameToConstructor["json"]
	delete(encoderNameToConstructor, "json")
	encoderMutex.Unlock()

	// Ensure we restore the encoder after the test
	defer func() {
		encoderMutex.Lock()
		encoderNameToConstructor["json"] = originalConstructor
		encoderMutex.Unlock()
	}()

	// Act: NewLogging should handle the Build() error gracefully
	// The function logs the error and continues (will panic on logger.Sugar()
	// since logger is nil when Build fails). We expect a panic here because
	// the function doesn't return early on error.
	assert.Panics(t, func() {
		NewLogging(zapcore.InfoLevel)
	})
}
