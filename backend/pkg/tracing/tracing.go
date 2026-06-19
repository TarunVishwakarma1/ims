// Package tracing wires OpenTelemetry traces for the backend. Exporter is
// chosen at boot from env:
//
//	OTEL_EXPORTER     = "stdout" (default in dev) | "otlp" (Tempo / Jaeger)
//	OTEL_ENDPOINT     = host:port for otlptracegrpc when exporter=otlp
//	OTEL_SERVICE_NAME = service name attribute (defaults to "ims-backend")
//
// Init returns a shutdown closure the caller defers to flush spans.
package tracing

import (
	"context"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.uber.org/zap"
)

// Init configures the global TracerProvider and propagator. Returns a
// shutdown func; safe to call no-op if init failed.
func Init(ctx context.Context, env string) (shutdown func(context.Context) error, err error) {
	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "ims-backend"
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.DeploymentEnvironment(env),
		),
		resource.WithProcessRuntimeName(),
	)
	if err != nil {
		return noopShutdown, err
	}

	var exporter sdktrace.SpanExporter
	switch os.Getenv("OTEL_EXPORTER") {
	case "otlp":
		endpoint := os.Getenv("OTEL_ENDPOINT")
		if endpoint == "" {
			endpoint = "localhost:4317"
		}
		client := otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint(endpoint),
			otlptracegrpc.WithInsecure(),
		)
		exporter, err = otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(endpoint), otlptracegrpc.WithInsecure())
		_ = client
		if err != nil {
			return noopShutdown, err
		}
		zap.L().Info("OTel OTLP exporter wired", zap.String("endpoint", endpoint))
	default:
		// Dev default: stdout pretty-printed.
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return noopShutdown, err
		}
		zap.L().Info("OTel stdout exporter wired (set OTEL_EXPORTER=otlp for collector)")
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
		),
		sdktrace.WithResource(res),
		// 100% in dev; flip to ratio-based via env in prod if cost matters.
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

func noopShutdown(context.Context) error { return nil }
