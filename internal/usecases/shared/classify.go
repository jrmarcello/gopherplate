package shared

import (
	"errors"

	"github.com/jrmarcello/gopherplate/pkg/telemetry"
	"go.opentelemetry.io/otel/trace"
)

// ClassifyError inspects err against a list of expected domain errors and
// routes telemetry accordingly:
//   - nil error: no-op (returns immediately).
//   - Match in expectedErrors (via errors.Is, supports wrapping): records a
//     warning event on the span using telemetry.WarnSpan.
//   - No match: marks the span as failed using telemetry.FailSpan.
func ClassifyError(span trace.Span, err error, expectedErrors []error, contextMsg string) {
	if err == nil {
		return
	}

	for _, expected := range expectedErrors {
		if errors.Is(err, expected) {
			telemetry.WarnSpan(span, "expected.error", err.Error())
			return
		}
	}

	telemetry.FailSpan(span, err, contextMsg)
}
