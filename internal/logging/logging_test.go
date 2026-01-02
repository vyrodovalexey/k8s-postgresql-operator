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
