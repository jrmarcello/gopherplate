package telemetry

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// FailSpan marks the span as failed by setting its status to Error,
// recording the error as an event, and attaching a human-readable message.
// It is a no-op if span is nil.
func FailSpan(span trace.Span, err error, msg string) {
	if span == nil {
		return
	}
	span.SetStatus(codes.Error, msg)
	span.RecordError(err)
}

// WarnSpan adds a semantic attribute to the span without changing its status
// to Error. Useful for annotating non-fatal conditions (e.g. cache misses,
// fallback paths) that are worth surfacing in traces.
// It is a no-op if span is nil.
func WarnSpan(span trace.Span, key, value string) {
	if span == nil {
		return
	}
	span.SetAttributes(attribute.String(key, value))
}
