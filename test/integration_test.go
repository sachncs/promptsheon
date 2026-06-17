package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"promptsheon/internal/api"
	"promptsheon/internal/eval"
	"promptsheon/internal/llm"
	"promptsheon/internal/metrics"
	"promptsheon/internal/store"
	"promptsheon/internal/trace"
)

// setupServer creates a test HTTP server backed by an in-memory SQLite database.
func setupServer(t *testing.T) *httptest.Server {
	t.Helper()
	db, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	logger := slog.Default()
	mockProvider := llm.NewMock("test response")
	evalRunner := eval.NewRunner(mockProvider, eval.ExactMatchScorer{})
	spans, err := trace.NewSQLite(db.DB())
	if err != nil {
		t.Fatalf("failed to create trace store: %v", err)
	}
	collector := metrics.NewCollector()

	srv := api.NewServer(db, logger, api.WithEvalRunner(evalRunner), api.WithTracing(spans, collector))
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts
}

// --- Tests ---

func TestIntegration_HealthEndpoint(t *testing.T) {
	ts := setupServer(t)
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestIntegration_PromptCRUD(t *testing.T) {
	ts := setupServer(t)

	// Create
	createBody := `{"name":"test-prompt","content":"You are a {{role}}. {{instruction}}","tags":["test"]}`
	resp, err := http.Post(ts.URL+"/api/v1/prompts", "application/json", bytes.NewBufferString(createBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", resp.StatusCode)
	}
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created) //nolint:errcheck
	promptID := created["id"].(string)

	// Read
	resp2, err := http.Get(ts.URL + "/api/v1/prompts/" + promptID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", resp2.StatusCode)
	}

	// List
	resp3, err := http.Get(ts.URL + "/api/v1/prompts")
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", resp3.StatusCode)
	}

	// Update
	updateBody := `{"name":"test-prompt-v2","content":"You are a {{role}}. Be concise.","tags":["test","v2"]}`
	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/prompts/"+promptID, bytes.NewBufferString(updateBody))
	req.Header.Set("Content-Type", "application/json")
	resp4, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("update: expected 200, got %d", resp4.StatusCode)
	}

	// Delete
	req5, _ := http.NewRequest("DELETE", ts.URL+"/api/v1/prompts/"+promptID, nil)
	resp5, err := http.DefaultClient.Do(req5)
	if err != nil {
		t.Fatal(err)
	}
	defer resp5.Body.Close()
	if resp5.StatusCode != http.StatusNoContent && resp5.StatusCode != http.StatusOK {
		t.Fatalf("delete: expected 200/204, got %d", resp5.StatusCode)
	}

	// Verify deleted
	resp6, err := http.Get(ts.URL + "/api/v1/prompts/" + promptID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp6.Body.Close()
	if resp6.StatusCode != http.StatusNotFound {
		t.Fatalf("get deleted: expected 404, got %d", resp6.StatusCode)
	}
}

func TestIntegration_DatasetCRUD(t *testing.T) {
	ts := setupServer(t)

	// Create
	createBody := `{"name":"test-dataset","cases":[{"input":{"text":"hello"},"expected_output":"world"}]}`
	resp, err := http.Post(ts.URL+"/api/v1/datasets", "application/json", bytes.NewBufferString(createBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", resp.StatusCode)
	}
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created) //nolint:errcheck
	datasetID := created["id"].(string)

	// Read
	resp2, err := http.Get(ts.URL + "/api/v1/datasets/" + datasetID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", resp2.StatusCode)
	}

	// List
	resp3, err := http.Get(ts.URL + "/api/v1/datasets")
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", resp3.StatusCode)
	}

	// Update
	updateBody := `{"name":"test-dataset-v2","description":"updated"}`
	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/datasets/"+datasetID, bytes.NewBufferString(updateBody))
	req.Header.Set("Content-Type", "application/json")
	resp4, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("update: expected 200, got %d", resp4.StatusCode)
	}

	// Delete
	req5, _ := http.NewRequest("DELETE", ts.URL+"/api/v1/datasets/"+datasetID, nil)
	resp5, err := http.DefaultClient.Do(req5)
	if err != nil {
		t.Fatal(err)
	}
	defer resp5.Body.Close()
	if resp5.StatusCode != http.StatusNoContent && resp5.StatusCode != http.StatusOK {
		t.Fatalf("delete: expected 200/204, got %d", resp5.StatusCode)
	}
}

