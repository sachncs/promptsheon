package webhook

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// BenchmarkDispatch exercises the webhook delivery hot path: a
// signed POST to a registered endpoint. The signing path
// (HMAC-SHA256) and JSON marshal are the dominant cost.
func BenchmarkDispatch(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := NewDispatcher(logger)
	ep := &Endpoint{
		ID:     "ep1",
		URL:    ts.URL,
		Secret: "secret-key",
		Events: []EventType{"capability.created"},
		Active: true,
	}
	d.Register(ep)

	evt := &Event{
		Type: "capability.created",
		Data: map[string]any{"id": "c1", "name": "greeting"},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.EmitContext(b.Context(), evt)
	}
}

// BenchmarkEmit exercises the publish path that fans out to all
// registered endpoints. The async fire-and-forget semantics are
// fast; this benchmark verifies the dispatch loop dominates.
func BenchmarkEmit(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := NewDispatcher(logger)
	for i := 0; i < 5; i++ {
		ep := &Endpoint{
			ID:     "ep-" + string(rune('a'+i)),
			URL:    "http://localhost:65535",
			Secret: "key",
			Events: []EventType{"*"},
			Active: true,
		}
		d.Register(ep)
	}

	evt := &Event{Type: "capability.created", Data: map[string]any{"id": "c1"}}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Emit(evt)
	}
}
