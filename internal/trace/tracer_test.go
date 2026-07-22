package trace

import (
	"context"
	"testing"
)

// TestContextNoSpan confirms SpanFromContext returns nil when no
// span is attached to the context. The OBS-TR-1 cleanup removed
// the SQLite tracer; the in-memory span context path remains.
func TestContextNoSpan(t *testing.T) {
	if _, ok := SpanFromContext(context.Background()); ok {
		t.Error("expected nil from empty context")
	}
}
