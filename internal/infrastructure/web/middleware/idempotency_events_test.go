package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/jrmarcello/gopherplate/pkg/idempotency"
)

var errStoreDown = stderrors.New("redis unreachable")

// newTracedRouterForIdempotency mounts otelgin.Middleware + Idempotency against
// an in-memory span recorder. Returns the engine, the recorder, and a cleanup
// that restores the previous tracer provider.
func newTracedRouterForIdempotency(t *testing.T, store *mockStore) (*gin.Engine, *tracetest.InMemoryExporter) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	exporter := tracetest.NewInMemoryExporter()
	prevTP := otel.GetTracerProvider()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.ForceFlush(context.Background())
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(prevTP)
	})

	r := gin.New()
	r.Use(otelgin.Middleware("idempotency-test"))
	r.Use(Idempotency(store))
	return r, exporter
}

func postJSON(t *testing.T, r *gin.Engine, key string, body map[string]any, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	r.POST("/tc", handler)
	payload, marshalErr := json.Marshal(body)
	require.NoError(t, marshalErr)
	req := httptest.NewRequest(http.MethodPost, "/tc", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func flushSpans(t *testing.T, exp *tracetest.InMemoryExporter) []tracetest.SpanStub {
	t.Helper()
	return exp.GetSpans()
}

// fingerprintOf returns the SHA-256 the middleware computes for a JSON body.
// Test helper: matches bodyFingerprint() in idempotency.go.
func fingerprintOf(body map[string]any) string {
	payload, _ := json.Marshal(body)
	return bodyFingerprint(payload)
}

func findEvent(spans []tracetest.SpanStub, name string) (sdktrace.Event, bool) {
	for _, s := range spans {
		for _, ev := range s.Events {
			if ev.Name == name {
				return ev, true
			}
		}
	}
	return sdktrace.Event{}, false
}

func eventAttr(ev sdktrace.Event, key string) string {
	for _, attr := range ev.Attributes {
		if string(attr.Key) == key {
			return attr.Value.String()
		}
	}
	return ""
}

// TC-UC-50: first POST with Idempotency-Key emits idempotency.key_acquired.
func TestIdempotency_Event_KeyAcquired(t *testing.T) {
	store := newMockStore()
	r, exp := newTracedRouterForIdempotency(t, store)

	w := postJSON(t, r, "acq-key", map[string]any{"name": "x"}, func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"id": 1})
	})
	assert.Equal(t, http.StatusCreated, w.Code)

	ev, found := findEvent(flushSpans(t, exp), "idempotency.key_acquired")
	require.True(t, found, "expected idempotency.key_acquired event")
	v := eventAttr(ev, "idempotency.key")
	assert.Equal(t, "acq-key", v)
}

// TC-UC-51: replay of a completed entry emits idempotency.replayed with
// status_code and does NOT use logutil.LogInfo.
func TestIdempotency_Event_Replayed(t *testing.T) {
	store := newMockStore()
	store.entries["idempotency:replay-key"] = &idempotency.Entry{
		Status:      idempotency.StatusCompleted,
		StatusCode:  http.StatusCreated,
		Body:        []byte(`{"id":1}`),
		Fingerprint: fingerprintOf(map[string]any{"name": "same"}),
	}

	r, exp := newTracedRouterForIdempotency(t, store)
	w := postJSON(t, r, "replay-key", map[string]any{"name": "same"}, func(c *gin.Context) {
		t.Fatalf("handler must not be invoked on replay")
	})
	_ = w

	ev, found := findEvent(flushSpans(t, exp), "idempotency.replayed")
	require.True(t, found, "expected idempotency.replayed event")
	key := eventAttr(ev, "idempotency.key")
	code := eventAttr(ev, "idempotency.status_code")
	assert.Equal(t, "replay-key", key)
	assert.Equal(t, "201", code)
}

// TC-UC-52: existing entry in Processing state emits idempotency.locked.
func TestIdempotency_Event_Locked(t *testing.T) {
	store := newMockStore()
	store.entries["idempotency:locked-key"] = &idempotency.Entry{Status: idempotency.StatusProcessing}

	r, exp := newTracedRouterForIdempotency(t, store)
	w := postJSON(t, r, "locked-key", map[string]any{}, func(c *gin.Context) {
		t.Fatalf("handler must not run when another request is processing")
	})
	assert.Equal(t, http.StatusConflict, w.Code)

	_, found := findEvent(flushSpans(t, exp), "idempotency.locked")
	assert.True(t, found, "expected idempotency.locked event")
}

// TC-UC-53: replay with different body fingerprint emits
// idempotency.fingerprint_mismatch (no logutil.LogWarn).
func TestIdempotency_Event_FingerprintMismatch(t *testing.T) {
	store := newMockStore()
	store.entries["idempotency:fp-key"] = &idempotency.Entry{
		Status:      idempotency.StatusCompleted,
		StatusCode:  http.StatusCreated,
		Body:        []byte(`{"id":1}`),
		Fingerprint: "original-fingerprint",
	}

	r, exp := newTracedRouterForIdempotency(t, store)
	w := postJSON(t, r, "fp-key", map[string]any{"different": "body"}, func(c *gin.Context) {
		t.Fatalf("handler must not run")
	})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	_, found := findEvent(flushSpans(t, exp), "idempotency.fingerprint_mismatch")
	assert.True(t, found, "expected idempotency.fingerprint_mismatch event")
}

// TC-UC-54: successful 2xx response emits idempotency.stored.
func TestIdempotency_Event_Stored(t *testing.T) {
	store := newMockStore()
	r, exp := newTracedRouterForIdempotency(t, store)

	w := postJSON(t, r, "stored-key", map[string]any{}, func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"id": 42})
	})
	assert.Equal(t, http.StatusCreated, w.Code)

	ev, found := findEvent(flushSpans(t, exp), "idempotency.stored")
	require.True(t, found, "expected idempotency.stored event")
	code := eventAttr(ev, "idempotency.status_code")
	assert.Equal(t, "201", code)
}

// TC-UC-55: 5xx response emits idempotency.released (lock unlocked for retry).
func TestIdempotency_Event_Released(t *testing.T) {
	store := newMockStore()
	r, exp := newTracedRouterForIdempotency(t, store)

	_ = postJSON(t, r, "released-key", map[string]any{}, func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "boom"})
	})

	_, found := findEvent(flushSpans(t, exp), "idempotency.released")
	assert.True(t, found, "expected idempotency.released event on 5xx")
}

// TC-UC-56: Store.Lock returns infra error — logutil.LogWarn IS still called
// (emergency-path policy kept) AND span event idempotency.store_unavailable
// is also emitted for trace correlation.
func TestIdempotency_Event_StoreUnavailable(t *testing.T) {
	store := newMockStore()
	store.lockErr = errStoreDown
	r, exp := newTracedRouterForIdempotency(t, store)

	_ = postJSON(t, r, "unavail-key", map[string]any{"x": 1}, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	_, found := findEvent(flushSpans(t, exp), "idempotency.store_unavailable")
	assert.True(t, found, "expected idempotency.store_unavailable event on Lock error")
}
