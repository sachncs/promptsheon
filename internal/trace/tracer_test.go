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
	defer func() { _ = db.Close() }()
	if _, e := NewSQLite(db); e != nil {
		t.Fatalf("NewSQLite: %v", e)
	}
	rows, err := db.QueryContext(context.Background(),
		`SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='traces'`)
	if err != nil {
		t.Fatalf("list indexes: %v", err)
	}
	defer func() { _ = rows.Close() }()
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

// TestSQLiteStartChildAndFinish exercises the full
// start/finish round-trip via the SQLite tracer, then reads
// the row back through GetSpan. This is the contract the
// API handler depends on for /api/v1/traces/{id}.
func TestSQLiteStartChildAndFinish(t *testing.T) {
	db := openTestDB(t)
	s, err := NewSQLite(db)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}

	root := s.Start(context.Background(), "root")
	if root.ID == "" {
		t.Fatal("expected root span to have an ID")
	}
	if root.TraceID == "" {
		t.Fatal("expected root span to have a trace ID")
	}
	if root.Status != StatusOK {
		t.Errorf("expected status ok, got %s", root.Status)
	}

	child := s.StartChild(context.Background(), root, "child")
	if child.ParentID != root.ID {
		t.Errorf("expected parent %s, got %s", root.ID, child.ParentID)
	}
	if child.TraceID != root.TraceID {
		t.Errorf("expected same trace ID, got %s", child.TraceID)
	}

	if e := s.Finish(child); e != nil {
		t.Fatalf("Finish child: %v", e)
	}
	if e := s.Finish(root); e != nil {
		t.Fatalf("Finish root: %v", e)
	}

	got, err := s.GetSpan(context.Background(), root.ID)
	if err != nil {
		t.Fatalf("GetSpan: %v", err)
	}
	if got.Operation != "root" {
		t.Errorf("expected operation 'root', got %q", got.Operation)
	}

	// GetSpan on an unknown ID should return sql.ErrNoRows
	// (the underlying sql.ErrNoRows), which callers can use
	// to render a 404.
	if _, err := s.GetSpan(context.Background(), "nope"); err == nil {
		t.Error("expected error for unknown span id")
	}
}

func TestSQLiteListSpansFilters(t *testing.T) {
	db := openTestDB(t)
	s, err := NewSQLite(db)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}

	// Insert three spans: two with operation "a", one with
	// operation "b".
	for _, op := range []string{"a", "a", "b"} {
		span := s.Start(context.Background(), op)
		_ = s.Finish(span)
	}

	all, err := s.ListSpans(context.Background(), &SpanFilter{})
	if err != nil {
		t.Fatalf("ListSpans: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 spans, got %d", len(all))
	}

	onlyA, err := s.ListSpans(context.Background(), &SpanFilter{Operation: "a"})
	if err != nil {
		t.Fatalf("ListSpans a: %v", err)
	}
	if len(onlyA) != 2 {
		t.Errorf("expected 2 'a' spans, got %d", len(onlyA))
	}

	limited, err := s.ListSpans(context.Background(), &SpanFilter{Limit: 1})
	if err != nil {
		t.Fatalf("ListSpans limit: %v", err)
	}
	if len(limited) != 1 {
		t.Errorf("expected 1 span with limit=1, got %d", len(limited))
	}
}

func TestSQLiteListSpansTimeRange(t *testing.T) {
	db := openTestDB(t)
	s, err := NewSQLite(db)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}

	span := s.Start(context.Background(), "x")
	_ = s.Finish(span)

	since := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)

	out, err := s.ListSpans(context.Background(), &SpanFilter{Since: &since})
	if err != nil {
		t.Fatalf("ListSpans Since: %v", err)
	}
	if len(out) != 1 {
		t.Errorf("expected 1 span in recent window, got %d", len(out))
	}

	out, err = s.ListSpans(context.Background(), &SpanFilter{Until: &future})
	if err != nil {
		t.Fatalf("ListSpans Until: %v", err)
	}
	if len(out) != 1 {
		t.Errorf("expected 1 span before future cutoff, got %d", len(out))
	}

	past := time.Now().Add(-2 * time.Hour)
	out, err = s.ListSpans(context.Background(), &SpanFilter{Until: &past})
	if err != nil {
		t.Fatalf("ListSpans past Until: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected 0 spans before past cutoff, got %d", len(out))
	}
}

func TestSQLiteGetTraceTree(t *testing.T) {
	db := openTestDB(t)
	s, err := NewSQLite(db)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}

	root := s.Start(context.Background(), "root")
	childA := s.StartChild(context.Background(), root, "child-a")
	childB := s.StartChild(context.Background(), root, "child-b")
	grandchild := s.StartChild(context.Background(), childA, "grandchild")
	for _, sp := range []*Span{grandchild, childB, childA, root} {
		_ = s.Finish(sp)
	}

	spans, err := s.GetTraceTree(context.Background(), root.TraceID)
	if err != nil {
		t.Fatalf("GetTraceTree: %v", err)
	}
	if len(spans) != 4 {
		t.Errorf("expected 4 spans, got %d", len(spans))
	}

	// Unknown trace ID should return an empty slice (no rows),
	// not an error.
	empty, err := s.GetTraceTree(context.Background(), "nope")
	if err != nil {
		t.Fatalf("GetTraceTree unknown: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("expected empty slice for unknown trace, got %d spans", len(empty))
	}
}

// openTestDB returns an in-memory sqlite database. The
// database is closed when the test ends.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
