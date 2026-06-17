package trace

import (
	"context"
	"testing"
	"time"
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
