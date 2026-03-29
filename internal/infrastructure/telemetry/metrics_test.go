package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestNewMetrics_Success(t *testing.T) {
	meter := noop.NewMeterProvider().Meter("test")

	metrics, createErr := NewMetrics(meter)

	require.NoError(t, createErr)
	require.NotNil(t, metrics)
}

func TestNewMetrics_AllFieldsPopulated(t *testing.T) {
	meter := noop.NewMeterProvider().Meter("test")

	metrics, createErr := NewMetrics(meter)

	require.NoError(t, createErr)
	require.NotNil(t, metrics)
	assert.NotNil(t, metrics.UsersCreated)
	assert.NotNil(t, metrics.UsersUpdated)
	assert.NotNil(t, metrics.UsersDeleted)
	assert.NotNil(t, metrics.OperationDuration)
}

func TestRecordCreate_Success(t *testing.T) {
	meter := noop.NewMeterProvider().Meter("test")
	metrics, createErr := NewMetrics(meter)
	require.NoError(t, createErr)

	assert.NotPanics(t, func() {
		metrics.RecordCreate(context.Background())
	})
}

func TestRecordCreate_NilReceiver(t *testing.T) {
	var metrics *Metrics

	assert.NotPanics(t, func() {
		metrics.RecordCreate(context.Background())
	})
}

func TestRecordCreate_NilCounter(t *testing.T) {
	metrics := &Metrics{UsersCreated: nil}

	assert.NotPanics(t, func() {
		metrics.RecordCreate(context.Background())
	})
}

func TestRecordUpdate_Success(t *testing.T) {
	meter := noop.NewMeterProvider().Meter("test")
	metrics, createErr := NewMetrics(meter)
	require.NoError(t, createErr)

	assert.NotPanics(t, func() {
		metrics.RecordUpdate(context.Background())
	})
}

func TestRecordUpdate_NilReceiver(t *testing.T) {
	var metrics *Metrics

	assert.NotPanics(t, func() {
		metrics.RecordUpdate(context.Background())
	})
}

func TestRecordDelete_Success(t *testing.T) {
	meter := noop.NewMeterProvider().Meter("test")
	metrics, createErr := NewMetrics(meter)
	require.NoError(t, createErr)

	assert.NotPanics(t, func() {
		metrics.RecordDelete(context.Background())
	})
}

func TestRecordDelete_NilReceiver(t *testing.T) {
	var metrics *Metrics

	assert.NotPanics(t, func() {
		metrics.RecordDelete(context.Background())
	})
}

func TestRecordDuration_Success(t *testing.T) {
	meter := noop.NewMeterProvider().Meter("test")
	metrics, createErr := NewMetrics(meter)
	require.NoError(t, createErr)

	assert.NotPanics(t, func() {
		metrics.RecordDuration(context.Background(), 0.123, "create")
	})
}

func TestRecordDuration_NilReceiver(t *testing.T) {
	var metrics *Metrics

	assert.NotPanics(t, func() {
		metrics.RecordDuration(context.Background(), 0.5, "get")
	})
}

func TestRecordDuration_NilHistogram(t *testing.T) {
	metrics := &Metrics{OperationDuration: nil}

	assert.NotPanics(t, func() {
		metrics.RecordDuration(context.Background(), 1.0, "update")
	})
}
