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

// BenchmarkHashAndTeeBody pins PERF-7b: hashing a 100MB body
// must stay O(64 bytes) in memory. The implementation spools
// to a temp file so the only heap allocations are the 32KB
// copy buffer and the SHA-256 state. Allocations per op are
// O(1) regardless of body size.
func BenchmarkHashAndTeeBody(b *testing.B) {
	const size = 100 * 1024 * 1024
	body := make([]byte, size)
	for i := range body {
		body[i] = byte(i % 256)
	}
	b.SetBytes(int64(size))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rc := readCloser{Reader: bytes.NewReader(body), Closer: closingNoop{}}
		hash, replay, err := hashAndTeeBody(rc)
		if err != nil {
			b.Fatal(err)
		}
		if hash == "" {
			b.Fatal("empty hash")
		}
		_ = replay.Close()
	}
}

type closingNoop struct{}

func (closingNoop) Close() error { return nil }

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
