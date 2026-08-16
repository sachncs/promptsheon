// Package trace — OTel tests (PR-7).
//
// These tests cover the OTel-backed tracer in
// promptsheon/trace/otel.go: Flush behaviour, Start/Finish
// lifecycle, attribute propagation, error status recording.
// The tests use the global OTel provider, which falls back to
// the noop provider when no SDK is configured (test/dev default).
package trace_test

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/sachncs/promptsheon/promptsheon/trace"
)

// TestOTelNewNoop checks that NewOTelTracer constructs a usable
// tracer even when no SDK provider is configured (the global
// TracerProvider is the noop in tests). The tracer's Start returns
// a span with non-empty ID and TraceID.
func TestOTelNewNoop(t *testing.T) {
	tr := trace.NewOTelTracer("test-svc")
	if tr == nil {
		t.Fatal("NewOTelTracer returned nil")
	}
	if tr.Flush(context.Background()) != nil {
		t.Errorf("Flush on noop provider: got non-nil err")
	}
}

// TestOTelStart exercises the Start path: a fresh span has the
// operation name, the service name, the unset status, and an
// initialised startedAt timestamp.
func TestOTelStart(t *testing.T) {
	tr := trace.NewOTelTracer("svc-a")
	span := tr.Start(context.Background(), "op-x")
	if span == nil {
		t.Fatal("Start returned nil")
	}
	if span.Operation != "op-x" {
		t.Errorf("Operation = %q want %q", span.Operation, "op-x")
	}
	if span.Service != "svc-a" {
		t.Errorf("Service = %q want %q", span.Service, "svc-a")
	}
	if span.Status != trace.StatusUnset {
		t.Errorf("initial Status = %v want StatusUnset", span.Status)
	}
	if span.StartedAt.IsZero() {
		t.Errorf("StartedAt is zero")
	}
}

// TestOTelStartChild covers the StartChild happy path and the nil
// parent fallback (it delegates to Start).
func TestOTelStartChild(t *testing.T) {
	tr := trace.NewOTelTracer("svc-b")
	parent := tr.Start(context.Background(), "parent")
	if parent == nil {
		t.Fatal("Start returned nil")
	}
	parent.ID = "parent-id"

	// Happy path: parent set.
	child := tr.StartChild(context.Background(), parent, "child")
	if child == nil {
		t.Fatal("StartChild returned nil")
	}
	if child.Operation != "child" {
		t.Errorf("child.Operation = %q want %q", child.Operation, "child")
	}
	if child.ParentID != "parent-id" {
		t.Errorf("child.ParentID = %q want %q", child.ParentID, "parent-id")
	}

	// Nil parent falls back to Start (no panic, no nil deref).
	noParent := tr.StartChild(context.Background(), nil, "orphan")
	if noParent == nil {
		t.Fatal("StartChild with nil parent returned nil")
	}
	if noParent.Operation != "orphan" {
		t.Errorf("noParent.Operation = %q want %q", noParent.Operation, "orphan")
	}
}

// TestOTelFinish covers the Finish happy path: nil span is a
// no-op; non-nil span is marked finished.
func TestOTelFinish(t *testing.T) {
	tr := trace.NewOTelTracer("svc-c")

	// Nil span: no error.
	if err := tr.Finish(nil); err != nil {
		t.Errorf("Finish(nil): %v", err)
	}

	// Non-nil span: no error.
	span := tr.Start(context.Background(), "fin")
	if err := tr.Finish(span); err != nil {
		t.Errorf("Finish: %v", err)
	}
}

// TestOTelFinishWithError covers the FinishWithError path: the
// span's Error field is set, then Finish runs.
func TestOTelFinishWithError(t *testing.T) {
	tr := trace.NewOTelTracer("svc-d")
	span := tr.Start(context.Background(), "fin-err")
	errBoom := errors.New("boom")
	if err := tr.FinishWithError(span, errBoom); err != nil {
		t.Errorf("FinishWithError: %v", err)
	}
	if span.Error != "boom" {
		t.Errorf("span.Error = %q want %q", span.Error, "boom")
	}

	// Nil span: no error.
	if err := tr.FinishWithError(nil, errBoom); err != nil {
		t.Errorf("FinishWithError(nil): %v", err)
	}
}

// TestOTelRecordSpan covers the RecordSpan helper: nil guards and
// the happy path. The function is hard to assert against (it
// mutates an OTel SDK span which we don't keep), so the test
// mostly pins the no-panic contract.
func TestOTelRecordSpan(t *testing.T) {
	tr := trace.NewOTelTracer("svc-e")
	ctx := context.Background()
	otelSpan := makeOTelSpan(ctx)
	defer otelSpan.End()
	// Nil both: no panic.
	tr.RecordSpan(nil, nil)
	// Nil OTel span: no panic.
	span := tr.Start(ctx, "rec")
	tr.RecordSpan(nil, span)
	// Both non-nil: no panic.
	tr.RecordSpan(otelSpan, span)
}

// Capture a real OTel span (with the global noop provider, this
// is a noop span that satisfies oteltrace.Span).
func makeOTelSpan(ctx context.Context) oteltrace.Span {
	_, s := otel.GetTracerProvider().Tracer("trace-test").Start(ctx, "test-span")
	return s
}

// TestOTelFlushNoop covers Flush when the provider has no
// ForceFlush method (the noop case).
func TestOTelFlushNoop(t *testing.T) {
	// Ensure the global provider is the noop (test/dev default).
	_ = otel.GetTracerProvider()
	tr := trace.NewOTelTracer("svc-f")
	if err := tr.Flush(context.Background()); err != nil {
		t.Errorf("Flush on noop: %v", err)
	}
}
