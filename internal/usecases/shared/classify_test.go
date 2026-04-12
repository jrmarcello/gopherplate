package shared

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestSpan creates a real, recording span backed by an in-memory exporter.
// Callers must call span.End() and tp.ForceFlush() before inspecting the
// exporter's finished spans.
func newTestSpan(exp *tracetest.InMemoryExporter) (sdktrace.ReadWriteSpan, *sdktrace.TracerProvider) {
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp),
	)
	_, span := tp.Tracer("test").Start(context.Background(), "test-span")
	return span.(sdktrace.ReadWriteSpan), tp
}

func TestClassifyError(t *testing.T) {
	errNotFound := errors.New("not found")
	errDuplicate := errors.New("duplicate entry")
	errInternal := errors.New("connection refused")

	tests := []struct {
		name           string
		err            error
		expectedErrors []error
		contextMsg     string
		wantStatus     codes.Code
		wantAttrKey    string // WarnSpan sets attribute, not event
		wantAttrValue  string
		wantNoSpanCall bool
	}{
		{
			name:           "TC-U-04: routes expected error to WarnSpan",
			err:            errNotFound,
			expectedErrors: []error{errNotFound, errDuplicate},
			contextMsg:     "getting user",
			wantStatus:     codes.Unset,
			wantAttrKey:    "expected.error",
			wantAttrValue:  errNotFound.Error(),
		},
		{
			name:           "TC-U-05: routes unexpected error to FailSpan",
			err:            errInternal,
			expectedErrors: []error{errNotFound, errDuplicate},
			contextMsg:     "getting user",
			wantStatus:     codes.Error,
		},
		{
			name:           "TC-U-06: wrapped expected error still matches via errors.Is",
			err:            fmt.Errorf("repo: %w", errNotFound),
			expectedErrors: []error{errNotFound},
			contextMsg:     "getting user",
			wantStatus:     codes.Unset,
			wantAttrKey:    "expected.error",
			wantAttrValue:  fmt.Errorf("repo: %w", errNotFound).Error(),
		},
		{
			name:           "TC-U-07: empty expectedErrors treats all as unexpected",
			err:            errNotFound,
			expectedErrors: []error{},
			contextMsg:     "getting user",
			wantStatus:     codes.Error,
		},
		{
			name:           "TC-U-08: nil error is no-op",
			err:            nil,
			expectedErrors: []error{errNotFound},
			contextMsg:     "getting user",
			wantNoSpanCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp := tracetest.NewInMemoryExporter()
			span, tp := newTestSpan(exp)

			ClassifyError(span, tt.err, tt.expectedErrors, tt.contextMsg)

			span.End()
			flushErr := tp.ForceFlush(context.Background())
			require.NoError(t, flushErr)

			finished := exp.GetSpans()
			require.Len(t, finished, 1)

			stub := finished[0]

			if tt.wantNoSpanCall {
				assert.Equal(t, codes.Unset, stub.Status.Code, "nil error should leave span status unset")
				assert.Empty(t, stub.Events, "nil error should not add events")
				return
			}

			assert.Equal(t, tt.wantStatus, stub.Status.Code)

			// WarnSpan uses SetAttributes (not events) — check attributes
			if tt.wantAttrKey != "" {
				wantAttr := attribute.String(tt.wantAttrKey, tt.wantAttrValue)
				assert.Contains(t, stub.Attributes, wantAttr, "expected attribute %s=%s", tt.wantAttrKey, tt.wantAttrValue)
			}

			if tt.wantStatus == codes.Error {
				assert.Equal(t, tt.contextMsg, stub.Status.Description)
				// FailSpan should have recorded the error as an exception event
				foundException := false
				for _, ev := range stub.Events {
					if ev.Name == "exception" {
						foundException = true
					}
				}
				assert.True(t, foundException, "expected RecordError to produce an 'exception' event")
			}
		})
	}
}
