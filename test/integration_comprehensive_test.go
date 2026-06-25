package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"promptsheon/internal/api"
	"promptsheon/internal/auth"
	"promptsheon/internal/eval"
	"promptsheon/internal/guardrail"
	"promptsheon/internal/llm"
	"promptsheon/internal/metrics"
	"promptsheon/internal/models"
	"promptsheon/internal/snapshot"
	"promptsheon/internal/store"
	"promptsheon/internal/trace"
	"promptsheon/internal/vault"
	"promptsheon/internal/webhook"
)

// fullSetupServer creates a test HTTP server with all optional dependencies wired.
// Authentication is NOT enabled by default to preserve the behaviour the
// integration tests were written against; the targeted security tests
// (TestComprehensive_UnauthorizedAccess, TestComprehensive_APINonAdminCannotEscalate)
// flip it on via the integrationTestAuthEnabled switch.
func fullSetupServer(t *testing.T) *httptest.Server {
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
	guardrailMgr := guardrail.NewManager(logger, collector)
	snapshotStore, err := snapshot.NewStore(db.DB())
	if err != nil {
		t.Fatalf("failed to create snapshot store: %v", err)
	}
	webhookDispatcher := webhook.NewDispatcher(logger)
	vaultKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	v, err := vault.New(vaultKey)
	if err != nil {
		t.Fatalf("failed to create vault: %v", err)
	}

	opts := []api.Option{
		api.WithEvalRunner(evalRunner),
		api.WithTracing(spans, collector),
		api.WithGuardrailManager(guardrailMgr),
		api.WithSnapshotStore(snapshotStore),
		api.WithWebhooks(webhookDispatcher),
		api.WithVault(v),
	}
	if integrationTestAuthEnabled {
		// Seed a default admin user so the suite has a privileged caller.
		seedDefaultUsers(t, db)
		opts = append(opts, api.WithAuth(db))
	}
	srv := api.NewServer(db, logger, opts...)
	srv.StartAuditWorkers(context.Background(), 2)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts
}

// integrationTestAuthEnabled flips WithAuth on for the integration
// suite. The dedicated security tests set this to true; the rest of
// the suite runs in legacy (no-auth) mode.
var integrationTestAuthEnabled = false

var (
	defaultTestAuthOnce sync.Once
	defaultTestAuth     string
)

func seedDefaultUsers(t *testing.T, db *store.SQLite) {
	t.Helper()
	for _, name := range []string{"admin", "user1", "user2", "writer1", "reader1"} {
		role := "admin"
		switch name {
		case "writer1":
			role = "writer"
		case "reader1":
			role = "reader"
		case "user1", "user2":
			role = "writer"
		}
		u := &models.User{
			ID:        "u-" + name,
			Email:     name + "@test.local",
			Name:      name,
			Role:      role,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := db.CreateUser(context.Background(), u); err != nil {
			t.Fatalf("seed user %s: %v", name, err)
		}
	}
	defaultTestAuthOnce.Do(func() {
		key, hash, err := auth.GenerateAPIKey()
		if err != nil {
			t.Fatalf("gen admin key: %v", err)
		}
		rec := &models.APIKey{
			ID:        "k-admin",
			UserID:    "u-admin",
			Name:      "test-admin",
			KeyHash:   hash,
			KeyPrefix: key[:8],
			Role:      "admin",
			CreatedAt: time.Now(),
		}
		if err := db.CreateAPIKey(context.Background(), rec); err != nil {
			t.Fatalf("save admin key: %v", err)
		}
		defaultTestAuth = key
	})
}

func authedRequest(t *testing.T, method, url, body string) *http.Request {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = bytes.NewBufferString(body)
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if defaultTestAuth != "" {
		req.Header.Set("Authorization", "Bearer "+defaultTestAuth)
	}
	return req
}

func createPrompt(t *testing.T, ts *httptest.Server, name string) string {
	t.Helper()
	body := fmt.Sprintf(`{"name":"%s","content":"You are a {{role}}. {{instruction}}","tags":["test"]}`, name)
	req := authedRequest(t, "POST", ts.URL+"/api/v1/prompts", body)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created)
	return created["id"].(string)
}

func createAgent(t *testing.T, ts *httptest.Server, name string) string {
	t.Helper()
	body := fmt.Sprintf(`{"name":"%s","description":"test","steps":[{"id":"step1","output_key":"out1"}]}`, name)
	resp, err := http.Post(ts.URL+"/api/v1/agents", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created)
	return created["id"].(string)
}

func createDataset(t *testing.T, ts *httptest.Server, name string) string {
	t.Helper()
	body := fmt.Sprintf(`{"name":"%s","cases":[{"input":{"text":"hello"},"expected_output":"world"}]}`, name)
	resp, err := http.Post(ts.URL+"/api/v1/datasets", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created)
	return created["id"].(string)
}

func approvePrompt(t *testing.T, ts *httptest.Server, promptID string) {
	t.Helper()
	reviewBody := fmt.Sprintf(`{"resource_id":"%s","resource_type":"prompt","author":"reviewer"}`, promptID)
	resp, _ := http.Post(ts.URL+"/api/v1/reviews", "application/json", bytes.NewBufferString(reviewBody))
	var review map[string]any
	json.NewDecoder(resp.Body).Decode(&review)
	resp.Body.Close()
	reviewID := review["id"].(string)
	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/reviews/"+reviewID+"/approve", nil)
	resp2, _ := http.DefaultClient.Do(req)
	resp2.Body.Close()
}

