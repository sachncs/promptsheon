package tests

import (
	"context"
	"testing"

	. "github.com/sachncs/promptsheon/promptsheon/trace"
)

// RunContextNoSpan confirms SpanFromContext returns nil when no
// span is attached to the context. The OBS-TR-1 cleanup removed
// the SQLite tracer; the in-memory span context path remains.
func RunContextNoSpan(t *testing.T) {
	if _, ok := SpanFromContext(context.Background()); ok {
		t.Error("expected nil from empty context")
	}
}
