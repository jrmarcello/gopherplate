package logutil

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseJSONLog parses the JSON output from slog.NewJSONHandler into a map.
func parseJSONLog(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var result map[string]any
	decodeErr := json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, decodeErr, "log output should be valid JSON: %s", buf.String())
	return result
}

// --- Handle ---

func TestMaskingHandler_Handle_masksStringAttributes(t *testing.T) {
	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, nil)
	masker := NewMasker(DefaultBRConfig())
	maskingHandler := NewMaskingHandler(masker, jsonHandler)
	logger := slog.New(maskingHandler)

	logger.Info("user action", "email", "user@example.com", "name", "Joao Silva")

	logEntry := parseJSONLog(t, &buf)
	assert.Equal(t, "u***@example.com", logEntry["email"], "email should be masked")
	assert.Equal(t, "J*** S***", logEntry["name"], "name should be masked")
	assert.Equal(t, "user action", logEntry["msg"], "message should be unchanged")
}

func TestMaskingHandler_Handle_passesNonSensitiveAttributes(t *testing.T) {
	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, nil)
	masker := NewMasker(DefaultBRConfig())
	maskingHandler := NewMaskingHandler(masker, jsonHandler)
	logger := slog.New(maskingHandler)

	logger.Info("request", "method", "GET", "path", "/api/v1/users", "status", 200)

	logEntry := parseJSONLog(t, &buf)
	assert.Equal(t, "GET", logEntry["method"], "method should pass through unchanged")
	assert.Equal(t, "/api/v1/users", logEntry["path"], "path should pass through unchanged")
	assert.Equal(t, float64(200), logEntry["status"], "status should pass through unchanged")
}

func TestMaskingHandler_Handle_skipsNonStringValues(t *testing.T) {
	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, nil)
	masker := NewMasker(DefaultBRConfig())
	maskingHandler := NewMaskingHandler(masker, jsonHandler)
	logger := slog.New(maskingHandler)

	logger.Info("test", "email", 12345, "name", true)

	logEntry := parseJSONLog(t, &buf)
	assert.Equal(t, float64(12345), logEntry["email"], "integer value for sensitive key should pass through")
	assert.Equal(t, true, logEntry["name"], "bool value for sensitive key should pass through")
}

func TestMaskingHandler_Handle_skipsEmptyStrings(t *testing.T) {
	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, nil)
	masker := NewMasker(DefaultBRConfig())
	maskingHandler := NewMaskingHandler(masker, jsonHandler)
	logger := slog.New(maskingHandler)

	logger.Info("test", "email", "", "name", "")

	logEntry := parseJSONLog(t, &buf)
	assert.Equal(t, "", logEntry["email"], "empty email should pass through unchanged")
	assert.Equal(t, "", logEntry["name"], "empty name should pass through unchanged")
}

func TestMaskingHandler_Handle_groupAttributes(t *testing.T) {
	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, nil)
	masker := NewMasker(DefaultBRConfig())
	maskingHandler := NewMaskingHandler(masker, jsonHandler)
	logger := slog.New(maskingHandler)

	logger.Info("test",
		slog.Group("user",
			slog.String("email", "user@example.com"),
			slog.String("name", "Maria Santos"),
			slog.String("role", "admin"),
		),
	)

	logEntry := parseJSONLog(t, &buf)
	userGroup, ok := logEntry["user"].(map[string]any)
	require.True(t, ok, "user should be a group/object")
	assert.Equal(t, "u***@example.com", userGroup["email"], "email inside group should be masked")
	assert.Equal(t, "M*** S***", userGroup["name"], "name inside group should be masked")
	assert.Equal(t, "admin", userGroup["role"], "non-sensitive field inside group should pass through")
}

// --- Enabled ---

func TestMaskingHandler_Enabled_delegatesToWrapped(t *testing.T) {
	tests := []struct {
		name        string
		handlerOpts *slog.HandlerOptions
		level       slog.Level
		expected    bool
	}{
		{
			name:        "info enabled for info handler",
			handlerOpts: &slog.HandlerOptions{Level: slog.LevelInfo},
			level:       slog.LevelInfo,
			expected:    true,
		},
		{
			name:        "debug disabled for info handler",
			handlerOpts: &slog.HandlerOptions{Level: slog.LevelInfo},
			level:       slog.LevelDebug,
			expected:    false,
		},
		{
			name:        "error enabled for debug handler",
			handlerOpts: &slog.HandlerOptions{Level: slog.LevelDebug},
			level:       slog.LevelError,
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonHandler := slog.NewJSONHandler(&bytes.Buffer{}, tt.handlerOpts)
			masker := NewMasker(DefaultBRConfig())
			maskingHandler := NewMaskingHandler(masker, jsonHandler)

			result := maskingHandler.Enabled(context.Background(), tt.level)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- WithAttrs ---

func TestMaskingHandler_WithAttrs_masksPreApplied(t *testing.T) {
	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, nil)
	masker := NewMasker(DefaultBRConfig())
	maskingHandler := NewMaskingHandler(masker, jsonHandler)

	withAttrs := maskingHandler.WithAttrs([]slog.Attr{
		slog.String("email", "pre@example.com"),
		slog.String("service", "my-svc"),
	})

	logger := slog.New(withAttrs)
	logger.Info("pre-applied test")

	logEntry := parseJSONLog(t, &buf)
	assert.Equal(t, "p***@example.com", logEntry["email"], "pre-applied email should be masked")
	assert.Equal(t, "my-svc", logEntry["service"], "pre-applied non-sensitive attr should pass through")
}

// --- WithGroup ---

func TestMaskingHandler_WithGroup_delegatesToWrapped(t *testing.T) {
	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, nil)
	masker := NewMasker(DefaultBRConfig())
	maskingHandler := NewMaskingHandler(masker, jsonHandler)

	withGroup := maskingHandler.WithGroup("request")
	logger := slog.New(withGroup)
	logger.Info("grouped", "key", "value")

	logEntry := parseJSONLog(t, &buf)
	requestGroup, ok := logEntry["request"].(map[string]any)
	require.True(t, ok, "request should be a group/object")
	assert.Equal(t, "value", requestGroup["key"], "attrs should appear under the group")
}

// --- Custom Masker ---

func TestMaskingHandler_customMasker(t *testing.T) {
	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, nil)

	customConfig := MaskConfig{
		Fields: map[string]MaskFunc{
			"secret_code": func(s string) string { return "REDACTED" },
			"api_key":     func(s string) string { return s[:3] + "***" },
		},
	}
	masker := NewMasker(customConfig)
	maskingHandler := NewMaskingHandler(masker, jsonHandler)
	logger := slog.New(maskingHandler)

	logger.Info("custom", "secret_code", "super-secret-123", "api_key", "abc123def", "email", "user@example.com")

	logEntry := parseJSONLog(t, &buf)
	assert.Equal(t, "REDACTED", logEntry["secret_code"], "custom field should be masked with custom function")
	assert.Equal(t, "abc***", logEntry["api_key"], "custom field should be masked with custom function")
	assert.Equal(t, "user@example.com", logEntry["email"], "email should NOT be masked (not in custom config)")
}
