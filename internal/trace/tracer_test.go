package trace

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestSpanBasics(t *testing.T) {
	span := &Span{
		ID:        "s1",
		TraceID:   "t1",
		Operation: "test",
		Service:   "test-service",
		StartedAt: time.Now(),
	}

	span.SetAttribute("key", "value")
	span.Finish()

	if span.DurationMs < 0 {
		t.Fatal("expected non-negative duration")
	}
	if span.EndedAt == nil {
		t.Fatal("expected EndedAt to be set")
	}
	if span.Attributes["key"] != "value" {
		t.Fatalf("expected attribute key=value, got %v", span.Attributes["key"])
	}
}

// TestNewSQLite_IndexesCreated pins the L-7 fix: NewSQLite must
// create the started_at index so the Since/Until range queries in
// ListSpans stay fast on large trace stores.
func TestNewSQLite_IndexesCreated(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "trace.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	if _, err := NewSQLite(db); err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	rows, err := db.QueryContext(context.Background(),
		`SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='traces'`)
	if err != nil {
		t.Fatalf("list indexes: %v", err)
	}
	defer rows.Close()
	indexes := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		indexes[name] = true
	}
	for _, want := range []string{"idx_traces_trace_id", "idx_traces_parent_id", "idx_traces_started_at"} {
		if !indexes[want] {
			t.Fatalf("missing index %q (have %v)", want, indexes)
		}
	}
	_ = os.Getenv // keep os import used in case future tests need env
}

func TestSpanError(t *testing.T) {
	span := &Span{ID: "s1", Status: StatusOK}
	span.SetError(nil)
	if span.Status != StatusError {
		t.Fatalf("expected error status, got %s", span.Status)
	}
}

func TestInMemoryTracer(t *testing.T) {
	tracer := NewInMemory()

	span := tracer.Start(context.Background(), "op-1")
	if span.Operation != "op-1" {
		t.Fatalf("expected op-1, got %s", span.Operation)
	}
	if span.TraceID == "" {
		t.Fatal("expected trace ID")
	}

	child := tracer.StartChild(context.Background(), span, "op-2")
	if child.ParentID != span.ID {
		t.Fatalf("expected parent ID %s, got %s", span.ID, child.ParentID)
	}
	if child.TraceID != span.TraceID {
		t.Fatalf("expected same trace ID, got %s", child.TraceID)
	}

	if err := tracer.Finish(span); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := tracer.Finish(child); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tracer.Spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(tracer.Spans))
	}
}

func TestContextSpanRoundtrip(t *testing.T) {
	span := &Span{ID: "ctx-test"}
	ctx := WithSpanContext(context.Background(), span)

	got, ok := SpanFromContext(ctx)
	if !ok {
		t.Fatal("expected span in context")
	}
	if got.ID != "ctx-test" {
		t.Fatalf("expected ctx-test, got %s", got.ID)
	}
}

func TestContextNoSpan(t *testing.T) {
	_, ok := SpanFromContext(context.Background())
	if ok {
		t.Fatal("expected no span in empty context")
	}
}