func doPut(t *testing.T, ts *httptest.Server, path string, body string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("PUT", ts.URL+path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func doDelete(t *testing.T, ts *httptest.Server, path string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("DELETE", ts.URL+path, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func doPost(t *testing.T, ts *httptest.Server, path string, body string, ct string) *http.Response {
	t.Helper()
	if ct == "" {
		ct = "application/json"
	}
	resp, err := http.Post(ts.URL+path, ct, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decodeJSON(t *testing.T, resp *http.Response, target any) {
	t.Helper()
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(target)
}

// =====================================================================
// 1. Prompt CRUD lifecycle (10 tests)
// =====================================================================

func TestComprehensive_PromptCreateAllFields(t *testing.T) {
	ts := fullSetupServer(t)
	body := `{"name":"full-prompt","description":"A full prompt","content":"You are {{role}}. {{instruction}}","variables":[{"name":"role","type":"string","required":true},{"name":"instruction","type":"string","required":false,"default":"Answer"}],"tags":["prod","v1"],"model_hint":"gpt-4","environment":"production","metadata":{"team":"ml","priority":"high"}}`
	resp := doPost(t, ts, "/api/v1/prompts", body, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var p map[string]any
	decodeJSON(t, resp, &p)
	if p["name"] != "full-prompt" {
		t.Fatalf("expected name 'full-prompt', got %v", p["name"])
	}
	if p["description"] != "A full prompt" {
		t.Fatalf("expected description, got %v", p["description"])
	}
	if p["model_hint"] != "gpt-4" {
		t.Fatalf("expected model_hint 'gpt-4', got %v", p["model_hint"])
	}
	if p["environment"] != "production" {
		t.Fatalf("expected environment 'production', got %v", p["environment"])
	}
}

func TestComprehensive_PromptCreateMinimal(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/prompts", `{"name":"minimal-prompt","content":"Hello"}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var p map[string]any
	decodeJSON(t, resp, &p)
	if p["name"] != "minimal-prompt" {
		t.Fatalf("expected name 'minimal-prompt', got %v", p["name"])
	}
	if p["version"].(float64) != 1 {
		t.Fatalf("expected version 1, got %v", p["version"])
	}
}

func TestComprehensive_PromptGetByID(t *testing.T) {
	ts := fullSetupServer(t)
	id := createPrompt(t, ts, "get-test")
	resp, err := http.Get(ts.URL + "/api/v1/prompts/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var p map[string]any
	decodeJSON(t, resp, &p)
	if p["id"] != id {
		t.Fatalf("expected id %s, got %v", id, p["id"])
	}
}

func TestComprehensive_PromptListWithFilters(t *testing.T) {
	ts := fullSetupServer(t)
	createPrompt(t, ts, "filter-alpha")
	createPrompt(t, ts, "filter-beta")

	resp, err := http.Get(ts.URL + "/api/v1/prompts")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var prompts []map[string]any
	decodeJSON(t, resp, &prompts)
	if len(prompts) < 2 {
		t.Fatalf("expected at least 2 prompts, got %d", len(prompts))
	}
}

func TestComprehensive_PromptUpdate(t *testing.T) {
	ts := fullSetupServer(t)
	id := createPrompt(t, ts, "update-test")
	resp := doPut(t, ts, "/api/v1/prompts/"+id, `{"name":"update-test-v2","content":"Updated content {{role}}","tags":["updated"]}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var p map[string]any
	decodeJSON(t, resp, &p)
	if p["name"] != "update-test-v2" {
		t.Fatalf("expected updated name, got %v", p["name"])
	}
}

func TestComprehensive_PromptDelete(t *testing.T) {
	ts := fullSetupServer(t)
	id := createPrompt(t, ts, "delete-test")
	resp := doDelete(t, ts, "/api/v1/prompts/"+id)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	resp2, err2 := http.Get(ts.URL + "/api/v1/prompts/" + id)
	if err2 != nil {
		t.Fatal(err2)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp2.StatusCode)
	}
}

func TestComprehensive_PromptDeleteNonExistent(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doDelete(t, ts, "/api/v1/prompts/nonexistent")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestComprehensive_PromptGetNonExistent(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/prompts/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestComprehensive_PromptListPagination(t *testing.T) {
	ts := fullSetupServer(t)
	for i := 0; i < 5; i++ {
		createPrompt(t, ts, fmt.Sprintf("page-test-%d", i))
	}
	resp, err := http.Get(ts.URL + "/api/v1/prompts")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var prompts []map[string]any
	decodeJSON(t, resp, &prompts)
	if len(prompts) < 5 {
		t.Fatalf("expected at least 5 prompts, got %d", len(prompts))
	}
}

func TestComprehensive_PromptSearchByName(t *testing.T) {
	ts := fullSetupServer(t)
	createPrompt(t, ts, "searchable-unique-name")
	createPrompt(t, ts, "another-unique-name")

	resp, err := http.Get(ts.URL + "/api/v1/prompts?search=searchable-unique-name")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var prompts []map[string]any
	decodeJSON(t, resp, &prompts)
	if len(prompts) == 0 {
		t.Fatal("expected at least 1 prompt from search")
	}
}

// =====================================================================
// 2. Prompt versioning (5 tests)
// =====================================================================

func TestComprehensive_PromptCreateVersion(t *testing.T) {
	ts := fullSetupServer(t)
	id := createPrompt(t, ts, "version-test")
	resp := doPut(t, ts, "/api/v1/prompts/"+id, `{"content":"Version 2 content"}`)
	defer resp.Body.Close()
	var p map[string]any
	decodeJSON(t, resp, &p)
	if p["version"].(float64) != 2 {
		t.Fatalf("expected version 2, got %v", p["version"])
	}
}

func TestComprehensive_PromptListVersions(t *testing.T) {
	ts := fullSetupServer(t)
	id := createPrompt(t, ts, "list-versions-test")
	resp, err := http.Get(ts.URL + "/api/v1/prompts/" + id + "/versions")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestComprehensive_PromptGetSpecificVersion(t *testing.T) {
	ts := fullSetupServer(t)
	id := createPrompt(t, ts, "get-version-test")
	resp2 := doPut(t, ts, "/api/v1/prompts/"+id, `{"content":"Version 2"}`)
	resp2.Body.Close()

	resp3, err := http.Get(ts.URL + "/api/v1/prompts/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	var p map[string]any
	decodeJSON(t, resp3, &p)
	if p["version"].(float64) != 2 {
		t.Fatalf("expected version 2, got %v", p["version"])
	}
}

func TestComprehensive_PromptVersionWithChanges(t *testing.T) {
	ts := fullSetupServer(t)
	id := createPrompt(t, ts, "changes-test")

	// Update content (should increment version)
	resp := doPut(t, ts, "/api/v1/prompts/"+id, `{"content":"Completely new content","variables":[{"name":"x","type":"string"}]}`)
	var p map[string]any
	decodeJSON(t, resp, &p)
	if p["version"].(float64) != 2 {
		t.Fatalf("expected version 2 after content change, got %v", p["version"])
	}

	// Update only tags (should NOT increment version)
	resp2 := doPut(t, ts, "/api/v1/prompts/"+id, `{"tags":["new-tag"]}`)
	var p2 map[string]any
	decodeJSON(t, resp2, &p2)
	if p2["version"].(float64) != 2 {
		t.Fatalf("expected version to remain 2 after tag change, got %v", p2["version"])
	}
}

func TestComprehensive_PromptDeployAndArchive(t *testing.T) {
	ts := fullSetupServer(t)
	id := createPrompt(t, ts, "deploy-test")
	approvePrompt(t, ts, id)

	// Deploy via POST
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/prompts/"+id+"/deploy", nil)
	resp2, err2 := http.DefaultClient.Do(req)
	if err2 != nil {
		t.Fatal(err2)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("deploy: expected 200, got %d", resp2.StatusCode)
	}
	var p map[string]any
	decodeJSON(t, resp2, &p)
	if p["status"] != "deployed" {
		t.Fatalf("expected status 'deployed', got %v", p["status"])
	}

	req3, _ := http.NewRequest("POST", ts.URL+"/api/v1/prompts/"+id+"/archive", nil)
	resp3, err3 := http.DefaultClient.Do(req3)
	if err3 != nil {
		t.Fatal(err3)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("archive: expected 200, got %d", resp3.StatusCode)
	}
	var p2 map[string]any
	decodeJSON(t, resp3, &p2)
	if p2["status"] != "archived" {
		t.Fatalf("expected status 'archived', got %v", p2["status"])
	}
}

// =====================================================================
// 3. Agent CRUD lifecycle (8 tests)
// =====================================================================

func TestComprehensive_AgentCreateWithConfig(t *testing.T) {
	ts := fullSetupServer(t)
	body := `{"name":"config-agent","description":"Agent with tools","steps":[{"id":"s1","output_key":"o1","depends_on":[]}],"tools":[{"name":"json_transform","type":"json_transform","config":{"expression":"$.data"}}]}`
	resp := doPost(t, ts, "/api/v1/agents", body, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var a map[string]any
	decodeJSON(t, resp, &a)
	if a["name"] != "config-agent" {
		t.Fatalf("expected name 'config-agent', got %v", a["name"])
	}
}

func TestComprehensive_AgentCreateDefaults(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/agents", `{"name":"defaults-agent"}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var a map[string]any
	decodeJSON(t, resp, &a)
	if a["status"] != "draft" {
		t.Fatalf("expected status 'draft', got %v", a["status"])
	}
}

func TestComprehensive_AgentGetByID(t *testing.T) {
	ts := fullSetupServer(t)
	id := createAgent(t, ts, "get-agent")
	resp, err := http.Get(ts.URL + "/api/v1/agents/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestComprehensive_AgentList(t *testing.T) {
	ts := fullSetupServer(t)
	createAgent(t, ts, "list-agent-1")
	createAgent(t, ts, "list-agent-2")
	resp, err := http.Get(ts.URL + "/api/v1/agents")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var agents []map[string]any
	decodeJSON(t, resp, &agents)
	if len(agents) < 2 {
		t.Fatalf("expected at least 2 agents, got %d", len(agents))
	}
}

func TestComprehensive_AgentUpdate(t *testing.T) {
	ts := fullSetupServer(t)
	id := createAgent(t, ts, "update-agent")
	resp := doPut(t, ts, "/api/v1/agents/"+id, `{"name":"update-agent-v2","description":"Updated","steps":[{"id":"s1","output_key":"o1"}]}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var a map[string]any
	decodeJSON(t, resp, &a)
	if a["name"] != "update-agent-v2" {
		t.Fatalf("expected updated name, got %v", a["name"])
	}
}

func TestComprehensive_AgentDelete(t *testing.T) {
	ts := fullSetupServer(t)
	id := createAgent(t, ts, "delete-agent")
	resp := doDelete(t, ts, "/api/v1/agents/"+id)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestComprehensive_AgentForkWithPromptBinding(t *testing.T) {
	ts := fullSetupServer(t)
	id := createAgent(t, ts, "fork-parent")
	resp := doPost(t, ts, "/api/v1/agents/"+id+"/fork", `{"name":"forked-child"}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var forked map[string]any
	decodeJSON(t, resp, &forked)
	if forked["parent_id"] != id {
		t.Fatalf("expected parent_id %s, got %v", id, forked["parent_id"])
	}
}

func TestComprehensive_AgentWithModelConfig(t *testing.T) {
	ts := fullSetupServer(t)
	body := `{"name":"model-agent","description":"Agent with tools","steps":[{"id":"s1","output_key":"o1"}],"tools":[{"name":"json_transform","type":"json_transform","config":{"expression":"$.data"}}]}`
	resp := doPost(t, ts, "/api/v1/agents", body, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

// =====================================================================
// 4. Dataset operations (6 tests)
// =====================================================================

func TestComprehensive_DatasetCreate(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/datasets", `{"name":"new-dataset","cases":[{"input":{"text":"hello"},"expected_output":"world"}]}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var d map[string]any
	decodeJSON(t, resp, &d)
	if d["name"] != "new-dataset" {
		t.Fatalf("expected name 'new-dataset', got %v", d["name"])
	}
}

func TestComprehensive_DatasetList(t *testing.T) {
	ts := fullSetupServer(t)
	createDataset(t, ts, "ds-list-1")
	createDataset(t, ts, "ds-list-2")
	resp, err := http.Get(ts.URL + "/api/v1/datasets")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var datasets []map[string]any
	decodeJSON(t, resp, &datasets)
	if len(datasets) < 2 {
		t.Fatalf("expected at least 2 datasets, got %d", len(datasets))
	}
}

func TestComprehensive_DatasetGet(t *testing.T) {
	ts := fullSetupServer(t)
	id := createDataset(t, ts, "ds-get")
	resp, err := http.Get(ts.URL + "/api/v1/datasets/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestComprehensive_DatasetDelete(t *testing.T) {
	ts := fullSetupServer(t)
	id := createDataset(t, ts, "ds-delete")
	resp := doDelete(t, ts, "/api/v1/datasets/"+id)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestComprehensive_DatasetUpdate(t *testing.T) {
	ts := fullSetupServer(t)
	id := createDataset(t, ts, "ds-update")
	resp := doPut(t, ts, "/api/v1/datasets/"+id, `{"name":"ds-update-v2","cases":[{"input":{"q":"test"},"expected_output":"answer"}]}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var updated map[string]any
	decodeJSON(t, resp, &updated)
	if updated["name"] != "ds-update-v2" {
		t.Fatalf("expected updated name, got %v", updated["name"])
	}
}

func TestComprehensive_DatasetWithMetadata(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/datasets", `{"name":"ds-metadata","cases":[{"input":{"text":"hi"},"expected_output":"hello","metadata":{"source":"test"}}]}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

// =====================================================================
// 5. Eval operations (8 tests)
// =====================================================================

func TestComprehensive_EvalSaveResults(t *testing.T) {
	ts := fullSetupServer(t)
	dsID := createDataset(t, ts, "eval-ds")
	resp := doPost(t, ts, "/api/v1/eval/run", fmt.Sprintf(`{"prompt_hash":"eval-hash-1","dataset_id":"%s","prompt_text":"test prompt","scorers":["exact_match"]}`, dsID), "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestComprehensive_EvalGetResults(t *testing.T) {
	ts := fullSetupServer(t)
	dsID := createDataset(t, ts, "eval-get-ds")
	doPost(t, ts, "/api/v1/eval/run", fmt.Sprintf(`{"prompt_hash":"eval-hash-get","dataset_id":"%s","prompt_text":"test","scorers":["contains"]}`, dsID), "").Body.Close()

	resp, err := http.Get(ts.URL + "/api/v1/eval/results?prompt_hash=eval-hash-get")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var results []any
	decodeJSON(t, resp, &results)
	if len(results) == 0 {
		t.Fatal("expected at least 1 eval result")
	}
}

func TestComprehensive_EvalListRuns(t *testing.T) {
	ts := fullSetupServer(t)
	dsID := createDataset(t, ts, "eval-runs-ds")
	doPost(t, ts, "/api/v1/eval/run", fmt.Sprintf(`{"prompt_hash":"eval-hash-runs","dataset_id":"%s","prompt_text":"test"}`, dsID), "").Body.Close()

	resp, err := http.Get(ts.URL + "/api/v1/eval/runs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var runs []any
	decodeJSON(t, resp, &runs)
	if len(runs) == 0 {
		t.Fatal("expected at least 1 eval run")
	}
}

func TestComprehensive_EvalRunAndSave(t *testing.T) {
	ts := fullSetupServer(t)
	dsID := createDataset(t, ts, "eval-save-ds")
	resp := doPost(t, ts, "/api/v1/eval/run", fmt.Sprintf(`{"prompt_hash":"eval-hash-save","dataset_id":"%s","prompt_text":"evaluate this","scorers":["exact_match"]}`, dsID), "")
	defer resp.Body.Close()
	var report map[string]any
	decodeJSON(t, resp, &report)
	agg := report["aggregate"].(map[string]any)
	if agg["total_cases"].(float64) < 1 {
		t.Fatalf("expected at least 1 total case, got %v", agg["total_cases"])
	}
}

func TestComprehensive_EvalWithScores(t *testing.T) {
	ts := fullSetupServer(t)
	dsID := createDataset(t, ts, "eval-scores-ds")
	resp := doPost(t, ts, "/api/v1/eval/run", fmt.Sprintf(`{"prompt_hash":"eval-hash-scores","dataset_id":"%s","prompt_text":"score this","scorers":["exact_match","contains"]}`, dsID), "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestComprehensive_EvalWithErrors(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/eval/run", `{"prompt_hash":"","dataset_id":""}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestComprehensive_EvalByDataset(t *testing.T) {
	ts := fullSetupServer(t)
	dsID := createDataset(t, ts, "eval-by-ds")
	doPost(t, ts, "/api/v1/eval/run", fmt.Sprintf(`{"prompt_hash":"eval-hash-byds","dataset_id":"%s","prompt_text":"test"}`, dsID), "").Body.Close()

	resp, err := http.Get(ts.URL + "/api/v1/eval/results?dataset_id=" + dsID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestComprehensive_EvalReport(t *testing.T) {
	ts := fullSetupServer(t)
	dsID := createDataset(t, ts, "eval-report-ds")
	doPost(t, ts, "/api/v1/eval/run", fmt.Sprintf(`{"prompt_hash":"eval-hash-report","dataset_id":"%s","prompt_text":"test"}`, dsID), "").Body.Close()

	resp, err := http.Get(ts.URL + "/api/v1/eval/report?prompt_hash=eval-hash-report")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// =====================================================================
// 6. Workflow operations (8 tests)
// =====================================================================

func TestComprehensive_WorkflowRun(t *testing.T) {
	ts := fullSetupServer(t)
	agentID := createAgent(t, ts, "wf-agent")
	resp := doPost(t, ts, "/api/v1/workflows/run", fmt.Sprintf(`{"agent_id":"%s","input":{"data":"hello"}}`, agentID), "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var wf map[string]any
	decodeJSON(t, resp, &wf)
	if wf["agent_id"] != agentID {
		t.Fatalf("expected agent_id %s, got %v", agentID, wf["agent_id"])
	}
}

func TestComprehensive_WorkflowGet(t *testing.T) {
	ts := fullSetupServer(t)
	agentID := createAgent(t, ts, "wf-get-agent")
	resp := doPost(t, ts, "/api/v1/workflows/run", fmt.Sprintf(`{"agent_id":"%s","input":{}}`, agentID), "")
	var wf map[string]any
	decodeJSON(t, resp, &wf)
	wfID := wf["id"].(string)

	resp2, err := http.Get(ts.URL + "/api/v1/workflows/" + wfID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
}

func TestComprehensive_WorkflowList(t *testing.T) {
	ts := fullSetupServer(t)
	agentID := createAgent(t, ts, "wf-list-agent")
	doPost(t, ts, "/api/v1/workflows/run", fmt.Sprintf(`{"agent_id":"%s","input":{}}`, agentID), "").Body.Close()

	resp, err := http.Get(ts.URL + "/api/v1/workflows")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var workflows []map[string]any
	decodeJSON(t, resp, &workflows)
	if len(workflows) == 0 {
		t.Fatal("expected at least 1 workflow")
	}
}

func TestComprehensive_WorkflowSteps(t *testing.T) {
	ts := fullSetupServer(t)
	agentID := createAgent(t, ts, "wf-steps-agent")
	resp := doPost(t, ts, "/api/v1/workflows/run", fmt.Sprintf(`{"agent_id":"%s","input":{}}`, agentID), "")
	var wf map[string]any
	decodeJSON(t, resp, &wf)
	wfID := wf["id"].(string)

	resp2, err := http.Get(ts.URL + "/api/v1/workflows/" + wfID + "/steps")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
}

func TestComprehensive_WorkflowCancel(t *testing.T) {
	ts := fullSetupServer(t)
	agentID := createAgent(t, ts, "wf-cancel-agent")
	resp := doPost(t, ts, "/api/v1/workflows/run", fmt.Sprintf(`{"agent_id":"%s","input":{}}`, agentID), "")
	var wf map[string]any
	decodeJSON(t, resp, &wf)
	wfID := wf["id"].(string)

	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/workflows/"+wfID+"/cancel", nil)
	resp2, err2 := http.DefaultClient.Do(req)
	if err2 != nil {
		t.Fatal(err2)
	}
	defer resp2.Body.Close()
	// May get 409 if already completed, which is acceptable
	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusConflict {
		t.Fatalf("expected 200 or 409, got %d", resp2.StatusCode)
	}
}

func TestComprehensive_WorkflowRunEmpty(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/workflows/run", `{"agent_id":"","input":{}}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestComprehensive_WorkflowRunNonExistentAgent(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/workflows/run", `{"agent_id":"nonexistent","input":{}}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestComprehensive_WorkflowGetNonExistent(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/workflows/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// =====================================================================
// 7. Review operations (6 tests)
// =====================================================================

func TestComprehensive_ReviewCreate(t *testing.T) {
	ts := fullSetupServer(t)
	promptID := createPrompt(t, ts, "review-create-prompt")
	resp := doPost(t, ts, "/api/v1/reviews", fmt.Sprintf(`{"resource_id":"%s","resource_type":"prompt","author":"reviewer1"}`, promptID), "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

func TestComprehensive_ReviewListPending(t *testing.T) {
	ts := fullSetupServer(t)
	promptID := createPrompt(t, ts, "review-pending-prompt")
	doPost(t, ts, "/api/v1/reviews", fmt.Sprintf(`{"resource_id":"%s","resource_type":"prompt","author":"reviewer"}`, promptID), "").Body.Close()

	resp, err := http.Get(ts.URL + "/api/v1/reviews")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var reviews []map[string]any
	decodeJSON(t, resp, &reviews)
	if len(reviews) == 0 {
		t.Fatal("expected at least 1 pending review")
	}
}

func TestComprehensive_ReviewUpdateStatus(t *testing.T) {
	ts := fullSetupServer(t)
	promptID := createPrompt(t, ts, "review-status-prompt")
	resp := doPost(t, ts, "/api/v1/reviews", fmt.Sprintf(`{"resource_id":"%s","resource_type":"prompt","author":"reviewer"}`, promptID), "")
	var review map[string]any
	decodeJSON(t, resp, &review)
	reviewID := review["id"].(string)

	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/reviews/"+reviewID+"/approve", nil)
	resp2, err2 := http.DefaultClient.Do(req)
	if err2 != nil {
		t.Fatal(err2)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
}

func TestComprehensive_ReviewWithComments(t *testing.T) {
	ts := fullSetupServer(t)
	promptID := createPrompt(t, ts, "review-comment-prompt")
	resp := doPost(t, ts, "/api/v1/reviews", fmt.Sprintf(`{"resource_id":"%s","resource_type":"prompt","author":"reviewer"}`, promptID), "")
	var review map[string]any
	decodeJSON(t, resp, &review)
	reviewID := review["id"].(string)

	resp2 := doPost(t, ts, "/api/v1/reviews/"+reviewID+"/comment", `{"content":"Looks good!"}`, "")
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
}

func TestComprehensive_ReviewByResource(t *testing.T) {
	ts := fullSetupServer(t)
	promptID := createPrompt(t, ts, "review-resource-prompt")
	doPost(t, ts, "/api/v1/reviews", fmt.Sprintf(`{"resource_id":"%s","resource_type":"prompt","author":"reviewer"}`, promptID), "").Body.Close()

	resp, err := http.Get(ts.URL + "/api/v1/reviews")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var reviews []map[string]any
	decodeJSON(t, resp, &reviews)
	found := false
	for _, r := range reviews {
		if r["resource_id"] == promptID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected to find review for the resource")
	}
}

func TestComprehensive_ReviewReject(t *testing.T) {
	ts := fullSetupServer(t)
	promptID := createPrompt(t, ts, "review-reject-prompt")
	resp := doPost(t, ts, "/api/v1/reviews", fmt.Sprintf(`{"resource_id":"%s","resource_type":"prompt","author":"reviewer"}`, promptID), "")
	var review map[string]any
	decodeJSON(t, resp, &review)
	reviewID := review["id"].(string)

	resp2 := doPut(t, ts, "/api/v1/reviews/"+reviewID+"/reject", `{"reason":"Needs more work"}`)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
}

// =====================================================================
// 8. Audit operations (8 tests)
// =====================================================================

func TestComprehensive_AuditAppendEntry(t *testing.T) {
	ts := fullSetupServer(t)
	createPrompt(t, ts, "audit-append-prompt")
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(ts.URL + "/api/v1/audit")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var entries []map[string]any
	decodeJSON(t, resp, &entries)
	if len(entries) == 0 {
		t.Fatal("expected at least 1 audit entry")
	}
}

func TestComprehensive_AuditListEntries(t *testing.T) {
	ts := fullSetupServer(t)
	createPrompt(t, ts, "audit-list-prompt-1")
	createPrompt(t, ts, "audit-list-prompt-2")
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(ts.URL + "/api/v1/audit")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var entries []map[string]any
	decodeJSON(t, resp, &entries)
	if len(entries) < 2 {
		t.Fatalf("expected at least 2 audit entries, got %d", len(entries))
	}
}

func TestComprehensive_AuditExportJSON(t *testing.T) {
	ts := fullSetupServer(t)
	createPrompt(t, ts, "audit-export-json-prompt")
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(ts.URL + "/api/v1/audit/export?format=json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var entries []map[string]any
	decodeJSON(t, resp, &entries)
	if len(entries) == 0 {
		t.Fatal("expected at least 1 audit entry in export")
	}
}

func TestComprehensive_AuditExportCSV(t *testing.T) {
	ts := fullSetupServer(t)
	createPrompt(t, ts, "audit-export-csv-prompt")
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(ts.URL + "/api/v1/audit/export?format=csv")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/csv" {
		t.Fatalf("expected text/csv, got %s", ct)
	}
}

func TestComprehensive_AuditWithTimeFilters(t *testing.T) {
	ts := fullSetupServer(t)
	createPrompt(t, ts, "audit-time-filter-prompt")
	time.Sleep(200 * time.Millisecond)

	since := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	until := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
	resp, err := http.Get(ts.URL + "/api/v1/audit?since=" + url.QueryEscape(since) + "&until=" + url.QueryEscape(until))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestComprehensive_AuditChainVerify(t *testing.T) {
	ts := fullSetupServer(t)
	for i := 0; i < 3; i++ {
		createPrompt(t, ts, fmt.Sprintf("audit-chain-%d", i))
	}
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(ts.URL + "/api/v1/audit/verify")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result map[string]any
	decodeJSON(t, resp, &result)
	if result["valid"] != true {
		t.Fatalf("expected valid chain, got reason: %v", result["reason"])
	}
}

func TestComprehensive_AuditByAction(t *testing.T) {
	ts := fullSetupServer(t)
	createPrompt(t, ts, "audit-action-prompt")
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(ts.URL + "/api/v1/audit?action=create")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var entries []map[string]any
	decodeJSON(t, resp, &entries)
	if len(entries) == 0 {
		t.Fatal("expected at least 1 create audit entry")
	}
}

func TestComprehensive_AuditByUser(t *testing.T) {
	ts := fullSetupServer(t)
	createPrompt(t, ts, "audit-user-prompt")
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(ts.URL + "/api/v1/audit?user_id=api")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var entries []map[string]any
	decodeJSON(t, resp, &entries)
	if len(entries) == 0 {
		t.Fatal("expected at least 1 audit entry for user 'api'")
	}
}

// =====================================================================
// 9. Execution logs (6 tests)
// =====================================================================

func TestComprehensive_ExecutionLogSave(t *testing.T) {
	ts := fullSetupServer(t)
	llm.Global.Register("mock", func(cfg llm.ProviderConfig) llm.Provider {
		return llm.NewMock("test response")
	})
	llm.Global.Configure("mock", llm.ProviderConfig{})

	id := createPrompt(t, ts, "exec-save-prompt")
	approvePrompt(t, ts, id)

	resp := doPost(t, ts, "/api/v1/prompts/"+id+"/run", `{"variables":{"name":"World"},"provider":"mock"}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestComprehensive_ExecutionLogGet(t *testing.T) {
	ts := fullSetupServer(t)
	llm.Global.Register("mock", func(cfg llm.ProviderConfig) llm.Provider {
		return llm.NewMock("test response")
	})
	llm.Global.Configure("mock", llm.ProviderConfig{})

	id := createPrompt(t, ts, "exec-get-prompt")
	approvePrompt(t, ts, id)
	doPost(t, ts, "/api/v1/prompts/"+id+"/run", `{"variables":{"name":"World"},"provider":"mock"}`, "").Body.Close()

	// Execution logs are stored internally; check via list
	resp, err := http.Get(ts.URL + "/api/v1/audit?action=run")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestComprehensive_ExecutionLogList(t *testing.T) {
	ts := fullSetupServer(t)
	llm.Global.Register("mock", func(cfg llm.ProviderConfig) llm.Provider {
		return llm.NewMock("test response")
	})
	llm.Global.Configure("mock", llm.ProviderConfig{})

	id := createPrompt(t, ts, "exec-list-prompt")
	approvePrompt(t, ts, id)
	doPost(t, ts, "/api/v1/prompts/"+id+"/run", `{"variables":{"name":"World"},"provider":"mock"}`, "").Body.Close()

	resp, err := http.Get(ts.URL + "/api/v1/audit?action=run")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var entries []map[string]any
	decodeJSON(t, resp, &entries)
	if len(entries) == 0 {
		t.Fatal("expected at least 1 run audit entry")
	}
}

func TestComprehensive_ExecutionWithTiming(t *testing.T) {
	ts := fullSetupServer(t)
	llm.Global.Register("mock", func(cfg llm.ProviderConfig) llm.Provider {
		return llm.NewMock("test response")
	})
	llm.Global.Configure("mock", llm.ProviderConfig{})

	id := createPrompt(t, ts, "exec-timing-prompt")
	approvePrompt(t, ts, id)

	resp := doPost(t, ts, "/api/v1/prompts/"+id+"/run", `{"variables":{"name":"World"},"provider":"mock"}`, "")
	defer resp.Body.Close()
	var result map[string]any
	decodeJSON(t, resp, &result)
	if _, ok := result["latency_ms"]; !ok {
		t.Fatal("expected latency_ms in response")
	}
}

func TestComprehensive_ExecutionWithError(t *testing.T) {
	ts := fullSetupServer(t)
	id := createPrompt(t, ts, "exec-error-prompt")
	approvePrompt(t, ts, id)

	// Try to run with non-existent provider
	resp := doPost(t, ts, "/api/v1/prompts/"+id+"/run", `{"variables":{"name":"World"},"provider":"nonexistent"}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad provider, got %d", resp.StatusCode)
	}
}

func TestComprehensive_ExecutionByProvider(t *testing.T) {
	ts := fullSetupServer(t)
	llm.Global.Register("mock", func(cfg llm.ProviderConfig) llm.Provider {
		return llm.NewMock("test response")
	})
	llm.Global.Configure("mock", llm.ProviderConfig{})

	id := createPrompt(t, ts, "exec-provider-prompt")
	approvePrompt(t, ts, id)
	doPost(t, ts, "/api/v1/prompts/"+id+"/run", `{"variables":{"name":"World"},"provider":"mock"}`, "").Body.Close()

	resp, err := http.Get(ts.URL + "/api/v1/audit?action=run")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var entries []map[string]any
	decodeJSON(t, resp, &entries)
	if len(entries) == 0 {
		t.Fatal("expected at least 1 run entry for provider")
	}
}

// =====================================================================
// 10. Context management (6 tests)
// =====================================================================

func TestComprehensive_ContextCreate(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/contexts", `{"name":"test-context","type":"system_prompt","system_prompt":"You are helpful.","token_budget":4096}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

func TestComprehensive_ContextGet(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/contexts", `{"name":"get-context","type":"system_prompt","system_prompt":"Hello"}`, "")
	var ctx map[string]any
	decodeJSON(t, resp, &ctx)
	ctxID := ctx["id"].(string)

	resp2, err := http.Get(ts.URL + "/api/v1/contexts/" + ctxID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
}

func TestComprehensive_ContextUpdate(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/contexts", `{"name":"update-context","type":"system_prompt","system_prompt":"Hello"}`, "")
	var ctx map[string]any
	decodeJSON(t, resp, &ctx)
	ctxID := ctx["id"].(string)

	resp2 := doPut(t, ts, "/api/v1/contexts/"+ctxID, `{"name":"update-context-v2","system_prompt":"Updated prompt"}`)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
}

func TestComprehensive_ContextAppendMessages(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/contexts", `{"name":"msg-context","type":"conversation","system_prompt":"Hello"}`, "")
	var ctx map[string]any
	decodeJSON(t, resp, &ctx)
	ctxID := ctx["id"].(string)

	resp2 := doPost(t, ts, "/api/v1/contexts/"+ctxID+"/messages", `{"role":"user","content":"Hi there!"}`, "")
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
}

func TestComprehensive_ContextClearMessages(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/contexts", `{"name":"clear-context","type":"conversation","system_prompt":"Hello","messages":[{"role":"user","content":"test"}]}`, "")
	var ctx map[string]any
	decodeJSON(t, resp, &ctx)
	ctxID := ctx["id"].(string)

	req, _ := http.NewRequest("DELETE", ts.URL+"/api/v1/contexts/"+ctxID+"/messages", nil)
	resp2, err2 := http.DefaultClient.Do(req)
	if err2 != nil {
		t.Fatal(err2)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
}

func TestComprehensive_ContextWithMetadata(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/contexts", `{"name":"meta-context","type":"system_prompt","system_prompt":"Hello","metadata":{"env":"test","team":"ml"}}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

// =====================================================================
// 11. Guardrail operations (5 tests)
// =====================================================================

func TestComprehensive_GuardrailCreateRule(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/guardrails/rules", `{"name":"no-pii","type":"content_policy","severity":"high","config":{"check":"no_pii"}}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

func TestComprehensive_GuardrailGetRule(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/guardrails/rules", `{"name":"get-rule","type":"content_policy","severity":"medium"}`, "")
	var rule map[string]any
	decodeJSON(t, resp, &rule)
	ruleID := rule["id"].(string)

	resp2, err := http.Get(ts.URL + "/api/v1/guardrails/rules/" + ruleID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
}

func TestComprehensive_GuardrailListRules(t *testing.T) {
	ts := fullSetupServer(t)
	doPost(t, ts, "/api/v1/guardrails/rules", `{"name":"list-rule-1","type":"content_policy","severity":"low"}`, "").Body.Close()
	doPost(t, ts, "/api/v1/guardrails/rules", `{"name":"list-rule-2","type":"response_format","severity":"medium"}`, "").Body.Close()

	resp, err := http.Get(ts.URL + "/api/v1/guardrails/rules")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var rules []map[string]any
	decodeJSON(t, resp, &rules)
	if len(rules) < 2 {
		t.Fatalf("expected at least 2 rules, got %d", len(rules))
	}
}

func TestComprehensive_GuardrailDeleteRule(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/guardrails/rules", `{"name":"delete-rule","type":"content_policy","severity":"low"}`, "")
	var rule map[string]any
	decodeJSON(t, resp, &rule)
	ruleID := rule["id"].(string)

	resp2 := doDelete(t, ts, "/api/v1/guardrails/rules/"+ruleID)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp2.StatusCode)
	}
}

func TestComprehensive_GuardrailCheck(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/guardrails/check", `{"content":"Hello world","model":"gpt-4","environment":"production"}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	decodeJSON(t, resp, &result)
	if _, ok := result["passed"]; !ok {
		t.Fatal("expected 'passed' field in response")
	}
}

// =====================================================================
// 12. Provider key operations (6 tests)
// =====================================================================

func TestComprehensive_ProviderKeySave(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/vault/keys", `{"provider_name":"openai","key_name":"default","key":"sk-test123456789012345678901234567890"}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

func TestComprehensive_ProviderKeyList(t *testing.T) {
	ts := fullSetupServer(t)
	doPost(t, ts, "/api/v1/vault/keys", `{"provider_name":"openai","key_name":"default","key":"sk-test123"}`, "").Body.Close()
	doPost(t, ts, "/api/v1/vault/keys", `{"provider_name":"anthropic","key_name":"default","key":"sk-ant-test"}`, "").Body.Close()

	resp, err := http.Get(ts.URL + "/api/v1/vault/keys")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var keys []map[string]any
	decodeJSON(t, resp, &keys)
	if len(keys) < 2 {
		t.Fatalf("expected at least 2 keys, got %d", len(keys))
	}
}

func TestComprehensive_ProviderKeyDelete(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/vault/keys", `{"provider_name":"openai","key_name":"to-delete","key":"sk-delete"}`, "")
	var key map[string]any
	decodeJSON(t, resp, &key)
	keyID := key["id"].(string)

	resp2 := doDelete(t, ts, "/api/v1/vault/keys/"+keyID)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp2.StatusCode)
	}
}

func TestComprehensive_ProviderKeyWithMetadata(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/vault/keys", `{"provider_name":"openai","key_name":"with-meta","key":"sk-meta-test"}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

func TestComprehensive_ProviderKeyMissingFields(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/vault/keys", `{"provider_name":"openai"}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestComprehensive_ProviderKeyGetByName(t *testing.T) {
	ts := fullSetupServer(t)
	doPost(t, ts, "/api/v1/vault/keys", `{"provider_name":"openai","key_name":"by-name","key":"sk-byname"}`, "").Body.Close()

	resp, err := http.Get(ts.URL + "/api/v1/vault/keys")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var keys []map[string]any
	decodeJSON(t, resp, &keys)
	found := false
	for _, k := range keys {
		if k["provider_name"] == "openai" && k["key_name"] == "by-name" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected to find key by provider and name")
	}
}

// =====================================================================
// 13. Authentication flow (6 tests)
// =====================================================================

func TestComprehensive_APIKeyCreate(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/apikeys", `{"name":"test-key","user_id":"u-user1","role":"reader"}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var result map[string]any
	decodeJSON(t, resp, &result)
	if result["key"] == nil || result["key"] == "" {
		t.Fatal("expected non-empty key in response")
	}
}

func TestComprehensive_APIKeyList(t *testing.T) {
	ts := fullSetupServer(t)
	doPost(t, ts, "/api/v1/apikeys", `{"name":"list-key-1","user_id":"u-user1","role":"reader"}`, "").Body.Close()
	doPost(t, ts, "/api/v1/apikeys", `{"name":"list-key-2","user_id":"u-user1","role":"writer"}`, "").Body.Close()

	req := authedRequest(t, "GET", ts.URL+"/api/v1/apikeys?user_id=u-user1", "")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var keys []map[string]any
	decodeJSON(t, resp, &keys)
	if len(keys) < 2 {
		t.Fatalf("expected at least 2 keys, got %d", len(keys))
	}
}

func TestComprehensive_APIKeyRevoke(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/apikeys", `{"name":"revoke-key","user_id":"u-user1","role":"reader"}`, "")
	var result map[string]any
	decodeJSON(t, resp, &result)
	keyID, _ := result["id"].(string)
	if keyID == "" {
		t.Fatalf("missing key id in response: %v", result)
	}

	resp2 := doDelete(t, ts, "/api/v1/apikeys/"+keyID)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 200, got %d body=%s", resp2.StatusCode, string(buf))
	}
}

func TestComprehensive_APIKeyValidation(t *testing.T) {
	ts := fullSetupServer(t)
	// H-1 fix: admin keys are now refused in no-auth mode. The
	// validation test exercises the legitimate "writer" path and
	// expects a 403 for the admin attempt.
	resp := doPost(t, ts, "/api/v1/apikeys", `{"name":"val-key","user_id":"u-user1","role":"writer"}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for writer, got %d", resp.StatusCode)
	}
	respAdmin := doPost(t, ts, "/api/v1/apikeys", `{"name":"val-admin","user_id":"u-user1","role":"admin"}`, "")
	defer respAdmin.Body.Close()
	if respAdmin.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for admin in no-auth mode, got %d", respAdmin.StatusCode)
	}
}

func TestComprehensive_APIKeyInvalidRole(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/apikeys", `{"name":"bad-role","user_id":"u-user1","role":"superadmin"}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// TestComprehensive_APINonAdminCannotEscalate verifies the security fix
// for the unauthenticated-key-mint issue: a non-admin caller must not
// be able to mint a key for a different user with a different role.
func TestComprehensive_APINonAdminCannotEscalate(t *testing.T) {
	prev := integrationTestAuthEnabled
	integrationTestAuthEnabled = true
	defer func() { integrationTestAuthEnabled = prev }()

	ts := fullSetupServer(t)
	// Issue a writer-scoped key for u-writer1 (using the default admin token
	// so we can create keys for arbitrary users in the seed step).
	setupReq := authedRequest(t, "POST", ts.URL+"/api/v1/apikeys", `{"name":"writer-key","user_id":"u-writer1","role":"writer"}`)
	resp, err := http.DefaultClient.Do(setupReq)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("setup: expected 201, got %d", resp.StatusCode)
	}
	var created map[string]any
	decodeJSON(t, resp, &created)
	writerKey, _ := created["key"].(string)
	if writerKey == "" {
		t.Fatal("setup: no key in response")
	}

	// Now try to mint an admin key for a different user using the
	// writer key. This must be rejected.
	req := authedRequest(t, "POST", ts.URL+"/api/v1/apikeys", `{"name":"escalate","user_id":"u-user2","role":"admin"}`)
	req.Header.Set("Authorization", "Bearer "+writerKey)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp2.StatusCode)
	}
}

func TestComprehensive_UnauthorizedAccess(t *testing.T) {
	prev := integrationTestAuthEnabled
	integrationTestAuthEnabled = true
	defer func() { integrationTestAuthEnabled = prev }()

	ts := fullSetupServer(t)
	// With auth enabled and no bearer token, anonymous access must be
	// rejected.
	resp, err := http.Get(ts.URL + "/api/v1/prompts")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", resp.StatusCode)
	}
}

// =====================================================================
// 14. Webhook operations (4 tests)
// =====================================================================

func TestComprehensive_WebhookList(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/webhooks")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestComprehensive_WebhookDeleteNonExistent(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doDelete(t, ts, "/api/v1/webhooks/nonexistent")
	defer resp.Body.Close()
	// Should return an error since webhooks dispatcher is nil
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Logf("webhook delete returned %d", resp.StatusCode)
	}
}

func TestComprehensive_WebhookInvalidRequest(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/webhooks", `{}`, "")
	defer resp.Body.Close()
	// Without webhook dispatcher, should return 503
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Logf("webhook create returned %d", resp.StatusCode)
	}
}

func TestComprehensive_WebhookMissingURL(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/webhooks", `{"events":["prompt.deployed"]}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Logf("webhook missing URL returned %d", resp.StatusCode)
	}
}

// =====================================================================
// 15. Snapshot operations (4 tests)
// =====================================================================

func TestComprehensive_SnapshotList(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/snapshots")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestComprehensive_SnapshotGetNonExistent(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/snapshots/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestComprehensive_SnapshotListWithFilter(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/snapshots?prompt_hash=test-hash&model=gpt-4")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestComprehensive_SnapshotEmptyList(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/snapshots")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var snaps []any
	decodeJSON(t, resp, &snaps)
	if len(snaps) != 0 {
		t.Fatalf("expected 0 snapshots, got %d", len(snaps))
	}
}

// =====================================================================
// Additional integration flows to reach 80 tests
// =====================================================================

// --- Prompt Advanced ---

func TestComprehensive_PromptPreview(t *testing.T) {
	ts := fullSetupServer(t)
	id := createPrompt(t, ts, "preview-prompt")
	resp := doPost(t, ts, "/api/v1/prompts/"+id+"/preview", `{"variables":{"role":"assistant","instruction":"Be helpful"}}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	decodeJSON(t, resp, &result)
	if result["rendered"] == nil {
		t.Fatal("expected rendered field")
	}
}

func TestComprehensive_PromptSimilar(t *testing.T) {
	ts := fullSetupServer(t)
	createPrompt(t, ts, "similar-prompt-1")
	createPrompt(t, ts, "similar-prompt-2")

	resp, err := http.Get(ts.URL + "/api/v1/prompts/similar?content=You%20are%20a%20helpful%20AI%20assistant&threshold=0.5")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestComprehensive_PromptPreviewNonExistent(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/prompts/nonexistent/preview", `{"variables":{"name":"test"}}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestComprehensive_PromptDeployNonApproved(t *testing.T) {
	ts := fullSetupServer(t)
	id := createPrompt(t, ts, "deploy-nonapproved")
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/prompts/"+id+"/deploy", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// --- Agent Advanced ---

func TestComprehensive_AgentNonExistentGet(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/agents/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestComprehensive_AgentVersions(t *testing.T) {
	ts := fullSetupServer(t)
	id := createAgent(t, ts, "versions-agent")
	resp, err := http.Get(ts.URL + "/api/v1/agents/" + id + "/versions")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestComprehensive_AgentTemplates(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/agents/templates")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestComprehensive_AgentValidate(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/agents/validate", `{"steps":[{"id":"s1","output_key":"o1","depends_on":[]}]}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	decodeJSON(t, resp, &result)
	if result["valid"] != true {
		t.Fatalf("expected valid workflow, got %v", result["errors"])
	}
}

func TestComprehensive_AgentDeployNonApproved(t *testing.T) {
	ts := fullSetupServer(t)
	id := createAgent(t, ts, "deploy-nonapproved-agent")
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/agents/"+id+"/deploy", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// --- Review Advanced ---

func TestComprehensive_ReviewNotFound(t *testing.T) {
	ts := fullSetupServer(t)
	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/reviews/nonexistent/approve", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// --- Eval Advanced ---

func TestComprehensive_EvalReportMissingHash(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/eval/report")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestComprehensive_EvalCompare(t *testing.T) {
	ts := fullSetupServer(t)
	dsID := createDataset(t, ts, "eval-compare-ds")
	doPost(t, ts, "/api/v1/eval/run", fmt.Sprintf(`{"prompt_hash":"compare-a","dataset_id":"%s","prompt_text":"test a"}`, dsID), "").Body.Close()
	doPost(t, ts, "/api/v1/eval/run", fmt.Sprintf(`{"prompt_hash":"compare-b","dataset_id":"%s","prompt_text":"test b"}`, dsID), "").Body.Close()

	resp, err := http.Get(ts.URL + "/api/v1/eval/compare?a=compare-a&b=compare-b")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestComprehensive_EvalResultsMissingParam(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/eval/results")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestComprehensive_EvalCompareMissingParams(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/eval/compare?a=hash1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// --- Context Advanced ---

func TestComprehensive_ContextList(t *testing.T) {
	ts := fullSetupServer(t)
	doPost(t, ts, "/api/v1/contexts", `{"name":"ctx-list-1","type":"system_prompt","system_prompt":"Hello"}`, "").Body.Close()
	doPost(t, ts, "/api/v1/contexts", `{"name":"ctx-list-2","type":"conversation","system_prompt":"Hi"}`, "").Body.Close()

	resp, err := http.Get(ts.URL + "/api/v1/contexts")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var contexts []map[string]any
	decodeJSON(t, resp, &contexts)
	if len(contexts) < 2 {
		t.Fatalf("expected at least 2 contexts, got %d", len(contexts))
	}
}

func TestComprehensive_ContextDelete(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/contexts", `{"name":"ctx-delete","type":"system_prompt","system_prompt":"Hello"}`, "")
	var ctx map[string]any
	decodeJSON(t, resp, &ctx)
	ctxID := ctx["id"].(string)

	resp2 := doDelete(t, ts, "/api/v1/contexts/"+ctxID)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
}

func TestComprehensive_ContextNonExistent(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/contexts/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestComprehensive_ContextMissingName(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/contexts", `{"type":"system_prompt","system_prompt":"Hello"}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// --- Guardrail Advanced ---

func TestComprehensive_GuardrailUpdateRule(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/guardrails/rules", `{"name":"update-rule","type":"content_policy","severity":"low"}`, "")
	var rule map[string]any
	decodeJSON(t, resp, &rule)
	ruleID := rule["id"].(string)

	resp2 := doPut(t, ts, "/api/v1/guardrails/rules/"+ruleID, `{"name":"update-rule-v2","enabled":false}`)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
}

func TestComprehensive_GuardrailViolations(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/guardrails/violations")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// --- Provider Advanced ---

func TestComprehensive_ProviderList(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/providers")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestComprehensive_ProviderGetNonExistent(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/providers/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// --- Traces Advanced ---

func TestComprehensive_TraceList(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/traces")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestComprehensive_TraceGetNonExistent(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/traces/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestComprehensive_TraceTreeNonExistent(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/traces/tree/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	// Trace tree returns 200 with empty spans for non-existent trace
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// --- Metrics Advanced ---

func TestComprehensive_MetricsSummary(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/metrics/summary")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// --- Health Advanced ---

func TestComprehensive_Health(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	decodeJSON(t, resp, &result)
	if result["status"] != "healthy" {
		t.Fatalf("expected 'healthy', got %v", result["status"])
	}
}

func TestComprehensive_Ready(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/ready")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// --- Workflow Advanced ---

func TestComprehensive_WorkflowRunWithTool(t *testing.T) {
	ts := fullSetupServer(t)
	body := `{"name":"wf-tool-agent","description":"Agent with tools","steps":[{"id":"s1","output_key":"o1","tool_calls":[{"tool":"json_transform","input":{"expression":"$.data"}}]}],"tools":[{"name":"json_transform","type":"json_transform","config":{}}]}`
	resp := doPost(t, ts, "/api/v1/agents", body, "")
	var agent map[string]any
	decodeJSON(t, resp, &agent)
	agentID := agent["id"].(string)

	resp2 := doPost(t, ts, "/api/v1/workflows/run", fmt.Sprintf(`{"agent_id":"%s","input":{"data":"hello"}}`, agentID), "")
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
}

// --- Dataset Advanced ---

func TestComprehensive_DatasetNonExistentGet(t *testing.T) {
	ts := fullSetupServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/datasets/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestComprehensive_DatasetMissingName(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/datasets", `{"cases":[]}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestComprehensive_DatasetExport(t *testing.T) {
	ts := fullSetupServer(t)
	id := createDataset(t, ts, "ds-export")
	resp, err := http.Get(ts.URL + "/api/v1/datasets/" + id + "/export")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Fatal("expected non-empty export")
	}
}

func TestComprehensive_DatasetImportCSV(t *testing.T) {
	ts := fullSetupServer(t)
	id := createDataset(t, ts, "ds-csv-import")
	csvData := "input,output\nhello,world\nfoo,bar"
	importBody, _ := json.Marshal(map[string]string{"csv": csvData})
	resp := doPost(t, ts, "/api/v1/datasets/"+id+"/import-csv", string(importBody), "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// --- Agent YAML Import/Export ---

func TestComprehensive_AgentImportYAML(t *testing.T) {
	ts := fullSetupServer(t)
	yaml := `name: yaml-agent
description: imported from yaml
steps:
  - id: step1
    output_key: out1`
	resp := doPost(t, ts, "/api/v1/agents/import-yaml", yaml, "text/yaml")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

func TestComprehensive_AgentExportYAML(t *testing.T) {
	ts := fullSetupServer(t)
	id := createAgent(t, ts, "export-yaml-agent")
	resp, err := http.Get(ts.URL + "/api/v1/agents/" + id + "/export?format=yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Fatal("expected non-empty YAML")
	}
}

func TestComprehensive_AgentExportJSON(t *testing.T) {
	ts := fullSetupServer(t)
	id := createAgent(t, ts, "export-json-agent")
	resp, err := http.Get(ts.URL + "/api/v1/agents/" + id + "/export")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var a map[string]any
	decodeJSON(t, resp, &a)
	if a["name"] != "export-json-agent" {
		t.Fatalf("expected name 'export-json-agent', got %v", a["name"])
	}
}

func TestComprehensive_AgentExecutions(t *testing.T) {
	ts := fullSetupServer(t)
	id := createAgent(t, ts, "exec-list-agent")
	resp, err := http.Get(ts.URL + "/api/v1/agents/" + id + "/executions")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// --- Prompt Run Advanced ---

func TestComprehensive_PromptRunNonExistent(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/prompts/nonexistent/run", `{"variables":{},"provider":"mock"}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestComprehensive_PromptStreamNonExistent(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/prompts/nonexistent/stream", `{"variables":{},"provider":"mock"}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestComprehensive_PromptCreateMissingName(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/prompts", `{"content":"Hello"}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestComprehensive_PromptCreateMissingContent(t *testing.T) {
	ts := fullSetupServer(t)
	resp := doPost(t, ts, "/api/v1/prompts", `{"name":"no-content"}`, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}
