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

// Package telemetry provides OpenTelemetry distributed tracing utilities
// for the k8s-postgresql-operator. It initializes OTLP gRPC exporters,
// manages TracerProvider lifecycle, and offers helpers for span creation
// and error recording.
package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	// serviceVersion is the current version of the operator.
	serviceVersion = "0.0.1"

	// shutdownTimeout is the maximum time to wait for TracerProvider shutdown.
	shutdownTimeout = 5 * time.Second

	// AttrPostgresqlID is the span attribute key for PostgreSQL instance ID.
	AttrPostgresqlID = "postgresql.id"
	// AttrUsername is the span attribute key for database username.
	AttrUsername = "db.user"
	// AttrDatabase is the span attribute key for database name.
	AttrDatabase = "db.name"
	// AttrController is the span attribute key for controller name.
	AttrController = "controller"
	// AttrOperation is the span attribute key for operation name.
	AttrOperation = "operation"
)

// InitTracerProvider sets up an OTLP gRPC exporter and TracerProvider.
// If otlpEndpoint is empty, the global no-op provider is used and nil
// is returned so the caller can safely skip shutdown.
func InitTracerProvider(
	ctx context.Context,
	serviceName, otlpEndpoint string,
	insecureConn bool,
) (*sdktrace.TracerProvider, error) {
	if otlpEndpoint == "" {
		// No endpoint configured; leave the default no-op provider.
		return nil, nil //nolint:nilnil // intentional: nil provider means no-op
	}

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(otlpEndpoint),
	}
	if insecureConn {
		opts = append(opts,
			otlptracegrpc.WithTLSCredentials(insecure.NewCredentials()),
		)
	}

	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create OTLP trace exporter: %w", err,
		)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create OTel resource: %w", err,
		)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)

	return tp, nil
}

// ShutdownTracerProvider gracefully shuts down the TracerProvider,
// flushing any pending spans. It is safe to call with a nil provider.
func ShutdownTracerProvider(
	ctx context.Context, tp *sdktrace.TracerProvider,
) {
	if tp == nil {
		return
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()

	_ = tp.Shutdown(shutdownCtx)
}

// StartSpan creates a new span with the given name and optional
// attributes. It returns the enriched context and the span.
func StartSpan(
	ctx context.Context,
	tracer trace.Tracer,
	spanName string,
	attrs ...attribute.KeyValue,
) (context.Context, trace.Span) {
	if len(attrs) > 0 {
		return tracer.Start(ctx, spanName,
			trace.WithAttributes(attrs...),
		)
	}
	return tracer.Start(ctx, spanName)
}

// RecordError records an error on the span and sets the span status
// to Error. It is safe to call with a nil error (no-op).
func RecordError(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}
