package metrics

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sachn-cs/promptsheon/internal/trace"
)

// noopTracer is a minimal in-memory tracer used so the
// middleware has a non-nil Tracer to call into. The test
// doesn't read the spans, it just exercises the
// middleware's tracer error path.
type noopTracer struct {
	finishErr error
	started   int
	finished  int
}

func (n *noopTracer) Start(ctx context.Context, op string) *trace.Span {
	n.started++
	return &trace.Span{ID: "s1", TraceID: "t1", Operation: op, Status: trace.StatusOK}
}
func (n *noopTracer) StartChild(ctx context.Context, parent *trace.Span, op string) *trace.Span {
	return &trace.Span{ID: "s2", TraceID: parent.TraceID, Operation: op, Status: trace.StatusOK}
}
func (n *noopTracer) Finish(span *trace.Span) error {
	n.finished++
	return n.finishErr
}

func TestHTTPMiddlewareRecordsRequest(t *testing.T) {
	c := NewCollector()
	tr := &noopTracer{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	calls := 0
	mw := HTTPMiddleware(c, tr, logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/prompts", nil)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	if calls != 1 {
		t.Errorf("expected handler to be called once, got %d", calls)
	}
	if c.RequestsTotal.Value() != 1 {
		t.Errorf("RequestsTotal: got %v", c.RequestsTotal.Value())
	}
	if tr.started != 1 {
		t.Errorf("tracer.Start: got %d", tr.started)
	}
}

func TestHTTPMiddlewareSkipsProbes(t *testing.T) {
	c := NewCollector()
	tr := &noopTracer{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	mw := HTTPMiddleware(c, tr, logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, path := range []string{"/health", "/ready"} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, req)
	}

	if c.RequestsTotal.Value() != 0 {
		t.Errorf("probes should not increment RequestsTotal, got %v", c.RequestsTotal.Value())
	}
}

func TestHTTPMiddlewareCountsErrors(t *testing.T) {
	c := NewCollector()
	tr := &noopTracer{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	mw := HTTPMiddleware(c, tr, logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest("GET", "/api/v1/x", nil)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	if c.ErrorsTotal.Value() != 1 {
		t.Errorf("ErrorsTotal: got %v", c.ErrorsTotal.Value())
	}
}

func TestHTTPMiddlewareLoggerReceivesFinishError(t *testing.T) {
	// When tracer.Finish returns an error, the middleware
	// logs it. We use a logger that records messages so
	// the test can assert on the call.
	var got strings.Builder
	logger := slog.New(slog.NewTextHandler(&got, &slog.HandlerOptions{Level: slog.LevelError}))
	tr := &noopTracer{finishErr: errors.New("boom")}

	mw := HTTPMiddleware(NewCollector(), tr, logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/x", nil)
	mw.ServeHTTP(httptest.NewRecorder(), req)

	if !strings.Contains(got.String(), "boom") {
		t.Errorf("expected 'boom' in log output, got %q", got.String())
	}
}

func TestLLMMiddleware(t *testing.T) {
	c := NewCollector()
	tr := &noopTracer{}

	mw := LLMMiddleware(c, tr, slog.Default())
	called := false
	inner := mw(func(operation string, req any) (any, error) {
		called = true
		return "response", nil
	})

	resp, err := inner("test", nil)
	if err != nil {
		t.Fatalf("inner: %v", err)
	}
	if !called {
		t.Error("expected inner to be called")
	}
	if resp != "response" {
		t.Errorf("response: got %v", resp)
	}
	if c.LLMCallsTotal.Value() != 1 {
		t.Errorf("LLMCallsTotal: got %v", c.LLMCallsTotal.Value())
	}
}

func TestWorkflowMiddleware(t *testing.T) {
	c := NewCollector()
	tr := &noopTracer{}

	mw := WorkflowMiddleware(c, tr)
	called := false
	inner := mw(func(agentID string, input map[string]any) (map[string]any, error) {
		called = true
		return map[string]any{"ok": true}, nil
	})

	out, err := inner("agent-1", map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("inner: %v", err)
	}
	if !called {
		t.Error("expected inner to be called")
	}
	if out["ok"] != true {
		t.Errorf("output: got %v", out)
	}
	if c.WorkflowRunsTotal.Value() != 1 {
		t.Errorf("WorkflowRunsTotal: got %v", c.WorkflowRunsTotal.Value())
	}
	if c.WorkflowActive.Value() != 0 {
		t.Errorf("WorkflowActive should be 0 after run, got %v", c.WorkflowActive.Value())
	}
}

func TestPathOnly(t *testing.T) {
	// The function strips the query string. This matters
	// because trace attributes must not persist query
	// strings that may contain API keys.
	if got := pathOnly(nil); got != "" {
		t.Errorf("nil URL: got %q", got)
	}
	u, _ := http.NewRequest("GET", "/api/v1/prompts?key=secret", nil)
	if got := pathOnly(u.URL); got != "/api/v1/prompts" {
		t.Errorf("pathOnly: got %q", got)
	}
}

func TestSpanOperationName(t *testing.T) {
	req := httptest.NewRequest("GET", "/x", nil)
	// No mux pattern, so we fall back to method + path.
	got := spanOperationName(req)
	if got != "GET /x" {
		t.Errorf("got %q", got)
	}
}

func TestMatchedRoute(t *testing.T) {
	req := httptest.NewRequest("GET", "/x", nil)
	if _, ok := matchedRoute(req); ok {
		t.Error("expected no match without a Pattern")
	}
	if got, ok := matchedRoute(nil); ok || got != "" {
		t.Errorf("nil request: got (%q, %v)", got, ok)
	}
}