func TestIntegration_AgentCRUD(t *testing.T) {
	ts := setupServer(t)

	// Create
	createBody := `{"name":"test-agent","description":"integration test agent","steps":[{"id":"step1","output_key":"out1"}]}`
	resp, err := http.Post(ts.URL+"/api/v1/agents", "application/json", bytes.NewBufferString(createBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", resp.StatusCode)
	}
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created) //nolint:errcheck
	agentID := created["id"].(string)

	// Read
	resp2, err := http.Get(ts.URL + "/api/v1/agents/" + agentID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", resp2.StatusCode)
	}

	// List
	resp3, err := http.Get(ts.URL + "/api/v1/agents")
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", resp3.StatusCode)
	}

	// Delete
	req, _ := http.NewRequest("DELETE", ts.URL+"/api/v1/agents/"+agentID, nil)
	resp4, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusNoContent && resp4.StatusCode != http.StatusOK {
		t.Fatalf("delete: expected 200/204, got %d", resp4.StatusCode)
	}
}

func TestIntegration_FullWorkflow(t *testing.T) {
	ts := setupServer(t)

	// 1. Create a prompt
	createPrompt := `{"name":"workflow-prompt","content":"Translate this to French: {{text}}","tags":["workflow"]}`
	resp, err := http.Post(ts.URL+"/api/v1/prompts", "application/json", bytes.NewBufferString(createPrompt))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var prompt map[string]any
	json.NewDecoder(resp.Body).Decode(&prompt) //nolint:errcheck
	promptID := prompt["id"].(string)

	// 2. Create a dataset
	createDataset := `{"name":"workflow-dataset","cases":[{"input":{"text":"hello"},"expected_output":"bonjour"},{"input":{"text":"goodbye"},"expected_output":"au revoir"}]}`
	resp2, err := http.Post(ts.URL+"/api/v1/datasets", "application/json", bytes.NewBufferString(createDataset))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var dataset map[string]any
	json.NewDecoder(resp2.Body).Decode(&dataset) //nolint:errcheck
	datasetID := dataset["id"].(string)

	// 3. Run eval
	createEval := fmt.Sprintf(`{"prompt_hash":"test-hash-123","dataset_id":"%s","scorers":["exact_match"],"prompt_text":"Translate to French: {{text}}"}`, datasetID)
	resp3, err := http.Post(ts.URL+"/api/v1/eval/run", "application/json", bytes.NewBufferString(createEval))
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("eval run: expected 200, got %d", resp3.StatusCode)
	}

	// 4. List eval results
	resp4, err := http.Get(ts.URL + "/api/v1/eval/results?prompt_hash=test-hash-123")
	if err != nil {
		t.Fatal(err)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("eval results: expected 200, got %d", resp4.StatusCode)
	}

	// 5. Create a review
	createReview := `{"resource_id":"` + promptID + `","resource_type":"prompt","author":"tester"}`
	resp5, err := http.Post(ts.URL+"/api/v1/reviews", "application/json", bytes.NewBufferString(createReview))
	if err != nil {
		t.Fatal(err)
	}
	defer resp5.Body.Close()
	if resp5.StatusCode != http.StatusCreated {
		t.Fatalf("create review: expected 201, got %d", resp5.StatusCode)
	}
	var review map[string]any
	json.NewDecoder(resp5.Body).Decode(&review) //nolint:errcheck
	reviewID := review["id"].(string)

	// 6. Add comment
	addComment := `{"content":"Looks good!"}`
	resp6, err := http.Post(ts.URL+"/api/v1/reviews/"+reviewID+"/comment", "application/json", bytes.NewBufferString(addComment))
	if err != nil {
		t.Fatal(err)
	}
	defer resp6.Body.Close()
	if resp6.StatusCode != http.StatusOK {
		t.Fatalf("add comment: expected 200, got %d", resp6.StatusCode)
	}

	// 7. Approve review
	req7, _ := http.NewRequest("PUT", ts.URL+"/api/v1/reviews/"+reviewID+"/approve", nil)
	resp7, err := http.DefaultClient.Do(req7)
	if err != nil {
		t.Fatal(err)
	}
	defer resp7.Body.Close()
	if resp7.StatusCode != http.StatusOK {
		t.Fatalf("approve review: expected 200, got %d", resp7.StatusCode)
	}

	// 8. Create an agent for workflow
	createAgent := `{"name":"workflow-agent","description":"test agent","steps":[{"id":"step1","output_key":"result1","tool_calls":[{"tool":"json_transform","input":{"expression":"$.data"}}]}],"tools":[{"name":"json_transform","type":"json_transform","config":{}}]}`
	resp8a, err := http.Post(ts.URL+"/api/v1/agents", "application/json", bytes.NewBufferString(createAgent))
	if err != nil {
		t.Fatal(err)
	}
	defer resp8a.Body.Close()
	var agent map[string]any
	json.NewDecoder(resp8a.Body).Decode(&agent) //nolint:errcheck
	agentID := agent["id"].(string)

	// 9. Run a workflow
	createWorkflow := fmt.Sprintf(`{"agent_id":"%s","input":{"data":"hello"}}`, agentID)
	resp8, err := http.Post(ts.URL+"/api/v1/workflows/run", "application/json", bytes.NewBufferString(createWorkflow))
	if err != nil {
		t.Fatal(err)
	}
	defer resp8.Body.Close()
	if resp8.StatusCode != http.StatusOK {
		t.Fatalf("run workflow: expected 200, got %d", resp8.StatusCode)
	}

	// 10. List workflows
	resp9, err := http.Get(ts.URL + "/api/v1/workflows")
	if err != nil {
		t.Fatal(err)
	}
	defer resp9.Body.Close()
	if resp9.StatusCode != http.StatusOK {
		t.Fatalf("list workflows: expected 200, got %d", resp9.StatusCode)
	}

	// 11. Check audit log
	resp10, err := http.Get(ts.URL + "/api/v1/audit")
	if err != nil {
		t.Fatal(err)
	}
	defer resp10.Body.Close()
	if resp10.StatusCode != http.StatusOK {
		t.Fatalf("list audit: expected 200, got %d", resp10.StatusCode)
	}

	// 12. Verify prompt still exists
	resp11, err := http.Get(ts.URL + "/api/v1/prompts/" + promptID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp11.Body.Close()
	if resp11.StatusCode != http.StatusOK {
		t.Fatalf("final prompt check: expected 200, got %d", resp11.StatusCode)
	}
}

