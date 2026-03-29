package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics contém as métricas de negócio da aplicação
type Metrics struct {
	// Counters
	UsersCreated metric.Int64Counter
	UsersUpdated metric.Int64Counter
	UsersDeleted metric.Int64Counter

	// Histograms
	OperationDuration metric.Float64Histogram
}

// NewMetrics creates business metrics instruments using the provided meter.
func NewMetrics(meter metric.Meter) (*Metrics, error) {
	usersCreated, createErr := meter.Int64Counter(
		"users_created_total",
		metric.WithDescription("Total number of users created"),
		metric.WithUnit("{user}"),
	)
	if createErr != nil {
		return nil, createErr
	}

	usersUpdated, updateErr := meter.Int64Counter(
		"users_updated_total",
		metric.WithDescription("Total number of users updated"),
		metric.WithUnit("{user}"),
	)
	if updateErr != nil {
		return nil, updateErr
	}

	usersDeleted, deleteErr := meter.Int64Counter(
		"users_deleted_total",
		metric.WithDescription("Total number of users deleted (soft delete)"),
		metric.WithUnit("{user}"),
	)
	if deleteErr != nil {
		return nil, deleteErr
	}

	operationDuration, durationErr := meter.Float64Histogram(
		"users_operation_duration_seconds",
		metric.WithDescription("Duration of users operations in seconds"),
		metric.WithUnit("s"),
	)
	if durationErr != nil {
		return nil, durationErr
	}

	return &Metrics{
		UsersCreated:      usersCreated,
		UsersUpdated:      usersUpdated,
		UsersDeleted:      usersDeleted,
		OperationDuration: operationDuration,
	}, nil
}

// RecordCreate registra uma criação de usuário
func (m *Metrics) RecordCreate(ctx context.Context) {
	if m != nil && m.UsersCreated != nil {
		m.UsersCreated.Add(ctx, 1)
	}
}

// RecordUpdate registra uma atualização de usuário
func (m *Metrics) RecordUpdate(ctx context.Context) {
	if m != nil && m.UsersUpdated != nil {
		m.UsersUpdated.Add(ctx, 1)
	}
}

// RecordDelete registra uma deleção de usuário
func (m *Metrics) RecordDelete(ctx context.Context) {
	if m != nil && m.UsersDeleted != nil {
		m.UsersDeleted.Add(ctx, 1)
	}
}

// RecordDuration registra a duração de uma operação
func (m *Metrics) RecordDuration(ctx context.Context, durationSeconds float64, operation string) {
	if m != nil && m.OperationDuration != nil {
		m.OperationDuration.Record(ctx, durationSeconds,
			metric.WithAttributes(attribute.String("operation", operation)),
		)
	}
}
