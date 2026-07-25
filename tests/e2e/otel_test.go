package e2e

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/jrmarcello/gopherplate/internal/bootstrap"
	"github.com/jrmarcello/gopherplate/internal/infrastructure/web/middleware"
	"github.com/jrmarcello/gopherplate/internal/infrastructure/web/router"
	"github.com/jrmarcello/gopherplate/pkg/cache"
)

// setupTracedTestRouter wires a gin.Engine equivalent to the production router
// (otelgin.Middleware -> SpanRename -> domain routes) against the shared
// TestContainers DB/cache fixtures, but with an in-memory span exporter bound
// as the global TracerProvider for the duration of the test. The previous
// TracerProvider is restored via t.Cleanup so other tests are unaffected.
//
// Scope: this helper exists purely to assert end-to-end trace shape for
// TC-E2E-01 / TC-E2E-02. It deliberately does not replicate every production
// middleware (metrics, auth, idempotency, body limit) — those are covered by
// the rest of the E2E suite.
func setupTracedTestRouter(t *testing.T, db *sqlx.DB, cacheClient cache.Cache) (*gin.Engine, *tracetest.InMemoryExporter) {
	t.Helper()

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))

	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = tp.Shutdown(shutdownCtx)
		otel.SetTracerProvider(prev)
	})

	c := bootstrap.NewForTest(t, db, cacheClient)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.CustomRecovery())

	// Production ordering: otelgin creates the root span, then SpanRename
	// renames it according to HTTPSpanName(method, route) after c.Next().
	r.Use(otelgin.Middleware("e2e-test", otelgin.WithTracerProvider(tp)))
	r.Use(middleware.SpanRename())

	group := r.Group("")
	router.RegisterRoleRoutes(group, c.Handlers.Role)
	router.RegisterUserRoutes(group, c.Handlers.User)

	return r, exporter
}

// findSpanByName returns the first SpanStub whose name exactly matches target
// and a bool indicating whether a match was found.
func findSpanByName(spans tracetest.SpanStubs, target string) (tracetest.SpanStub, bool) {
	for _, s := range spans {
		if s.Name == target {
			return s, true
		}
	}
	return tracetest.SpanStub{}, false
}

// spanNames returns just the names of the given spans, for diagnostic output
// in failing assertions.
func spanNames(spans tracetest.SpanStubs) []string {
	names := make([]string, len(spans))
	for i, s := range spans {
		names[i] = s.Name
	}
	return names
}

// TC-E2E-01 — REQ-3: POST /users succeeds and produces a root HTTP span named
// `http.post.users` with a `db.insert.users` child span underneath it. Asserts
// the production wiring (otelgin + SpanRename + StartDBSpan) end-to-end.
func TestE2E_OTel_PostUsers_SpanNames(t *testing.T) {
	require.NoError(t, CleanupUsers())
	r, exporter := setupTracedTestRouter(t, GetTestDB(), GetTestCache())

	body := `{
		"name": "OTel Test User",
		"email": "otel-post@example.com"
	}`

	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())

	spans := exporter.GetSpans()
	require.NotEmpty(t, spans, "expected at least one exported span")

	httpSpan, ok := findSpanByName(spans, "http.post.users")
	require.True(t, ok, "expected root span http.post.users; got names=%v", spanNames(spans))

	dbSpan, ok := findSpanByName(spans, "db.insert.users")
	require.True(t, ok, "expected child span db.insert.users; got names=%v", spanNames(spans))

	// The DB span must share the HTTP span's trace and declare a parent —
	// this is the end-to-end evidence that StartDBSpan opens under otelgin's
	// root span rather than creating a detached trace.
	assert.Equal(t, httpSpan.SpanContext.TraceID(), dbSpan.SpanContext.TraceID(),
		"db span must share trace with http span")
	assert.True(t, dbSpan.Parent.IsValid(), "db span must have a parent span context")
}

// TC-E2E-02 — REQ-1, REQ-2: GET /users/<non-existent> returns 404 AND the
// expected-error path leaves the root span status NOT Error, with
// `app.result=not_found` recorded as an attribute somewhere in the trace.
// This is the defining behavior of ADR-009's WarnSpan classification and the
// whole point of REQ-2: expected errors are observable without being alarming.
func TestE2E_OTel_GetUserNotFound_ClassifiedAsWarn(t *testing.T) {
	r, exporter := setupTracedTestRouter(t, GetTestDB(), GetTestCache())

	// Syntactically valid UUID v7 that is guaranteed not to exist.
	req := httptest.NewRequest(http.MethodGet, "/users/018e4a2c-6b4d-7000-9410-abcdef999999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())

	spans := exporter.GetSpans()
	require.NotEmpty(t, spans)

	httpSpan, ok := findSpanByName(spans, "http.get.users_by_id")
	require.True(t, ok, "expected root span http.get.users_by_id; got names=%v", spanNames(spans))

	// The root span must NOT be marked as Error — not-found is an expected
	// outcome (REQ-2). otelgin sets codes.Error for 5xx but leaves 4xx alone,
	// and no middleware in the chain should promote this to Error.
	assert.NotEqual(t, codes.Error, httpSpan.Status.Code,
		"root span must not be marked Error for expected not-found path; got code=%v desc=%q",
		httpSpan.Status.Code, httpSpan.Status.Description)

	// `app.result=not_found` is attached by shared.ClassifyError via WarnSpan.
	// It can live on any span in the trace (typically the use-case/internal
	// span created by otelgin or a nested handler span). If it is never
	// recorded, classification is broken.
	foundResultAttr := false
	var seenAttrs []string
	for _, s := range spans {
		for _, kv := range s.Attributes {
			if string(kv.Key) == "app.result" {
				seenAttrs = append(seenAttrs, s.Name+":"+kv.Value.String())
				if kv.Value.AsString() == "not_found" {
					foundResultAttr = true
				}
			}
		}
	}
	assert.True(t, foundResultAttr,
		"expected app.result=not_found attribute on some span; seen app.result values=%v, span names=%v",
		seenAttrs, spanNames(spans))
}

// TC-E2E-03 — REQ-1: FailSpan path (error.type + stack trace attributes) is
// intentionally NOT asserted at the E2E layer. The TestContainers harness
// does not expose a supported fault-injection surface (can't inject network
// timeouts or broken connections without racing the teardown), so the unit
// layer is the authoritative coverage for this path:
//
//   - TC-UC-32 in internal/usecases/user/create_test.go drives the use case
//     with a mocked repository that returns a network error and asserts
//     FailSpan enriches the span with `error.type` + stack trace attributes.
//
// Keeping this stub keeps the TC-ID discoverable and the rationale in-tree.
func TestE2E_OTel_DBErrorPath_FailSpan(t *testing.T) {
	t.Skip("TC-E2E-03: covered by TC-UC-32 in internal/usecases/user/create_test.go — TestContainers harness has no fault-injection surface for DB errors")
}