func TestIntegration_ConcurrentRequests(t *testing.T) {
	ts := setupServer(t)

	done := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		go func(n int) {
			defer func() { done <- true }()
			body := fmt.Sprintf(`{"name":"concurrent-%d","content":"test content %d","tags":["concurrent"]}`, n, n)
			resp, err := http.Post(ts.URL+"/api/v1/prompts", "application/json", bytes.NewBufferString(body))
			if err != nil {
				t.Errorf("goroutine %d: %v", n, err)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusCreated {
				t.Errorf("goroutine %d: expected 201, got %d", n, resp.StatusCode)
			}
		}(i)
	}

	for i := 0; i < 20; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for concurrent requests")
		}
	}

	// Verify all were created
	resp, err := http.Get(ts.URL + "/api/v1/prompts")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result []map[string]any
	json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck
	if len(result) != 20 {
		t.Fatalf("expected 20 prompts, got %d", len(result))
	}
}

func TestIntegration_MetricsEndpoint(t *testing.T) {
	ts := setupServer(t)

	// Generate some traffic
	for i := 0; i < 5; i++ {
		http.Get(ts.URL + "/health") //nolint:errcheck
	}

	resp, err := http.Get(ts.URL + "/api/v1/metrics/summary")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metrics summary: expected 200, got %d", resp.StatusCode)
	}
}

