package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// Metrics contém as métricas de negócio da aplicação
type Metrics struct {
	// Counters
	PeopleCreated metric.Int64Counter
	PeopleUpdated metric.Int64Counter
	PeopleDeleted metric.Int64Counter

	// Histograms
	OperationDuration metric.Float64Histogram
}

var globalMetrics *Metrics

// GetMetrics retorna a instância global de métricas
func GetMetrics() *Metrics {
	return globalMetrics
}

// setupMetrics inicializa as métricas de negócio
func setupMetrics(serviceName string) (*Metrics, error) {
	meter := otel.Meter(serviceName)

	peopleCreated, err := meter.Int64Counter(
		"people_created_total",
		metric.WithDescription("Total number of people created"),
		metric.WithUnit("{person}"),
	)
	if err != nil {
		return nil, err
	}

	peopleUpdated, err := meter.Int64Counter(
		"people_updated_total",
		metric.WithDescription("Total number of people updated"),
		metric.WithUnit("{person}"),
	)
	if err != nil {
		return nil, err
	}

	peopleDeleted, err := meter.Int64Counter(
		"people_deleted_total",
		metric.WithDescription("Total number of people deleted (soft delete)"),
		metric.WithUnit("{person}"),
	)
	if err != nil {
		return nil, err
	}

	operationDuration, err := meter.Float64Histogram(
		"people_operation_duration_seconds",
		metric.WithDescription("Duration of people operations in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	m := &Metrics{
		PeopleCreated:     peopleCreated,
		PeopleUpdated:     peopleUpdated,
		PeopleDeleted:     peopleDeleted,
		OperationDuration: operationDuration,
	}

	globalMetrics = m
	return m, nil
}

// RecordCreate registra uma criação de pessoa
func (m *Metrics) RecordCreate(ctx context.Context) {
	if m != nil && m.PeopleCreated != nil {
		m.PeopleCreated.Add(ctx, 1)
	}
}

// RecordUpdate registra uma atualização de pessoa
func (m *Metrics) RecordUpdate(ctx context.Context) {
	if m != nil && m.PeopleUpdated != nil {
		m.PeopleUpdated.Add(ctx, 1)
	}
}

// RecordDelete registra uma deleção de pessoa
func (m *Metrics) RecordDelete(ctx context.Context) {
	if m != nil && m.PeopleDeleted != nil {
		m.PeopleDeleted.Add(ctx, 1)
	}
}

// RecordDuration registra a duração de uma operação
func (m *Metrics) RecordDuration(ctx context.Context, durationSeconds float64, operation string) {
	if m != nil && m.OperationDuration != nil {
		m.OperationDuration.Record(ctx, durationSeconds)
	}
}
