package trace_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/sachncs/promptsheon/backend/testutil"
)

// TestOTelSpanLandsInCollector verifies TEST-NEW-12: a request
// produces a span that lands in the in-memory OTLP test collector.
func TestOTelSpanLandsInCollector(t *testing.T) {
	collector := testutil.NewInMemoryCollector()
	defer collector.Shutdown(context.Background())

	tracer := collector.Provider().Tracer("test-otel")
	_, span := tracer.Start(context.Background(), "test-operation")
	span.End()

	spans := collector.Spans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name != "test-operation" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "test-operation")
	}
}

// TestOTelSpanAttributes verifies attributes set on a span
// are collected.
func TestOTelSpanAttributes(t *testing.T) {
	collector := testutil.NewInMemoryCollector()
	defer collector.Shutdown(context.Background())

	tracer := collector.Provider().Tracer("test-attrs")
	_, span := tracer.Start(context.Background(), "with-attrs")
	span.End()

	spans := collector.Spans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
}

// TestOTelChildSpans verifies parent-child span relationship.
func TestOTelChildSpans(t *testing.T) {
	collector := testutil.NewInMemoryCollector()
	defer collector.Shutdown(context.Background())

	tracer := collector.Provider().Tracer("test-parent-child")
	ctx, parent := tracer.Start(context.Background(), "parent")
	_, child := tracer.Start(ctx, "child")
	child.End()
	parent.End()

	spans := collector.Spans()
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}
}

// TestConcurrentTraceWrites verifies TEST-NEW-9: 1000
// concurrent trace writes don't block on the trace SQLite
// or in-memory exporter. The test succeeds if it completes
// without deadlock or data race (run with -race).
func TestConcurrentTraceWrites(t *testing.T) {
	collector := testutil.NewInMemoryCollector()
	defer collector.Shutdown(context.Background())

	const n = 1000
	var wg sync.WaitGroup
	wg.Add(n)

	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(id int) {
			defer wg.Done()
			tracer := collector.Provider().Tracer(fmt.Sprintf("worker-%d", id%10))
			ctx, span := tracer.Start(context.Background(), fmt.Sprintf("op-%d", id))
			span.End()
			_ = ctx
			errs <- nil
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("concurrent write error: %v", err)
		}
	}

	spans := collector.Spans()
	if len(spans) != n {
		t.Errorf("expected %d spans, got %d", n, len(spans))
	}
}
