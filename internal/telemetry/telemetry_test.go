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

package telemetry

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestInitTracerProvider_EmptyEndpoint(t *testing.T) {
	// Arrange
	ctx := context.Background()

	// Act
	tp, err := InitTracerProvider(ctx, "test-service", "", true)

	// Assert
	assert.NoError(t, err)
	assert.Nil(t, tp)
}

func TestInitTracerProvider_WithEndpoint(t *testing.T) {
	// Arrange
	ctx := context.Background()

	// Act
	tp, err := InitTracerProvider(
		ctx, "test-service", "localhost:4317", false,
	)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, tp)

	// Cleanup
	t.Cleanup(func() {
		_ = tp.Shutdown(ctx)
	})
}

func TestInitTracerProvider_WithInsecure(t *testing.T) {
	// Arrange
	ctx := context.Background()

	// Act
	tp, err := InitTracerProvider(
		ctx, "test-service", "localhost:4317", true,
	)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, tp)

	// Cleanup
	t.Cleanup(func() {
		_ = tp.Shutdown(ctx)
	})
}

func TestInitTracerProvider_SetsGlobalProvider(t *testing.T) {
	// Arrange
	ctx := context.Background()

	// Act
	tp, err := InitTracerProvider(
		ctx, "test-service", "localhost:4317", true,
	)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, tp)

	globalTP := otel.GetTracerProvider()
	assert.Equal(t, tp, globalTP)

	// Cleanup
	t.Cleanup(func() {
		_ = tp.Shutdown(ctx)
	})
}

func TestShutdownTracerProvider_NilProvider(t *testing.T) {
	// Arrange
	ctx := context.Background()

	// Act & Assert - should not panic
	assert.NotPanics(t, func() {
		ShutdownTracerProvider(ctx, nil)
	})
}

func TestShutdownTracerProvider_WithProvider(t *testing.T) {
	// Arrange
	ctx := context.Background()
	tp, err := InitTracerProvider(
		ctx, "test-service", "localhost:4317", true,
	)
	require.NoError(t, err)
	require.NotNil(t, tp)

	// Act & Assert - should not panic
	assert.NotPanics(t, func() {
		ShutdownTracerProvider(ctx, tp)
	})
}

func TestStartSpan_WithoutAttributes(t *testing.T) {
	// Arrange
	ctx := context.Background()
	tracer := noop.NewTracerProvider().Tracer("test")

	// Act
	spanCtx, span := StartSpan(ctx, tracer, "test-span")

	// Assert
	assert.NotNil(t, spanCtx)
	assert.NotNil(t, span)
	span.End()
}

func TestStartSpan_WithAttributes(t *testing.T) {
	// Arrange
	ctx := context.Background()
	tracer := noop.NewTracerProvider().Tracer("test")
	attrs := []attribute.KeyValue{
		attribute.String(AttrPostgresqlID, "pg-1"),
		attribute.String(AttrDatabase, "testdb"),
	}

	// Act
	spanCtx, span := StartSpan(ctx, tracer, "test-span", attrs...)

	// Assert
	assert.NotNil(t, spanCtx)
	assert.NotNil(t, span)
	span.End()
}

func TestStartSpan_WithSingleAttribute(t *testing.T) {
	// Arrange
	ctx := context.Background()
	tracer := noop.NewTracerProvider().Tracer("test")

	// Act
	spanCtx, span := StartSpan(
		ctx, tracer, "test-span",
		attribute.String(AttrController, "test-ctrl"),
	)

	// Assert
	assert.NotNil(t, spanCtx)
	assert.NotNil(t, span)
	span.End()
}

func TestRecordError_NilError(t *testing.T) {
	// Arrange
	tracer := noop.NewTracerProvider().Tracer("test")
	_, span := tracer.Start(context.Background(), "test-span")

	// Act & Assert - should not panic
	assert.NotPanics(t, func() {
		RecordError(span, nil)
	})
	span.End()
}

func TestRecordError_WithError(t *testing.T) {
	// Arrange
	tracer := noop.NewTracerProvider().Tracer("test")
	_, span := tracer.Start(context.Background(), "test-span")
	testErr := errors.New("test error")

	// Act & Assert - should not panic
	assert.NotPanics(t, func() {
		RecordError(span, testErr)
	})
	span.End()
}

func TestRecordError_WithRealTracerProvider(t *testing.T) {
	// Arrange
	ctx := context.Background()
	tp, err := InitTracerProvider(
		ctx, "test-service", "localhost:4317", true,
	)
	require.NoError(t, err)
	require.NotNil(t, tp)

	t.Cleanup(func() {
		_ = tp.Shutdown(ctx)
	})

	tracer := tp.Tracer("test")
	spanCtx, span := tracer.Start(ctx, "test-span")
	testErr := errors.New("real tracer error")

	// Act & Assert
	assert.NotPanics(t, func() {
		RecordError(span, testErr)
	})
	assert.NotNil(t, spanCtx)
	span.End()
}

func TestConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "AttrPostgresqlID",
			constant: AttrPostgresqlID,
			expected: "postgresql.id",
		},
		{
			name:     "AttrUsername",
			constant: AttrUsername,
			expected: "db.user",
		},
		{
			name:     "AttrDatabase",
			constant: AttrDatabase,
			expected: "db.name",
		},
		{
			name:     "AttrController",
			constant: AttrController,
			expected: "controller",
		},
		{
			name:     "AttrOperation",
			constant: AttrOperation,
			expected: "operation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.constant)
		})
	}
}

func TestServiceVersion(t *testing.T) {
	// Verify the internal constant is set
	assert.Equal(t, "0.0.1", serviceVersion)
}

func TestShutdownTimeout(t *testing.T) {
	// Verify the shutdown timeout constant
	assert.Equal(t, 5*1e9, float64(shutdownTimeout))
}

func TestInitTracerProvider_EmptyEndpoint_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		wantNil  bool
	}{
		{
			name:     "empty string endpoint",
			endpoint: "",
			wantNil:  true,
		},
		{
			name:     "non-empty endpoint",
			endpoint: "localhost:4317",
			wantNil:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := context.Background()

			// Act
			tp, err := InitTracerProvider(
				ctx, "test-service", tt.endpoint, true,
			)

			// Assert
			assert.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, tp)
			} else {
				assert.NotNil(t, tp)
				t.Cleanup(func() {
					_ = tp.Shutdown(ctx)
				})
			}
		})
	}
}

func TestStartSpan_ReturnsEnrichedContext(t *testing.T) {
	// Arrange
	ctx := context.Background()
	tp := sdktrace.NewTracerProvider()
	tracer := tp.Tracer("test")

	t.Cleanup(func() {
		_ = tp.Shutdown(ctx)
	})

	// Act
	spanCtx, span := StartSpan(
		ctx, tracer, "enriched-span",
		attribute.String(AttrOperation, "test-op"),
	)

	// Assert - context should be different from original
	assert.NotNil(t, spanCtx)
	assert.NotNil(t, span)
	assert.NotEqual(t, ctx, spanCtx)
	span.End()
}

func TestStartSpan_EmptyAttributes(t *testing.T) {
	// Arrange
	ctx := context.Background()
	tracer := noop.NewTracerProvider().Tracer("test")
	emptyAttrs := []attribute.KeyValue{}

	// Act - empty slice should take the no-attributes path
	spanCtx, span := StartSpan(ctx, tracer, "test-span", emptyAttrs...)

	// Assert
	assert.NotNil(t, spanCtx)
	assert.NotNil(t, span)
	span.End()
}
