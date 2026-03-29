package logutil

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInjectAndExtract(t *testing.T) {
	ctx := context.Background()
	lc := LogContext{
		RequestID: "req-123",
		TraceID:   "trace-456",
		Step:      StepHandler,
	}

	ctx = Inject(ctx, lc)
	extracted, ok := Extract(ctx)

	assert.True(t, ok)
	assert.Equal(t, "req-123", extracted.RequestID)
	assert.Equal(t, "trace-456", extracted.TraceID)
	assert.Equal(t, StepHandler, extracted.Step)
}

func TestExtractFromEmptyContext(t *testing.T) {
	ctx := context.Background()
	_, ok := Extract(ctx)
	assert.False(t, ok)
}

func TestLogContext_WithStep(t *testing.T) {
	lc := LogContext{RequestID: "req-123", Step: StepHandler}
	ucLC := lc.WithStep(StepUseCase)

	assert.Equal(t, StepUseCase, ucLC.Step)
	assert.Equal(t, StepHandler, lc.Step) // original not mutated
}

func TestLogContext_WithResource(t *testing.T) {
	lc := LogContext{RequestID: "req-123"}
	withRes := lc.WithResource("entity")

	assert.Equal(t, "entity", withRes.Resource)
	assert.Equal(t, "", lc.Resource) // original not mutated
}

func TestLogContext_WithAction(t *testing.T) {
	lc := LogContext{RequestID: "req-123"}
	withAct := lc.WithAction("create")

	assert.Equal(t, "create", withAct.Action)
	assert.Equal(t, "", lc.Action) // original not mutated
}

func TestLogContext_ToSlogAttrs(t *testing.T) {
	lc := LogContext{
		RequestID:     "req-123",
		TraceID:       "trace-456",
		Step:          StepUseCase,
		Resource:      "entity",
		Action:        "create",
		CallerService: "api-gateway",
	}

	attrs := lc.ToSlogAttrs()

	// attrs should be flat key-value pairs: []any{"key1", "val1", "key2", "val2", ...}
	assert.True(t, len(attrs)%2 == 0, "attrs should have even length (key-value pairs)")

	keys := make(map[string]any)
	for i := 0; i < len(attrs); i += 2 {
		key, ok := attrs[i].(string)
		assert.True(t, ok, "even-indexed elements should be string keys")
		keys[key] = attrs[i+1]
	}

	assert.Equal(t, "req-123", keys["request_id"])
	assert.Equal(t, "trace-456", keys["trace_id"])
	assert.Equal(t, StepUseCase, keys["step"])
	assert.Equal(t, "entity", keys["resource"])
	assert.Equal(t, "create", keys["action"])
	assert.Equal(t, "api-gateway", keys["caller_service"])
}

func TestLogContext_ToSlogAttrs_omitsEmptyFields(t *testing.T) {
	lc := LogContext{
		RequestID: "req-123",
		// All other fields empty
	}

	attrs := lc.ToSlogAttrs()

	// Should only contain request_id
	assert.Equal(t, 2, len(attrs)) // "request_id", "req-123"
	assert.Equal(t, "request_id", attrs[0])
	assert.Equal(t, "req-123", attrs[1])
}

func TestLogContext_ToSlogAttrs_includesExtra(t *testing.T) {
	lc := LogContext{
		RequestID: "req-123",
		Extra:     map[string]any{"custom_key": "custom_value"},
	}

	attrs := lc.ToSlogAttrs()

	keys := make(map[string]any)
	for i := 0; i < len(attrs); i += 2 {
		key, ok := attrs[i].(string)
		assert.True(t, ok)
		keys[key] = attrs[i+1]
	}

	assert.Equal(t, "custom_value", keys["custom_key"])
}

func TestErrorLogFields_DomainErrorCode(t *testing.T) {
	domainErr := errors.New("user not found")
	attrs := ErrorLogFields(domainErr, "NOT_FOUND")

	keys := make(map[string]any)
	for i := 0; i < len(attrs); i += 2 {
		key, ok := attrs[i].(string)
		assert.True(t, ok)
		keys[key] = attrs[i+1]
	}

	assert.Equal(t, "user not found", keys["error.message"])
	assert.Equal(t, "NOT_FOUND", keys["error.code"])
	_, hasStack := keys["error.stack"]
	assert.False(t, hasStack, "domain error codes should NOT have stack trace")
}