func TestIntegration_AgentFork(t *testing.T) {
	ts := setupServer(t)

	// Create original agent
	createBody := `{"name":"original-agent","description":"test","steps":[{"id":"step1","output_key":"out1"}],"tags":["test"]}`
	resp, err := http.Post(ts.URL+"/api/v1/agents", "application/json", bytes.NewBufferString(createBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var agent map[string]any
	json.NewDecoder(resp.Body).Decode(&agent) //nolint:errcheck
	agentID := agent["id"].(string)

	// Fork agent
	forkBody := `{"name":"forked-agent"}`
	resp2, err := http.Post(ts.URL+"/api/v1/agents/"+agentID+"/fork", "application/json", bytes.NewBufferString(forkBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("fork: expected 201, got %d", resp2.StatusCode)
	}
	var forked map[string]any
	json.NewDecoder(resp2.Body).Decode(&forked) //nolint:errcheck
	if forked["parent_id"] != agentID {
		t.Fatalf("expected parent_id %s, got %v", agentID, forked["parent_id"])
	}
	if forked["name"] != "forked-agent" {
		t.Fatalf("expected name 'forked-agent', got %v", forked["name"])
	}
}

func TestIntegration_ListTemplates(t *testing.T) {
	ts := setupServer(t)

	// Create a template agent (need to create then update to set is_template)
	createBody := `{"name":"template-agent","description":"template","steps":[{"id":"step1","output_key":"out1"}]}`
	resp, err := http.Post(ts.URL+"/api/v1/agents", "application/json", bytes.NewBufferString(createBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var agent map[string]any
	json.NewDecoder(resp.Body).Decode(&agent) //nolint:errcheck

	// List templates (should be empty since is_template defaults to false)
	resp2, err := http.Get(ts.URL + "/api/v1/agents/templates")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("list templates: expected 200, got %d", resp2.StatusCode)
	}
}

func TestIntegration_SimilarPrompts(t *testing.T) {
	ts := setupServer(t)

	// Create prompts with similar content
	http.Post(ts.URL+"/api/v1/prompts", "application/json", bytes.NewBufferString(`{"name":"p1","content":"You are a helpful AI assistant. Please answer questions clearly."}`)) //nolint:errcheck
	http.Post(ts.URL+"/api/v1/prompts", "application/json", bytes.NewBufferString(`{"name":"p2","content":"You are a helpful AI assistant. Please answer questions clearly and concisely."}`)) //nolint:errcheck
	http.Post(ts.URL+"/api/v1/prompts", "application/json", bytes.NewBufferString(`{"name":"p3","content":"Completely unrelated content about quantum physics and wormholes."}`)) //nolint:errcheck

	// Find similar
	resp, err := http.Get(ts.URL + "/api/v1/prompts/similar?content=You%20are%20a%20helpful%20AI%20assistant&threshold=0.6")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("similar: expected 200, got %d", resp.StatusCode)
	}
}

func TestIntegration_ReviewGatedStatus(t *testing.T) {
	ts := setupServer(t)

	// Create prompt
	resp, err := http.Post(ts.URL+"/api/v1/prompts", "application/json", bytes.NewBufferString(`{"name":"gated-prompt","content":"test content"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var prompt map[string]any
	json.NewDecoder(resp.Body).Decode(&prompt) //nolint:errcheck
	promptID := prompt["id"].(string)

	// Try to approve directly (should fail)
	updateBody := `{"status":"approved"}`
	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/prompts/"+promptID, bytes.NewBufferString(updateBody))
	req.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for direct approve, got %d", resp2.StatusCode)
	}

	// Create review
	reviewBody := `{"resource_id":"` + promptID + `","resource_type":"prompt","author":"reviewer"}`
	resp3, err := http.Post(ts.URL+"/api/v1/reviews", "application/json", bytes.NewBufferString(reviewBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	var review map[string]any
	json.NewDecoder(resp3.Body).Decode(&review) //nolint:errcheck
	reviewID := review["id"].(string)

	// Approve review (should auto-approve the prompt)
	req4, _ := http.NewRequest("PUT", ts.URL+"/api/v1/reviews/"+reviewID+"/approve", nil)
	resp4, err := http.DefaultClient.Do(req4)
	if err != nil {
		t.Fatal(err)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("approve review: expected 200, got %d", resp4.StatusCode)
	}

	// Verify prompt is now approved
	resp5, err := http.Get(ts.URL + "/api/v1/prompts/" + promptID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp5.Body.Close()
	var updated map[string]any
	json.NewDecoder(resp5.Body).Decode(&updated) //nolint:errcheck
	if updated["status"] != "approved" {
		t.Fatalf("expected prompt status 'approved', got %v", updated["status"])
	}
}

func TestIntegration_PromptDeployArchive(t *testing.T) {
	ts := setupServer(t)

	// Create prompt
	body := `{"name":"deploy-test","content":"test content","variables":[]}`
	resp, err := http.Post(ts.URL+"/api/v1/prompts", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var p map[string]any
	json.NewDecoder(resp.Body).Decode(&p) //nolint:errcheck
	id := p["id"].(string)

	// Approve via review
	reviewBody := `{"resource_id":"` + id + `","resource_type":"prompt","author":"reviewer"}`
	resp2, _ := http.Post(ts.URL+"/api/v1/reviews", "application/json", bytes.NewBufferString(reviewBody))
	var review map[string]any
	json.NewDecoder(resp2.Body).Decode(&review) //nolint:errcheck
	resp2.Body.Close()
	reviewID := review["id"].(string)
	req2, _ := http.NewRequest("PUT", ts.URL+"/api/v1/reviews/"+reviewID+"/approve", nil)
	resp3, _ := http.DefaultClient.Do(req2)
	resp3.Body.Close()

	// Deploy
	req4, _ := http.NewRequest("POST", ts.URL+"/api/v1/prompts/"+id+"/deploy", nil)
	resp4, err := http.DefaultClient.Do(req4)
	if err != nil {
		t.Fatal(err)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("deploy: expected 200, got %d", resp4.StatusCode)
	}
	var deployed map[string]any
	json.NewDecoder(resp4.Body).Decode(&deployed) //nolint:errcheck
	if deployed["status"] != "deployed" {
		t.Fatalf("expected status 'deployed', got %v", deployed["status"])
	}

	// Archive
	req5, _ := http.NewRequest("POST", ts.URL+"/api/v1/prompts/"+id+"/archive", nil)
	resp5, err := http.DefaultClient.Do(req5)
	if err != nil {
		t.Fatal(err)
	}
	defer resp5.Body.Close()
	if resp5.StatusCode != http.StatusOK {
		t.Fatalf("archive: expected 200, got %d", resp5.StatusCode)
	}
	var archived map[string]any
	json.NewDecoder(resp5.Body).Decode(&archived) //nolint:errcheck
	if archived["status"] != "archived" {
		t.Fatalf("expected status 'archived', got %v", archived["status"])
	}
}

func TestIntegration_AuditExport(t *testing.T) {
	ts := setupServer(t)

	// Create a prompt to generate audit entries
	body := `{"name":"audit-test","content":"test","variables":[]}`
	resp, err := http.Post(ts.URL+"/api/v1/prompts", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Wait for async audit write
	time.Sleep(100 * time.Millisecond)

	// Export as JSON
	resp2, err := http.Get(ts.URL + "/api/v1/audit/export?format=json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
	var entries []map[string]any
	json.NewDecoder(resp2.Body).Decode(&entries) //nolint:errcheck
	if len(entries) == 0 {
		t.Fatal("expected at least one audit entry")
	}

	// Verify hash fields exist
	entry := entries[0]
	if _, ok := entry["entry_hash"]; !ok {
		t.Fatal("expected entry_hash field")
	}
	if _, ok := entry["previous_hash"]; !ok {
		t.Fatal("expected previous_hash field")
	}

	// Export as CSV
	resp3, err := http.Get(ts.URL + "/api/v1/audit/export?format=csv")
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp3.StatusCode)
	}
	if ct := resp3.Header.Get("Content-Type"); ct != "text/csv" {
		t.Fatalf("expected text/csv, got %s", ct)
	}
}

func TestIntegration_AuditVerify(t *testing.T) {
	ts := setupServer(t)

	// Create entries to build a chain
	for i := 0; i < 3; i++ {
		body := fmt.Sprintf(`{"name":"chain-test-%d","content":"test","variables":[]}`, i)
		resp, _ := http.Post(ts.URL+"/api/v1/prompts", "application/json", bytes.NewBufferString(body))
		resp.Body.Close()
	}

	// Verify chain
	resp, err := http.Get(ts.URL + "/api/v1/audit/verify")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck
	if result["valid"] != true {
		t.Fatalf("expected valid chain, got reason: %v", result["reason"])
	}
}

func TestIntegration_ProviderList(t *testing.T) {
	ts := setupServer(t)

	resp, err := http.Get(ts.URL + "/api/v1/providers")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck
	providers, ok := result["providers"].([]any)
	if !ok || len(providers) == 0 {
		t.Fatal("expected at least one registered provider")
	}
}

func TestIntegration_AgentStatusGating(t *testing.T) {
	ts := setupServer(t)

	// Create agent
	body := `{"name":"gated-agent","description":"test","steps":[]}`
	resp, err := http.Post(ts.URL+"/api/v1/agents", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var agent map[string]any
	json.NewDecoder(resp.Body).Decode(&agent) //nolint:errcheck
	id := agent["id"].(string)

	// Try to approve without review (should fail)
	updateBody := `{"status":"approved"}`
	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/agents/"+id, bytes.NewBufferString(updateBody))
	req.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp2.StatusCode)
	}

	// Create review for agent
	reviewBody := `{"resource_id":"` + id + `","resource_type":"agent","author":"reviewer"}`
	resp3, _ := http.Post(ts.URL+"/api/v1/reviews", "application/json", bytes.NewBufferString(reviewBody))
	var review map[string]any
	json.NewDecoder(resp3.Body).Decode(&review) //nolint:errcheck
	resp3.Body.Close()
	reviewID := review["id"].(string)

	// Approve review (should auto-approve agent)
	req3, _ := http.NewRequest("PUT", ts.URL+"/api/v1/reviews/"+reviewID+"/approve", nil)
	resp4, _ := http.DefaultClient.Do(req3)
	resp4.Body.Close()

	// Verify agent is now approved
	resp5, err := http.Get(ts.URL + "/api/v1/agents/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp5.Body.Close()
	var updated map[string]any
	json.NewDecoder(resp5.Body).Decode(&updated) //nolint:errcheck
	if updated["status"] != "approved" {
		t.Fatalf("expected agent status 'approved', got %v", updated["status"])
	}
}

func TestIntegration_PromptRun(t *testing.T) {
	ts := setupServer(t)

	// Register mock provider in global registry for this test
	llm.Global.Register("mock", func(cfg llm.ProviderConfig) llm.Provider {
		return llm.NewMock("test response")
	})

	// Create and approve a prompt
	body := `{"name":"run-test","content":"Hello {{name}}","variables":[{"name":"name","type":"string","required":true,"description":"Name"}]}`
	resp, err := http.Post(ts.URL+"/api/v1/prompts", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]any
	json.NewDecoder(resp.Body).Decode(&p) //nolint:errcheck
	resp.Body.Close()
	id := p["id"].(string)

	// Approve via review
	reviewBody := `{"resource_id":"` + id + `","resource_type":"prompt","author":"reviewer"}`
	resp2, _ := http.Post(ts.URL+"/api/v1/reviews", "application/json", bytes.NewBufferString(reviewBody))
	var review map[string]any
	json.NewDecoder(resp2.Body).Decode(&review) //nolint:errcheck
	resp2.Body.Close()
	reviewID := review["id"].(string)
	req2, _ := http.NewRequest("PUT", ts.URL+"/api/v1/reviews/"+reviewID+"/approve", nil)
	resp3, _ := http.DefaultClient.Do(req2)
	resp3.Body.Close()

	// Run the prompt using the mock provider
	runBody := `{"variables":{"name":"World"},"provider":"mock"}`
	resp4, err := http.Post(ts.URL+"/api/v1/prompts/"+id+"/run", "application/json", bytes.NewBufferString(runBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp4.StatusCode)
	}
	var result map[string]any
	json.NewDecoder(resp4.Body).Decode(&result) //nolint:errcheck
	if result["content"] != "test response" {
		t.Fatalf("expected 'test response', got %v", result["content"])
	}
}

func TestIntegration_PromptPreview(t *testing.T) {
	ts := setupServer(t)

	// Create a prompt
	body := `{"name":"preview-test","content":"Hello {{name}}","variables":[{"name":"name","type":"string","required":true,"description":"Name"}]}`
	resp, err := http.Post(ts.URL+"/api/v1/prompts", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]any
	json.NewDecoder(resp.Body).Decode(&p) //nolint:errcheck
	resp.Body.Close()
	id := p["id"].(string)

	// Preview the prompt
	previewBody := `{"variables":{"name":"World"}}`
	resp2, err := http.Post(ts.URL+"/api/v1/prompts/"+id+"/preview", "application/json", bytes.NewBufferString(previewBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
	var result map[string]any
	json.NewDecoder(resp2.Body).Decode(&result) //nolint:errcheck
	if result["rendered"] != "Hello World" {
		t.Fatalf("expected 'Hello World', got %v", result["rendered"])
	}
	if result["estimated_tokens"].(float64) < 1 {
		t.Fatal("expected positive token count")
	}
}

func TestIntegration_AgentImportYAML(t *testing.T) {
	ts := setupServer(t)

	yaml := `name: yaml-agent
description: imported from yaml
steps:
  - id: step1
    prompt_id: test-prompt
    depends_on: []`

	resp, err := http.Post(ts.URL+"/api/v1/agents/import-yaml", "text/yaml", bytes.NewBufferString(yaml))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var agent map[string]any
	json.NewDecoder(resp.Body).Decode(&agent) //nolint:errcheck
	if agent["name"] != "yaml-agent" {
		t.Fatalf("expected 'yaml-agent', got %v", agent["name"])
	}
}

func TestIntegration_AgentExportYAML(t *testing.T) {
	ts := setupServer(t)

	// Create agent
	body := `{"name":"export-agent","description":"test","steps":[{"id":"s1","prompt_id":"p1"}]}`
	resp, _ := http.Post(ts.URL+"/api/v1/agents", "application/json", bytes.NewBufferString(body))
	var agent map[string]any
	json.NewDecoder(resp.Body).Decode(&agent) //nolint:errcheck
	resp.Body.Close()
	id := agent["id"].(string)

	// Export as YAML
	resp2, err := http.Get(ts.URL + "/api/v1/agents/" + id + "/export?format=yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
	yamlBytes, err := io.ReadAll(resp2.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(yamlBytes) == 0 {
		t.Fatal("expected non-empty YAML response")
	}
}

func TestIntegration_CSVImport(t *testing.T) {
	ts := setupServer(t)

	// Create dataset
	body := `{"name":"csv-test","cases":[]}`
	resp, _ := http.Post(ts.URL+"/api/v1/datasets", "application/json", bytes.NewBufferString(body))
	var ds map[string]any
	json.NewDecoder(resp.Body).Decode(&ds) //nolint:errcheck
	resp.Body.Close()
	id := ds["id"].(string)

	// Import CSV - proper JSON encoding
	csvData := "input,output\nhello,world\nfoo,bar"
	importBody, _ := json.Marshal(map[string]string{"csv": csvData})
	resp2, err := http.Post(ts.URL+"/api/v1/datasets/"+id+"/import-csv", "application/json", bytes.NewBuffer(importBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
	var result map[string]any
	json.NewDecoder(resp2.Body).Decode(&result) //nolint:errcheck
	if result["cases_added"].(float64) != 2 {
		t.Fatalf("expected 2 cases added, got %v", result["cases_added"])
	}
}
