package telemetry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestDefaultApdexThresholds(t *testing.T) {
	thresholds := DefaultApdexThresholds()

	assert.Equal(t, 500*time.Millisecond, thresholds.Satisfied)
	assert.Equal(t, 2*time.Second, thresholds.Tolerating)
}

func TestNewHTTPMetrics_Success(t *testing.T) {
	otel.SetMeterProvider(noop.NewMeterProvider())
	t.Cleanup(func() { otel.SetMeterProvider(nil) })

	metrics, createErr := NewHTTPMetrics("test-service")

	require.NoError(t, createErr)
	require.NotNil(t, metrics)
}

func TestNewHTTPMetrics_AllFieldsPopulated(t *testing.T) {
	otel.SetMeterProvider(noop.NewMeterProvider())
	t.Cleanup(func() { otel.SetMeterProvider(nil) })

	metrics, createErr := NewHTTPMetrics("test-service")

	require.NoError(t, createErr)
	require.NotNil(t, metrics)
	assert.NotNil(t, metrics.RequestCount)
	assert.NotNil(t, metrics.RequestDuration)
	assert.NotNil(t, metrics.SlowRequests)
	assert.NotNil(t, metrics.ApdexSatisfied)
	assert.NotNil(t, metrics.ApdexTolerating)
	assert.NotNil(t, metrics.ApdexFrustrated)
}

func TestNewHTTPMetrics_ThresholdsSet(t *testing.T) {
	otel.SetMeterProvider(noop.NewMeterProvider())
	t.Cleanup(func() { otel.SetMeterProvider(nil) })

	metrics, createErr := NewHTTPMetrics("test-service")

	require.NoError(t, createErr)
	require.NotNil(t, metrics)

	expected := DefaultApdexThresholds()
	assert.Equal(t, expected.Satisfied, metrics.Thresholds.Satisfied)
	assert.Equal(t, expected.Tolerating, metrics.Thresholds.Tolerating)
}
