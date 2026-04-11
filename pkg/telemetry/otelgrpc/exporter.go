package otelgrpc

import (
	"context"

	"github.com/jrmarcello/go-boilerplate/pkg/telemetry"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
)

// Config holds gRPC-specific exporter configuration.
type Config struct {
	CollectorURL string
	Insecure     bool
}

// Exporters creates OTLP gRPC trace and metric exporters and returns them
// as telemetry.Option values ready to pass to telemetry.Setup.
func Exporters(ctx context.Context, cfg Config) ([]telemetry.Option, error) {
	traceOpts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.CollectorURL),
	}
	if cfg.Insecure {
		traceOpts = append(traceOpts, otlptracegrpc.WithInsecure())
	}

	traceExporter, traceErr := otlptracegrpc.New(ctx, traceOpts...)
	if traceErr != nil {
		return nil, traceErr
	}

	metricOpts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.CollectorURL),
	}
	if cfg.Insecure {
		metricOpts = append(metricOpts, otlpmetricgrpc.WithInsecure())
	}

	metricExporter, metricErr := otlpmetricgrpc.New(ctx, metricOpts...)
	if metricErr != nil {
		return nil, metricErr
	}

	return []telemetry.Option{
		telemetry.WithTraceExporter(traceExporter),
		telemetry.WithMetricExporter(metricExporter),
	}, nil
}
