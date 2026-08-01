package promptsheon

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sachncs/promptsheon/promptsheon/capability"
)

func TestHandleListExecutions(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/versions/v1/executions", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestHandleCreateExecution exercises the production invoke
// path end-to-end. newInvokeTestServer wires a real
// invoke.Invoker backed by an in-memory LLM provider, so the
// handler actually runs the request and persists a real
// Execution row with deterministic token counts (1 prompt / 1
// completion token / $0.01 cost).

func TestHandleCreateExecution(t *testing.T) {
	s := newInvokeTestServer(t)
	body := mustMarshal(t, map[string]any{
		"inputs":   map[string]any{"query": "hello"},
		"model":    "gpt-4",
		"provider": "openai",
	})
	req := httptest.NewRequest("POST", "/api/v1/versions/v1/executions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["model"] != "gpt-4" {
		t.Errorf("expected gpt-4, got %v", resp["model"])
	}
	if got, want := resp["total_tokens"].(float64), float64(2); got != want {
		t.Errorf("expected total_tokens=%v, got %v", want, got)
	}
	if got, want := resp["cost_usd"].(float64), 0.01; got != want {
		t.Errorf("expected cost_usd=%v, got %v", want, got)
	}
}

func TestHandleGetExecution(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateExecution(context.Background(), &capability.Execution{ID: "e1", CapabilityVersionID: "v1"})
	s := newTestServer(t)
	s.db = newDB(repo)

	req := httptest.NewRequest("GET", "/api/v1/executions/e1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetExecution_NotFound(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/executions/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Rate Limiting Tests
// ---------------------------------------------------------------------------
