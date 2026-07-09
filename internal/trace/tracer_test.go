package trace

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

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

// --- Context key functions ---

func TestContextKeys(t *testing.T) {
	ctx := context.Background()

	// WithTraceID / IDFromContext
	ctx = WithTraceID(ctx, "trace-42")
	id, ok := IDFromContext(ctx)
	if !ok {
		t.Fatal("expected trace ID in context")
	}
	if id != "trace-42" {
		t.Errorf("expected 'trace-42', got %q", id)
	}

	// WithRequestID / RequestIDFromContext
	ctx = WithRequestID(ctx, "req-99")
	rid, ok := RequestIDFromContext(ctx)
	if !ok {
		t.Fatal("expected request ID in context")
	}
	if rid != "req-99" {
		t.Errorf("expected 'req-99', got %q", rid)
	}

	// WithUserID / UserIDFromContext
	ctx = WithUserID(ctx, "user-abc")
	uid, ok := UserIDFromContext(ctx)
	if !ok {
		t.Fatal("expected user ID in context")
	}
	if uid != "user-abc" {
		t.Errorf("expected 'user-abc', got %q", uid)
	}

	// Empty context returns false / zero value
	_, ok = IDFromContext(context.Background())
	if ok {
		t.Fatal("expected no trace ID in empty context")
	}
	_, ok = RequestIDFromContext(context.Background())
	if ok {
		t.Fatal("expected no request ID in empty context")
	}
	_, ok = UserIDFromContext(context.Background())
	if ok {
		t.Fatal("expected no user ID in empty context")
	}
}

// --- Span SetError with actual error ---

func TestSpanSetErrorWithErr(t *testing.T) {
	span := &Span{ID: "s1", Status: StatusOK}
	err := errors.New("something broke")
	span.SetError(err)
	if span.Status != StatusError {
		t.Fatalf("expected StatusError, got %s", span.Status)
	}
	if span.Error != "something broke" {
		t.Fatalf("expected 'something broke', got %q", span.Error)
	}
}

func TestSpanSetAttributeNilMap(t *testing.T) {
	span := &Span{ID: "s1"}
	span.SetAttribute("a", "b")
	if span.Attributes == nil {
		t.Fatal("expected Attributes map to be initialized")
	}
	if span.Attributes["a"] != "b" {
		t.Fatalf("expected 'b', got %q", span.Attributes["a"])
	}

	// Set on already-initialized map
	span.SetAttribute("c", "d")
	if span.Attributes["c"] != "d" {
		t.Fatalf("expected 'd', got %q", span.Attributes["c"])
	}
}

// --- OTel tracer tests ---

func newOTelTestTracer(t *testing.T) (*OTelTracer, *sdktrace.TracerProvider) {
	t.Helper()
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	return NewOTelTracer("test-service"), tp
}

func TestNewOTelTracer(t *testing.T) {
	tracer, tp := newOTelTestTracer(t)
	defer tp.Shutdown(context.Background())

	if tracer == nil {
		t.Fatal("expected non-nil tracer")
	}
	if tracer.serviceName != "test-service" {
		t.Errorf("expected 'test-service', got %q", tracer.serviceName)
	}
	if tracer.tracer == nil {
		t.Error("expected non-nil OTel tracer")
	}
}

func TestOTelTracerStart(t *testing.T) {
	tracer, tp := newOTelTestTracer(t)
	defer tp.Shutdown(context.Background())

	span := tracer.Start(context.Background(), "test-op")
	if span == nil {
		t.Fatal("expected non-nil span")
	}
	if span.Operation != "test-op" {
		t.Errorf("expected 'test-op', got %q", span.Operation)
	}
	if span.ID == "" {
		t.Error("expected non-empty span ID")
	}
	if span.TraceID == "" {
		t.Error("expected non-empty trace ID")
	}
	if span.Service != "test-service" {
		t.Errorf("expected 'test-service', got %q", span.Service)
	}
	if span.Status != StatusUnset {
		t.Errorf("expected StatusUnset, got %s", span.Status)
	}
	if span.StartedAt.IsZero() {
		t.Error("expected non-zero StartedAt")
	}
}

func TestOTelTracerStartChild(t *testing.T) {
	tracer, tp := newOTelTestTracer(t)
	defer tp.Shutdown(context.Background())

	parent := &Span{
		ID:        "parent-id",
		TraceID:   "trace-id",
		Operation: "parent-op",
		Service:   "test-service",
		Status:    StatusUnset,
		StartedAt: time.Now(),
	}

	child := tracer.StartChild(context.Background(), parent, "child-op")
	if child == nil {
		t.Fatal("expected non-nil child span")
	}
	if child.Operation != "child-op" {
		t.Errorf("expected 'child-op', got %q", child.Operation)
	}
	if child.ParentID != "parent-id" {
		t.Errorf("expected parent 'parent-id', got %q", child.ParentID)
	}
	if child.ID == "" {
		t.Error("expected non-empty child span ID")
	}
}

