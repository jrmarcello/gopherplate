package telemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// Config contém as configurações de telemetria
type Config struct {
	ServiceName  string
	CollectorURL string
	Enabled      bool
}

// Provider encapsula os providers de tracing e métricas
type Provider struct {
	tp      *sdktrace.TracerProvider
	mp      *sdkmetric.MeterProvider
	metrics *Metrics
}

// Setup inicializa OpenTelemetry (Traces + Metrics)
func Setup(ctx context.Context, cfg Config) (*Provider, error) {
	if !cfg.Enabled || cfg.CollectorURL == "" {
		slog.Info("OpenTelemetry disabled or no collector URL configured")
		return &Provider{}, nil
	}

	// Resource com informações do serviço
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, err
	}

	// Traces
	tp, err := setupTracer(ctx, cfg.CollectorURL, res)
	if err != nil {
		return nil, err
	}

	// Metrics
	mp, err := setupMeter(ctx, cfg.CollectorURL, res)
	if err != nil {
		return nil, err
	}

	// Business Metrics
	metrics, err := setupMetrics(cfg.ServiceName)
	if err != nil {
		return nil, err
	}

	slog.Info("OpenTelemetry initialized",
		"service", cfg.ServiceName,
		"collector", cfg.CollectorURL,
	)

	return &Provider{tp: tp, mp: mp, metrics: metrics}, nil
}

func setupTracer(ctx context.Context, collectorURL string, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(collectorURL),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}

func setupMeter(ctx context.Context, collectorURL string, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	exporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(collectorURL),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(mp)

	return mp, nil
}

// Shutdown encerra os providers de telemetria
func (p *Provider) Shutdown(ctx context.Context) error {
	if p.tp != nil {
		if err := p.tp.Shutdown(ctx); err != nil {
			return err
		}
	}
	if p.mp != nil {
		if err := p.mp.Shutdown(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Metrics retorna as métricas de negócio
func (p *Provider) Metrics() *Metrics {
	return p.metrics
}
