// Package trace — tracer tests (PR-7).
//
// These tests cover the in-process tracer primitives in
// promptsheon/trace/tracer.go: Span lifecycle, Tracer interface
// (noop + Multi), context propagation, ID generation. The OTel-
// specific logic lives in otel_test.go.
package trace_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/sachncs/promptsheon/promptsheon/trace"
)

// TestSpanLifecycle exercises the Finish / SetError path on a Span
// built by the noop tracer. The noop tracer is the baseline; OTel
// wrappers add semantics on top of it.
func TestSpanLifecycle(t *testing.T) {
	tr := trace.NewNoopTracer()
	span := tr.Start(context.Background(), "test-op")
	if span == nil {
		t.Fatal("Start returned nil")
	}
	if span.Operation != "test-op" {
		t.Errorf("operation = %q want %q", span.Operation, "test-op")
	}
	if span.Status != trace.StatusUnset {
		t.Errorf("initial status = %v want StatusUnset", span.Status)
	}

	if err := tr.Finish(span); err != nil {
		t.Errorf("Finish: %v", err)
	}
	// The noop tracer is intentionally minimal; status transitions
	// are the OTel tracer's responsibility. We only assert that
	// Finish does not error and does not panic.

	span2 := tr.Start(context.Background(), "err-op")
	span2.SetError(errors.New("boom"))
	if span2.Status != trace.StatusError {
		t.Errorf("post-SetError status = %v want StatusError", span2.Status)
	}
	if span2.Error != "boom" {
		t.Errorf("error message = %q want %q", span2.Error, "boom")
	}
}

// TestSpanAttributes covers SetAttribute: keys/values land in
// span.Attributes, and multiple calls accumulate.
func TestSpanAttributes(t *testing.T) {
	tr := trace.NewNoopTracer()
	span := tr.Start(context.Background(), "attrs")
	span.SetAttribute("user_id", "alice")
	span.SetAttribute("env", "test")
	if got := span.Attributes["user_id"]; got != "alice" {
		t.Errorf("user_id = %q want %q", got, "alice")
	}
	if got := span.Attributes["env"]; got != "test" {
		t.Errorf("env = %q want %q", got, "test")
	}
}

// TestContextPropagation covers With* and SpanFromContext round-trips.
// WithSpanContext attaches a *Span; SpanFromContext retrieves it.
// With{Trace,Request,User}ID stores ID-like values retrievable by
// IDFromContext.
func TestContextPropagation(t *testing.T) {
	ctx := context.Background()
	span := trace.NewNoopTracer().Start(ctx, "ctx-test")
	ctx = trace.WithSpanContext(ctx, span)
	got, ok := trace.SpanFromContext(ctx)
	if !ok || got != span {
		t.Errorf("SpanFromContext: ok=%v got=%v want=%v", ok, got, span)
	}

	// IDFromContext reads TraceIDContextKey; RequestIDFromContext
	// reads RequestIDContextKey. They are distinct accessors.
	ctxTrace := trace.WithTraceID(context.Background(), "trace-1")
	if id, ok := trace.IDFromContext(ctxTrace); !ok || id != "trace-1" {
		t.Errorf("TraceID: ok=%v id=%q", ok, id)
	}

	ctxReq := trace.WithRequestID(context.Background(), "req-1")
	if id, ok := trace.RequestIDFromContext(ctxReq); !ok || id != "req-1" {
		t.Errorf("RequestID: ok=%v id=%q", ok, id)
	}

	ctx = trace.WithUserID(ctx, "user-1")
	if id, ok := trace.UserIDFromContext(ctx); !ok || id != "user-1" {
		t.Errorf("UserID: ok=%v id=%q", ok, id)
	}

	// Missing keys return ok=false.
	if _, ok := trace.UserIDFromContext(context.Background()); ok {
		t.Errorf("missing UserID: ok=true want=false")
	}
}