func TestErrorLogFields_InternalError(t *testing.T) {
	internalErr := errors.New("db connection failed")
	attrs := ErrorLogFields(internalErr, "")

	keys := make(map[string]any)
	for i := 0; i < len(attrs); i += 2 {
		key, ok := attrs[i].(string)
		assert.True(t, ok)
		keys[key] = attrs[i+1]
	}

	assert.Equal(t, "db connection failed", keys["error.message"])
	assert.Equal(t, "", keys["error.code"])
	_, hasStack := keys["error.stack"]
	assert.True(t, hasStack, "internal errors SHOULD have stack trace")
}

func TestErrorLogFields_UnknownCode(t *testing.T) {
	unknownErr := errors.New("something unexpected")
	attrs := ErrorLogFields(unknownErr, "UNKNOWN_CODE")

	keys := make(map[string]any)
	for i := 0; i < len(attrs); i += 2 {
		key, ok := attrs[i].(string)
		assert.True(t, ok)
		keys[key] = attrs[i+1]
	}

	assert.Equal(t, "UNKNOWN_CODE", keys["error.code"])
	_, hasStack := keys["error.stack"]
	assert.True(t, hasStack, "unknown error codes SHOULD have stack trace")
}

func TestStepConstants(t *testing.T) {
	assert.Equal(t, "handler", StepHandler)
	assert.Equal(t, "usecase", StepUseCase)
	assert.Equal(t, "repository", StepRepository)
	assert.Equal(t, "cache", StepCache)
	assert.Equal(t, "middleware", StepMiddleware)
}

func TestContextArgsFromCtx_withLogContext(t *testing.T) {
	ctx := context.Background()
	lc := LogContext{
		RequestID: "req-abc",
		Step:      StepMiddleware,
	}
	ctx = Inject(ctx, lc)

	args := contextArgsFromCtx(ctx)

	assert.NotNil(t, args)
	assert.True(t, len(args) >= 4, "should have at least request_id and step pairs")
}

func TestContextArgsFromCtx_withoutLogContext(t *testing.T) {
	ctx := context.Background()
	args := contextArgsFromCtx(ctx)
	assert.Nil(t, args)
}

func TestLogInfo(t *testing.T) {
	t.Run("with empty context", func(t *testing.T) {
		var buf bytes.Buffer
		handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
		original := slog.Default()
		slog.SetDefault(slog.New(handler))
		defer slog.SetDefault(original)

		ctx := context.Background()
		LogInfo(ctx, "test info message", "extra_key", "extra_value")

		output := buf.String()
		assert.Contains(t, output, "test info message")
		assert.Contains(t, output, "extra_key")
	})

	t.Run("with enriched context", func(t *testing.T) {
		var buf bytes.Buffer
		handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
		original := slog.Default()
		slog.SetDefault(slog.New(handler))
		defer slog.SetDefault(original)

		lc := LogContext{
			RequestID: "req-info-123",
			Step:      StepHandler,
		}
		ctx := Inject(context.Background(), lc)
		LogInfo(ctx, "enriched info message")

		output := buf.String()
		assert.Contains(t, output, "enriched info message")
		assert.Contains(t, output, "req-info-123")
	})
}

func TestLogError(t *testing.T) {
	t.Run("with empty context", func(t *testing.T) {
		var buf bytes.Buffer
		handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
		original := slog.Default()
		slog.SetDefault(slog.New(handler))
		defer slog.SetDefault(original)

		ctx := context.Background()
		LogError(ctx, "test error message", "error_key", "error_value")

		output := buf.String()
		assert.Contains(t, output, "test error message")
		assert.Contains(t, output, "error_key")
	})
}

func TestLogWarn(t *testing.T) {
	t.Run("with empty context", func(t *testing.T) {
		var buf bytes.Buffer
		handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
		original := slog.Default()
		slog.SetDefault(slog.New(handler))
		defer slog.SetDefault(original)

		ctx := context.Background()
		LogWarn(ctx, "test warn message", "warn_key", "warn_value")

		output := buf.String()
		assert.Contains(t, output, "test warn message")
		assert.Contains(t, output, "warn_key")
	})
}
