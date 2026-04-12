package telemetry

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// newTestSpan creates a real SDK span backed by an in-memory recorder,
// returning both the span and the recorder so assertions can inspect
// the finished span data.
func newTestSpan(t *testing.T) (trace.Span, *tracetest.InMemoryExporter) {
	t.Helper()

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
	})

	_, span := tp.Tracer("test").Start(context.Background(), "test-op")
	return span, exporter
}

func TestFailSpan(t *testing.T) {
	t.Run("sets Error status and records error event", func(t *testing.T) {
		span, exporter := newTestSpan(t)
		testErr := errors.New("something went wrong")

		FailSpan(span, testErr, "operation failed")
		span.End()

		spans := exporter.GetSpans()
		require.Len(t, spans, 1)

		recorded := spans[0]
		assert.Equal(t, codes.Error, recorded.Status.Code)
		assert.Equal(t, "operation failed", recorded.Status.Description)

		// Verify the error was recorded as an event
		require.NotEmpty(t, recorded.Events)

		foundErrEvent := false
		for _, evt := range recorded.Events {
			if evt.Name == "exception" {
				foundErrEvent = true
				break
			}
		}
		assert.True(t, foundErrEvent, "expected an exception event to be recorded")
	})

	t.Run("nil span is no-op", func(t *testing.T) {
		// Must not panic
		assert.NotPanics(t, func() {
			FailSpan(nil, errors.New("err"), "msg")
		})
	})
}

func TestWarnSpan(t *testing.T) {
	t.Run("adds attribute without setting Error status", func(t *testing.T) {
		span, exporter := newTestSpan(t)

		WarnSpan(span, "warn.reason", "cache miss")
		span.End()

		spans := exporter.GetSpans()
		require.Len(t, spans, 1)

		recorded := spans[0]

		// Status must NOT be Error
		assert.NotEqual(t, codes.Error, recorded.Status.Code)

		// Verify the attribute was added
		foundAttr := false
		for _, attr := range recorded.Attributes {
			if attr.Key == attribute.Key("warn.reason") && attr.Value.AsString() == "cache miss" {
				foundAttr = true
				break
			}
		}
		assert.True(t, foundAttr, "expected attribute warn.reason=cache miss to be present")
	})

	t.Run("nil span is no-op", func(t *testing.T) {
		assert.NotPanics(t, func() {
			WarnSpan(nil, "key", "value")
		})
	})
}
