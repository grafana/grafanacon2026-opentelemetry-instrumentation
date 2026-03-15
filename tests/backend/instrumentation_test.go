package backend_test

import (
	"context"
	"os"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"

	dbpkg "github.com/workshop/tapas-backend/db"
)

// setupTraceRecorder installs a TracerProvider backed by an in-memory
// SpanRecorder and restores the previous provider when the test ends.
func setupTraceRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return recorder
}

// instrumentedDB wraps the shared testDB's underlying *sql.DB with a fresh
// OTel instrumentation layer. This lets each test install its own providers
// before calling NewDB so the tracer is captured from the right provider.
func instrumentedDB(t *testing.T) *dbpkg.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DB_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5433/tapas?sslmode=disable"
	}
	db, err := dbpkg.NewDB(testDB.DB, dsn)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	return db
}

func TestQueryContextCreatesSpan(t *testing.T) {
	recorder := setupTraceRecorder(t)
	db := instrumentedDB(t)

	query := "SELECT id FROM restaurants LIMIT 1"
	rows, err := db.QueryContext(context.Background(), query)
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}
	rows.Close()

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]

	if span.Name() != "SELECT restaurants" {
		t.Errorf("span name: want %q, got %q", "SELECT restaurants", span.Name())
	}
	if span.SpanKind() != trace.SpanKindClient {
		t.Errorf("span kind: want Client, got %s", span.SpanKind())
	}

	attrs := attrMap(span.Attributes())
	assertAttr(t, attrs, semconv.DBSystemNameKey.String("postgresql"))
	assertAttr(t, attrs, semconv.ServerAddressKey.String("localhost"))
	assertAttr(t, attrs, semconv.ServerPortKey.Int(5433))
	assertAttr(t, attrs, semconv.DBNamespaceKey.String("tapas"))
	assertAttr(t, attrs, semconv.DBQuerySummaryKey.String("SELECT restaurants"))
	assertAttr(t, attrs, semconv.DBQueryTextKey.String(query))
}

func TestQueryRowContextCreatesSpan(t *testing.T) {
	recorder := setupTraceRecorder(t)
	db := instrumentedDB(t)

	query := "SELECT COUNT(*) FROM restaurants"
	var count int
	err := db.QueryRowContext(context.Background(), query).Scan(&count)
	if err != nil {
		t.Fatalf("QueryRowContext: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]

	if span.Name() != "SELECT restaurants" {
		t.Errorf("span name: want %q, got %q", "SELECT restaurants", span.Name())
	}
	if span.SpanKind() != trace.SpanKindClient {
		t.Errorf("span kind: want Client, got %s", span.SpanKind())
	}

	attrs := attrMap(span.Attributes())
	assertAttr(t, attrs, semconv.DBSystemNameKey.String("postgresql"))
	assertAttr(t, attrs, semconv.ServerAddressKey.String("localhost"))
	assertAttr(t, attrs, semconv.ServerPortKey.Int(5433))
	assertAttr(t, attrs, semconv.DBNamespaceKey.String("tapas"))
	assertAttr(t, attrs, semconv.DBQuerySummaryKey.String("SELECT restaurants"))
	assertAttr(t, attrs, semconv.DBQueryTextKey.String(query))
}

func TestExecContextCreatesSpan(t *testing.T) {
	recorder := setupTraceRecorder(t)
	db := instrumentedDB(t)

	// A no-op update that matches nothing — just exercises ExecContext.
	query := "UPDATE restaurants SET updated_at = updated_at WHERE id = $1"
	_, err := db.ExecContext(context.Background(), query, "nonexistent-id")
	if err != nil {
		t.Fatalf("ExecContext: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]

	if span.Name() != "UPDATE restaurants" {
		t.Errorf("span name: want %q, got %q", "UPDATE restaurants", span.Name())
	}
	if span.SpanKind() != trace.SpanKindClient {
		t.Errorf("span kind: want Client, got %s", span.SpanKind())
	}

	attrs := attrMap(span.Attributes())
	assertAttr(t, attrs, semconv.DBSystemNameKey.String("postgresql"))
	assertAttr(t, attrs, semconv.ServerAddressKey.String("localhost"))
	assertAttr(t, attrs, semconv.ServerPortKey.Int(5433))
	assertAttr(t, attrs, semconv.DBNamespaceKey.String("tapas"))
	assertAttr(t, attrs, semconv.DBQuerySummaryKey.String("UPDATE restaurants"))
	assertAttr(t, attrs, semconv.DBQueryTextKey.String(query))
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func attrMap(attrs []attribute.KeyValue) map[attribute.Key]attribute.Value {
	m := make(map[attribute.Key]attribute.Value, len(attrs))
	for _, a := range attrs {
		m[a.Key] = a.Value
	}
	return m
}

func assertAttr(t *testing.T, attrs map[attribute.Key]attribute.Value, want attribute.KeyValue) {
	t.Helper()
	got, ok := attrs[want.Key]
	if !ok {
		t.Errorf("attribute %q missing", want.Key)
		return
	}
	if got != want.Value {
		t.Errorf("attribute %q: want %q, got %q", want.Key, want.Value.AsString(), got.AsString())
	}
}