// TestGenerateID covers the collision-safe ID generator: two calls
// produce distinct values, and IDs are non-empty.
func TestGenerateID(t *testing.T) {
	a := trace.GenerateID()
	b := trace.GenerateID()
	if a == "" || b == "" {
		t.Fatalf("GenerateID returned empty: a=%q b=%q", a, b)
	}
	if a == b {
		t.Errorf("GenerateID returned same value twice: %q", a)
	}
	// Two IDs should differ; if the format is "span-<unixnano>-<counter>"
	// they may share a prefix. Use a Contains-style check on length.
	if len(a) < 5 {
		t.Errorf("GenerateID returned suspiciously short value: %q (len=%d)", a, len(a))
	}
}

// recordingTracer is a minimal Tracer impl used to verify Multi
// behaviour: it records every Finish call.
type recordingTracer struct {
	mu       sync.Mutex
	finished []*trace.Span
	err      error // returned by Finish
}

func (r *recordingTracer) Start(_ context.Context, op string) *trace.Span {
	return &trace.Span{Operation: op}
}
func (r *recordingTracer) StartChild(_ context.Context, parent *trace.Span, op string) *trace.Span {
	if parent != nil {
		return &trace.Span{Operation: op, ParentID: parent.ID}
	}
	return &trace.Span{Operation: op}
}
func (r *recordingTracer) Finish(s *trace.Span) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.finished = append(r.finished, s)
	return r.err
}
func (r *recordingTracer) Flush(_ context.Context) error { return nil }

// TestMulti_Finish exercises Multi.Finish: the primary tracer's
// error wins; subsequent tracers are still attempted.
func TestMulti_Finish(t *testing.T) {
	primaryErr := errors.New("primary failed")
	primary := &recordingTracer{err: primaryErr}
	secondary := &recordingTracer{}
	multi := trace.NewMulti(primary, secondary)

	span := multi.Start(context.Background(), "multi-finish")
	if err := multi.Finish(span); !errors.Is(err, primaryErr) {
		t.Errorf("Finish err = %v want primaryErr", err)
	}
	// Both tracers saw the span.
	if len(primary.finished) != 1 {
		t.Errorf("primary.finished len = %d want 1", len(primary.finished))
	}
	if len(secondary.finished) != 1 {
		t.Errorf("secondary.finished len = %d want 1", len(secondary.finished))
	}
}

// TestMulti_Flush exercises Multi.Flush: every underlying tracer
// is flushed; the first error wins.
func TestMulti_Flush(t *testing.T) {
	primary := &recordingTracer{}
	secondary := &recordingTracer{}
	multi := trace.NewMulti(primary, secondary)

	if err := multi.Flush(context.Background()); err != nil {
		t.Errorf("Flush: %v", err)
	}
}

// TestMulti_StartChild covers the parent/child linkage path.
func TestMulti_StartChild(t *testing.T) {
	primary := &recordingTracer{}
	multi := trace.NewMulti(primary)

	parent := multi.Start(context.Background(), "parent")
	parent.ID = "parent-id"
	child := multi.StartChild(context.Background(), parent, "child")
	if child.ParentID != "parent-id" {
		t.Errorf("child.ParentID = %q want %q", child.ParentID, "parent-id")
	}
	if child.Operation != "child" {
		t.Errorf("child.Operation = %q want %q", child.Operation, "child")
	}
}

// TestNoopTracerFlush covers the noop Flush path (returns nil).
func TestNoopTracerFlush(t *testing.T) {
	tr := trace.NewNoopTracer()
	if err := tr.Flush(context.Background()); err != nil {
		t.Errorf("Flush: %v", err)
	}
	// Start on noop returns a non-nil span with the operation.
	span := tr.Start(context.Background(), "noop")
	if span == nil || span.Operation != "noop" {
		t.Errorf("noop Start: span=%v", span)
	}
	if err := tr.Finish(span); err != nil {
		t.Errorf("noop Finish: %v", err)
	}
}

// TestSpanStatusStrings covers the status string serialization.
func TestSpanStatusStrings(t *testing.T) {
	// StatusUnset is the empty string by design (a fresh span
	// has no status). The other two are non-empty.
	if string(trace.StatusUnset) != "" {
		t.Errorf("StatusUnset = %q want empty", string(trace.StatusUnset))
	}
	if string(trace.StatusOK) == "" || string(trace.StatusError) == "" {
		t.Errorf("StatusOK/StatusError should be non-empty")
	}
}
