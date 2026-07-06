package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sachn-cs/promptsheon/internal/llm"
	"github.com/sachn-cs/promptsheon/internal/models"
)

// TestHandleListABTests confirms the placeholder
// implementation returns an empty list with the expected
// JSON shape.
func TestHandleListABTests(t *testing.T) {
	srv := setupTestServerMinimal(t)
	defer srv.StopAuditWorkers(context.Background())

	req := httptest.NewRequest("GET", "/api/v1/ab-tests", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["total"].(float64) != 0 {
		t.Errorf("total: got %v, want 0", body["total"])
	}
}

func TestHandleGetABTest(t *testing.T) {
	srv := setupTestServerMinimal(t)
	defer srv.StopAuditWorkers(context.Background())

	req := httptest.NewRequest("GET", "/api/v1/ab-tests/abc-123", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d", w.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["id"] != "abc-123" {
		t.Errorf("id: got %v", body["id"])
	}
}

func TestHandleStopABTest(t *testing.T) {
	srv := setupTestServerMinimal(t)
	defer srv.StopAuditWorkers(context.Background())

	req := httptest.NewRequest("POST", "/api/v1/ab-tests/abc-123/stop", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "test stopped") {
		t.Errorf("expected 'test stopped' in body, got %q", w.Body.String())
	}
}

func TestHandleGetABTestResults(t *testing.T) {
	srv := setupTestServerMinimal(t)
	defer srv.StopAuditWorkers(context.Background())

	req := httptest.NewRequest("GET", "/api/v1/ab-tests/abc-123/results", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d", w.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["test_id"] != "abc-123" {
		t.Errorf("test_id: got %v", body["test_id"])
	}
	if body["is_significant"] != false {
		t.Errorf("is_significant: got %v, want false", body["is_significant"])
	}
}

func TestHandleCreateABTestBadJSON(t *testing.T) {
	srv := setupTestServerMinimal(t)
	defer srv.StopAuditWorkers(context.Background())

	body := strings.NewReader("not json")
	req := httptest.NewRequest("POST", "/api/v1/ab-tests", body)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleCreateABTestMissingFields(t *testing.T) {
	srv := setupTestServerMinimal(t)
	defer srv.StopAuditWorkers(context.Background())

	cases := []map[string]any{
		{"name": "x"}, // missing prompt_id
		{"prompt_id": "p1"},
		{"name": "x", "prompt_id": "p1"}, // missing variants
		{"name": "x", "prompt_id": "p1", "variants": []any{"a"}}, // <2 variants
	}
	for _, c := range cases {
		bodyJSON, _ := json.Marshal(c)
		req := httptest.NewRequest("POST", "/api/v1/ab-tests", strings.NewReader(string(bodyJSON)))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("case %+v: expected 400, got %d", c, w.Code)
		}
	}
}

func TestHandleCreateABTestPromptNotFound(t *testing.T) {
	srv := setupTestServerMinimal(t)
	defer srv.StopAuditWorkers(context.Background())

	body := map[string]any{
		"name":      "test",
		"prompt_id": "missing",
		"variants":  []any{"a", "b"},
	}
	bodyJSON, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/ab-tests", strings.NewReader(string(bodyJSON)))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Without an LLM provider available (and without a
	// matching prompt row), the handler may return 404
	// (prompt not found), 400 (provider not available),
	// or 500 depending on which check fails first. We
	// just assert it's not 200.
	if w.Code == http.StatusOK {
		t.Errorf("expected non-200 for missing prompt, got 200")
	}
}

// TestRecordExecutionLogNilDB confirms the helper is safe
// when the server has no database configured.
func TestRecordExecutionLogNilDB(t *testing.T) {
	// Server without DB
	srv := &Server{}
	srv.recordExecutionLog(context.Background(), executionLogInput{
		Prompt:   &models.Prompt{ID: "p"},
		Provider: "openai",
		Model:    "gpt-4",
		Usage:    llm.Usage{TotalTokens: 1},
		Latency:  time.Millisecond,
	})
	// Reaching here without panic is the assertion.
}
