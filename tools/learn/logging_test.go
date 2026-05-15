package learn

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestNew_InjectsCmdAttribute verifies that the logger returned by New emits
// records with a "cmd" key set to the cmdName argument.
func TestNew_InjectsCmdAttribute(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf, "stats")
	logger.Info("hello", "k", "v")

	rec := map[string]any{}
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rec); err != nil {
		t.Fatalf("expected valid JSON, got %q: %v", buf.String(), err)
	}
	if got, ok := rec["cmd"].(string); !ok || got != "stats" {
		t.Errorf("expected cmd=stats, got %v", rec["cmd"])
	}
	if got, ok := rec["msg"].(string); !ok || got != "hello" {
		t.Errorf("expected msg=hello, got %v", rec["msg"])
	}
	if _, ok := rec["time"]; !ok {
		t.Errorf("expected time key in record")
	}
	if _, ok := rec["level"]; !ok {
		t.Errorf("expected level key in record")
	}
	if got, ok := rec["k"].(string); !ok || got != "v" {
		t.Errorf("expected extra attr k=v, got %v", rec["k"])
	}
}

// TestNewJSONHandler_ProducesValidJSON verifies the raw handler emits one valid
// JSON object per record.
func TestNewJSONHandler_ProducesValidJSON(t *testing.T) {
	var buf bytes.Buffer
	h := newJSONHandler(&buf)
	if h == nil {
		t.Fatalf("newJSONHandler returned nil")
	}
	logger := newLogger(&buf, "test")
	logger.Warn("be careful")
	out := strings.TrimSpace(buf.String())
	if out == "" {
		t.Fatalf("expected output, got empty")
	}
	for _, line := range strings.Split(out, "\n") {
		var v map[string]any
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			t.Errorf("line %q is not valid JSON: %v", line, err)
		}
	}
}
