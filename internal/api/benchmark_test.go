package api

import (
	"bytes"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/sachncs/promptsheon/internal/llm"
)

// BenchmarkHandleHealth exercises the smallest possible handler
// path: GET /health. Use to track the per-request overhead of the
// middleware chain (Recovery, Logging, RequestID, Security) that
// runs in front of every handler.
func BenchmarkHandleHealth(b *testing.B) {
	s := newBenchServer(b)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		s.mux.ServeHTTP(w, req)
	}
}

// BenchmarkHandleGetWorkspaceObservation exercises a handler that
// reads from the rollup aggregator. The aggregate path is the
// hot path for the observability surface; tracking it keeps
// regressions visible.
func BenchmarkHandleGetWorkspaceObservation(b *testing.B) {
	s := newBenchServer(b)

	req := httptest.NewRequest("GET", "/api/v1/workspaces/w1/observation", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		s.mux.ServeHTTP(w, req)
	}
}

// newBenchServer builds a Server suitable for benchmarking. It
// uses an in-memory mock repo and the error-level logger writing
// to a discard buffer so logging noise does not contaminate
// allocation counts.
func newBenchServer(b *testing.B) *Server {
	b.Helper()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))
	providers := llm.NewRegistry()
	providers.Configure("openai", llm.ProviderConfig{APIKey: "sk-test"})
	repo := newMockRepo()
	return NewServer(newRepositories(repo), logger, WithProviders(providers))
}