func TestOTelTracerStartChildNilParent(t *testing.T) {
	tracer, tp := newOTelTestTracer(t)
	defer tp.Shutdown(context.Background())

	// When parent is nil, StartChild falls back to Start (root span)
	span := tracer.StartChild(context.Background(), nil, "orphan")
	if span == nil {
		t.Fatal("expected non-nil span")
	}
	if span.Operation != "orphan" {
		t.Errorf("expected 'orphan', got %q", span.Operation)
	}
	if span.ParentID != "" {
		t.Errorf("expected empty parent for root span, got %q", span.ParentID)
	}
}

func TestOTelTracerFinish(t *testing.T) {
	tracer, tp := newOTelTestTracer(t)
	defer tp.Shutdown(context.Background())

	// nil span is a no-op
	if err := tracer.Finish(nil); err != nil {
		t.Errorf("expected nil error for nil span, got %v", err)
	}

	span := tracer.Start(context.Background(), "finish-me")
	if err := tracer.Finish(span); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if span.EndedAt == nil {
		t.Error("expected EndedAt to be set")
	}
	if span.DurationMs < 0 {
		t.Errorf("expected non-negative duration, got %d", span.DurationMs)
	}
}

func TestOTelTracerFinishWithError(t *testing.T) {
	tracer, tp := newOTelTestTracer(t)
	defer tp.Shutdown(context.Background())

	// nil span is a no-op
	if err := tracer.FinishWithError(nil, nil); err != nil {
		t.Errorf("expected nil error for nil span, got %v", err)
	}

	// Finish with no error
	span := tracer.Start(context.Background(), "ok")
	if err := tracer.FinishWithError(span, nil); err != nil {
		t.Fatalf("FinishWithError nil: %v", err)
	}
	if span.EndedAt == nil {
		t.Error("expected EndedAt to be set after FinishWithError")
	}

	// Finish with an error
	span2 := tracer.Start(context.Background(), "fail")
	err := errors.New("oops")
	if err := tracer.FinishWithError(span2, err); err != nil {
		t.Fatalf("FinishWithError err: %v", err)
	}
	if span2.Status != StatusError {
		t.Errorf("expected StatusError, got %s", span2.Status)
	}
	if span2.Error != "oops" {
		t.Errorf("expected 'oops', got %q", span2.Error)
	}
	if span2.EndedAt == nil {
		t.Error("expected EndedAt to be set")
	}
}

func TestOTelTracerRecordSpan(t *testing.T) {
	tracer, tp := newOTelTestTracer(t)
	defer tp.Shutdown(context.Background())

	// Nil arguments should not panic
	tracer.RecordSpan(nil, nil)
	tracer.RecordSpan(nil, &Span{ID: "s1"})

	_, otelSpan := tracer.tracer.Start(context.Background(), "record-normal")
	span := &Span{
		ID:         "s1",
		Attributes: map[string]string{"key1": "val1", "key2": "val2"},
	}
	tracer.RecordSpan(otelSpan, span)

	// Record with error
	_, otelSpan2 := tracer.tracer.Start(context.Background(), "record-error")
	span2 := &Span{
		ID:         "s2",
		Attributes: map[string]string{"k": "v"},
		Error:      "something failed",
	}
	tracer.RecordSpan(otelSpan2, span2)
}

func TestOTelAttributeString(t *testing.T) {
	kv := otelAttributeString("my-key", "my-value")
	if kv.Key != "my-key" {
		t.Errorf("expected key 'my-key', got %q", kv.Key)
	}
	var _ attribute.KeyValue = kv
}

// --- Exporter tests ---

func TestNoopExporter(t *testing.T) {
	e := newnoopExporter()
	if e == nil {
		t.Fatal("expected non-nil exporter")
	}

	if err := e.ExportSpans(context.Background(), nil); err != nil {
		t.Errorf("ExportSpans: %v", err)
	}

	if err := e.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
}

