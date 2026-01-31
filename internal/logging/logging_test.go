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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

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

// TestParseLogLevel_TableDriven tests ParseLogLevel with various inputs
func TestParseLogLevel_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedLevel zapcore.Level
	}{
		// Standard levels
		{
			name:          "debug lowercase",
			input:         "debug",
			expectedLevel: zapcore.DebugLevel,
		},
		{
			name:          "info lowercase",
			input:         "info",
			expectedLevel: zapcore.InfoLevel,
		},
		{
			name:          "warn lowercase",
			input:         "warn",
			expectedLevel: zapcore.WarnLevel,
		},
		{
			name:          "warning lowercase",
			input:         "warning",
			expectedLevel: zapcore.WarnLevel,
		},
		{
			name:          "error lowercase",
			input:         "error",
			expectedLevel: zapcore.ErrorLevel,
		},
		// Uppercase
		{
			name:          "DEBUG uppercase",
			input:         "DEBUG",
			expectedLevel: zapcore.DebugLevel,
		},
		{
			name:          "INFO uppercase",
			input:         "INFO",
			expectedLevel: zapcore.InfoLevel,
		},
		{
			name:          "WARN uppercase",
			input:         "WARN",
			expectedLevel: zapcore.WarnLevel,
		},
		{
			name:          "WARNING uppercase",
			input:         "WARNING",
			expectedLevel: zapcore.WarnLevel,
		},
		{
			name:          "ERROR uppercase",
			input:         "ERROR",
			expectedLevel: zapcore.ErrorLevel,
		},
		// Mixed case
		{
			name:          "Debug mixed case",
			input:         "DeBuG",
			expectedLevel: zapcore.DebugLevel,
		},
		{
			name:          "Info mixed case",
			input:         "InFo",
			expectedLevel: zapcore.InfoLevel,
		},
		// With whitespace
		{
			name:          "debug with leading space",
			input:         "  debug",
			expectedLevel: zapcore.DebugLevel,
		},
		{
			name:          "debug with trailing space",
			input:         "debug  ",
			expectedLevel: zapcore.DebugLevel,
		},
		{
			name:          "debug with both spaces",
			input:         "  debug  ",
			expectedLevel: zapcore.DebugLevel,
		},
		{
			name:          "info with tabs",
			input:         "\tinfo\t",
			expectedLevel: zapcore.InfoLevel,
		},
		// Invalid/unknown levels - should default to info
		{
			name:          "empty string",
			input:         "",
			expectedLevel: zapcore.InfoLevel,
		},
		{
			name:          "unknown level",
			input:         "unknown",
			expectedLevel: zapcore.InfoLevel,
		},
		{
			name:          "invalid level",
			input:         "invalid",
			expectedLevel: zapcore.InfoLevel,
		},
		{
			name:          "trace (not supported)",
			input:         "trace",
			expectedLevel: zapcore.InfoLevel,
		},
		{
			name:          "fatal (not supported)",
			input:         "fatal",
			expectedLevel: zapcore.InfoLevel,
		},
		{
			name:          "panic (not supported)",
			input:         "panic",
			expectedLevel: zapcore.InfoLevel,
		},
		{
			name:          "numeric value",
			input:         "1",
			expectedLevel: zapcore.InfoLevel,
		},
		{
			name:          "special characters",
			input:         "!@#$%",
			expectedLevel: zapcore.InfoLevel,
		},
		{
			name:          "whitespace only",
			input:         "   ",
			expectedLevel: zapcore.InfoLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseLogLevel(tt.input)
			assert.Equal(t, tt.expectedLevel, result)
		})
	}
}

// TestNewLoggingFromString_TableDriven tests NewLoggingFromString with various inputs
func TestNewLoggingFromString_TableDriven(t *testing.T) {
	tests := []struct {
		name  string
		level string
	}{
		{
			name:  "debug level",
			level: "debug",
		},
		{
			name:  "info level",
			level: "info",
		},
		{
			name:  "warn level",
			level: "warn",
		},
		{
			name:  "warning level",
			level: "warning",
		},
		{
			name:  "error level",
			level: "error",
		},
		{
			name:  "empty string defaults to info",
			level: "",
		},
		{
			name:  "invalid level defaults to info",
			level: "invalid",
		},
		{
			name:  "uppercase DEBUG",
			level: "DEBUG",
		},
		{
			name:  "mixed case InFo",
			level: "InFo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLoggingFromString(tt.level)
			assert.NotNil(t, logger)
		})
	}
}

// TestNewLoggingFromString_CanLog tests that loggers created from string can actually log
func TestNewLoggingFromString_CanLog(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error"}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			logger := NewLoggingFromString(level)
			assert.NotNil(t, logger)

			// Test that we can actually log without panicking
			logger.Info("test message")
			logger.Infow("test structured log", "key", "value")
			logger.Debug("test debug")
			logger.Warn("test warn")
			logger.Error("test error")
		})
	}
}

// TestParseLogLevel_Consistency tests that ParseLogLevel is consistent
func TestParseLogLevel_Consistency(t *testing.T) {
	// Multiple calls with same input should return same result
	for i := 0; i < 10; i++ {
		assert.Equal(t, zapcore.DebugLevel, ParseLogLevel("debug"))
		assert.Equal(t, zapcore.InfoLevel, ParseLogLevel("info"))
		assert.Equal(t, zapcore.WarnLevel, ParseLogLevel("warn"))
		assert.Equal(t, zapcore.ErrorLevel, ParseLogLevel("error"))
	}
}

// TestNewLoggingFromString_MultipleInstances tests that each call creates a new logger
func TestNewLoggingFromString_MultipleInstances(t *testing.T) {
	logger1 := NewLoggingFromString("info")
	logger2 := NewLoggingFromString("info")

	assert.NotNil(t, logger1)
	assert.NotNil(t, logger2)
	assert.NotSame(t, logger1, logger2)
}