func TestInitTracerProviderNoop(t *testing.T) {
	tp, err := InitTracerProvider("test-svc", "", false)
	if err != nil {
		t.Fatalf("InitTracerProvider: %v", err)
	}
	if tp == nil {
		t.Fatal("expected non-nil TracerProvider")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := tp.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
}

// --- OTel StartChild with OTel span in context ---

func TestOTelTracerStartChild_WithOTelSpanInContext(t *testing.T) {
	tracer, tp := newOTelTestTracer(t)
	defer tp.Shutdown(context.Background())

	ctx, otelSpan := tracer.tracer.Start(context.Background(), "parent-otel")
	ctx = context.WithValue(ctx, otelSpanKey{}, otelSpan)

	parent := &Span{ID: "parent-id"}
	child := tracer.StartChild(ctx, parent, "child")
	if child.ParentID != "parent-id" {
		t.Errorf("expected 'parent-id', got %q", child.ParentID)
	}
	if child.TraceID == "" {
		t.Error("expected non-empty trace ID")
	}
}

// --- SQLite error / edge-case tests ---

func TestNewSQLite_ClosedDB(t *testing.T) {
	db := openTestDB(t)
	db.Close()

	_, err := NewSQLite(db)
	if err == nil {
		t.Fatal("expected error with closed DB")
	}
}

func TestSQLiteFinish_ClosedDB(t *testing.T) {
	db := openTestDB(t)
	s, err := NewSQLite(db)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	db.Close()

	span := s.Start(context.Background(), "test")
	err = s.Finish(span)
	if err == nil {
		t.Fatal("expected error with closed DB")
	}
}

func TestSQLiteListSpans_Error(t *testing.T) {
	db := openTestDB(t)
	s, err := NewSQLite(db)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	db.Close()

	_, err = s.ListSpans(context.Background(), &SpanFilter{})
	if err == nil {
		t.Fatal("expected error with closed DB")
	}
}

func TestSQLiteGetTraceTree_Error(t *testing.T) {
	db := openTestDB(t)
	s, err := NewSQLite(db)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	db.Close()

	_, err = s.GetTraceTree(context.Background(), "trace-1")
	if err == nil {
		t.Fatal("expected error with closed DB")
	}
}

func TestScanSpan_InvalidJSON(t *testing.T) {
	db := openTestDB(t)
	s, err := NewSQLite(db)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}

	// Insert a row with unparseable JSON in the attributes column
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO traces (id, trace_id, parent_id, operation, service, status, attributes, started_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"bad-json", "trace-x", "", "op", "svc", "ok", "{invalid json}", time.Now())
	if err != nil {
		t.Fatalf("insert invalid json: %v", err)
	}

	span, err := s.GetSpan(context.Background(), "bad-json")
	if err != nil {
		t.Fatalf("GetSpan: %v", err)
	}
	if span.ID != "bad-json" {
		t.Errorf("expected 'bad-json', got %q", span.ID)
	}
	if span.Attributes != nil {
		t.Errorf("expected nil attributes for unparseable json, got %v", span.Attributes)
	}
}

// --- Additional SQLite tests ---

func TestSQLiteFinishWithAttributes(t *testing.T) {
	db := openTestDB(t)
	s, err := NewSQLite(db)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}

	span := s.Start(context.Background(), "attr-test")
	span.SetAttribute("color", "blue")
	span.SetAttribute("size", "large")
	if err := s.Finish(span); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	got, err := s.GetSpan(context.Background(), span.ID)
	if err != nil {
		t.Fatalf("GetSpan: %v", err)
	}
	if got.Attributes["color"] != "blue" {
		t.Errorf("expected color=blue, got %v", got.Attributes["color"])
	}
	if got.Attributes["size"] != "large" {
		t.Errorf("expected size=large, got %v", got.Attributes["size"])
	}
	if got.Operation != "attr-test" {
		t.Errorf("expected 'attr-test', got %q", got.Operation)
	}
}

func TestSQLiteListSpansExtraFilters(t *testing.T) {
	db := openTestDB(t)
	s, err := NewSQLite(db)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}

	span1 := s.Start(context.Background(), "op1")
	span1.Service = "service-a"
	span1.Status = StatusOK
	_ = s.Finish(span1)

	span2 := s.Start(context.Background(), "op2")
	span2.Service = "service-b"
	span2.Status = StatusError
	_ = s.Finish(span2)

	// Filter by service
	out, err := s.ListSpans(context.Background(), &SpanFilter{Service: "service-a"})
	if err != nil {
		t.Fatalf("ListSpans service: %v", err)
	}
	if len(out) != 1 {
		t.Errorf("expected 1 span for service-a, got %d", len(out))
	}

	// Filter by status
	out, err = s.ListSpans(context.Background(), &SpanFilter{Status: StatusError})
	if err != nil {
		t.Fatalf("ListSpans status: %v", err)
	}
	if len(out) != 1 {
		t.Errorf("expected 1 span with error status, got %d", len(out))
	}

	// Filter by traceID
	out, err = s.ListSpans(context.Background(), &SpanFilter{TraceID: span1.TraceID})
	if err != nil {
		t.Fatalf("ListSpans traceID: %v", err)
	}
	if len(out) != 1 {
		t.Errorf("expected 1 span for traceID, got %d", len(out))
	}

	// Combined filters with no match
	out, err = s.ListSpans(context.Background(), &SpanFilter{Service: "service-a", Status: StatusError})
	if err != nil {
		t.Fatalf("ListSpans combined: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected 0 spans for combined filter, got %d", len(out))
	}
}
