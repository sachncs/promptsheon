package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// roundTripFunc lets tests substitute a custom transport without
// spinning up an httptest server. The closure receives the
// outgoing request and returns the response it wants the SDK to
// see. The httptest.NewServer wrapper in each test gives the
// closure a real URL.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestNewDefaultsTo30sTimeout(t *testing.T) {
	c := New("http://example.invalid", "k")
	if c.httpClient.Timeout != 30*1_000_000_000 {
		t.Fatalf("expected 30s timeout, got %v", c.httpClient.Timeout)
	}
}

func TestNewWithHTTPNilUsesDefault(t *testing.T) {
	c := NewWithHTTP("http://example.invalid", "k", nil)
	if c.httpClient != http.DefaultClient {
		t.Fatal("expected http.DefaultClient when nil is passed")
	}
}

func TestNewWithHTTPCustomTransport(t *testing.T) {
	want := &http.Client{}
	c := NewWithHTTP("http://example.invalid", "k", want)
	if c.httpClient != want {
		t.Fatal("expected the provided http.Client to be used verbatim")
	}
}

func TestAuthHeaderSentWhenKeyPresent(t *testing.T) {
	var seenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "ps_test_secret")
	if _, err := c.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
	if seenAuth != "Bearer ps_test_secret" {
		t.Fatalf("expected Bearer auth header, got %q", seenAuth)
	}
}

func TestAuthHeaderOmittedWhenKeyEmpty(t *testing.T) {
	var seenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	if _, err := c.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
	if seenAuth != "" {
		t.Fatalf("expected no Authorization header, got %q", seenAuth)
	}
}

func TestAPIErrorDecodedFromCanonicalBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"prompt name is required"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	_, err := c.GetPrompt(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error from 4xx response")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Status != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", apiErr.Status)
	}
	if apiErr.Message != "prompt name is required" {
		t.Fatalf("expected decoded message, got %q", apiErr.Message)
	}
}

func TestAPIErrorDecodedFromLegacyMessageField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	_, err := c.ListPrompts(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Message != "unauthorized" {
		t.Fatalf("expected 'unauthorized', got %q", apiErr.Message)
	}
}

func TestAPIErrorFallsBackToRawBodyForPlainText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("upstream proxy error"))
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	_, err := c.Health(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Status != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", apiErr.Status)
	}
	if !strings.Contains(apiErr.Message, "upstream proxy error") {
		t.Fatalf("expected raw body in message, got %q", apiErr.Message)
	}
}

func TestAPIErrorHandlesEmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	_, err := c.Health(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Message != "" {
		t.Fatalf("expected empty message, got %q", apiErr.Message)
	}
	if !strings.Contains(apiErr.Error(), "no message body") {
		t.Fatalf("expected 'no message body' in Error(), got %q", apiErr.Error())
	}
}

func TestPromptCRUDRoundtrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/prompts":
			_, _ = w.Write([]byte(`[{"id":"p1","name":"greeting","content":"hi","version":1,"status":"draft","created_by":"u1","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"}]`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/prompts/"):
			_, _ = w.Write([]byte(`{"id":"p1","name":"greeting","content":"hi","version":1,"status":"draft","created_by":"u1","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/prompts":
			var req CreatePromptRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decode body: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusCreated)
			body, _ := json.Marshal(Prompt{
				ID:      "p2",
				Name:    req.Name,
				Content: req.Content,
				Version: 1,
				Status:  "draft",
			})
			_, _ = w.Write(body)
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "k")

	// List
	prompts, err := c.ListPrompts(context.Background())
	if err != nil {
		t.Fatalf("ListPrompts: %v", err)
	}
	if len(prompts) != 1 || prompts[0].ID != "p1" {
		t.Fatalf("unexpected list result: %+v", prompts)
	}

	// Get
	p, err := c.GetPrompt(context.Background(), "p1")
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if p.Name != "greeting" {
		t.Fatalf("unexpected prompt: %+v", p)
	}

	// Create
	created, err := c.CreatePrompt(context.Background(), &CreatePromptRequest{
		Name:    "farewell",
		Content: "bye",
	})
	if err != nil {
		t.Fatalf("CreatePrompt: %v", err)
	}
	if created.ID != "p2" {
		t.Fatalf("expected id p2, got %q", created.ID)
	}

	// Delete
	if err := c.DeletePrompt(context.Background(), "p1"); err != nil {
		t.Fatalf("DeletePrompt: %v", err)
	}
}

func TestRunPromptRoundtrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/prompts/p1/run" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"content":"hi there","model":"gpt-4","usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5},"latency_ms":120}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	resp, err := c.RunPrompt(context.Background(), "p1", &RunPromptRequest{
		Variables: map[string]string{"name": "world"},
	})
	if err != nil {
		t.Fatalf("RunPrompt: %v", err)
	}
	if resp.Content != "hi there" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
	if resp.Usage.TotalTokens != 5 {
		t.Fatalf("unexpected usage: %+v", resp.Usage)
	}
	if resp.LatencyMs != 120 {
		t.Fatalf("unexpected latency: %d", resp.LatencyMs)
	}
}

func TestContextCancellationPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled
	_, err := c.Health(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestCustomRoundTripperInvoked(t *testing.T) {
	var invoked bool
	var seenPath string
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		invoked = true
		seenPath = r.URL.Path
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"status":"healthy"}`)),
			Header:     make(http.Header),
		}, nil
	})
	c := NewWithHTTP("http://example.invalid", "k", &http.Client{Transport: rt})
	if _, err := c.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
	if !invoked {
		t.Fatal("expected the custom transport to receive the request")
	}
	if seenPath != "/health" {
		t.Fatalf("expected /health, got %q", seenPath)
	}
}

func TestUpdateDeployArchiveRoundtrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/prompts/p1":
			var req UpdatePromptRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			body, _ := json.Marshal(Prompt{
				ID:      "p1",
				Name:    *req.Name,
				Content: "hello",
				Version: 2,
				Status:  "draft",
			})
			_, _ = w.Write(body)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/prompts/p1/deploy":
			_, _ = w.Write([]byte(`{"id":"p1","name":"greeting","content":"hi","version":2,"status":"active","created_by":"u1","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/prompts/p1/archive":
			_, _ = w.Write([]byte(`{"id":"p1","name":"greeting","content":"hi","version":2,"status":"archived","created_by":"u1","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	ctx := context.Background()

	name := "greeting-v2"
	updated, err := c.UpdatePrompt(ctx, "p1", &UpdatePromptRequest{
		Name: &name,
	})
	if err != nil {
		t.Fatalf("UpdatePrompt: %v", err)
	}
	if updated.Version != 2 {
		t.Fatalf("expected version 2, got %d", updated.Version)
	}

	deployed, err := c.DeployPrompt(ctx, "p1")
	if err != nil {
		t.Fatalf("DeployPrompt: %v", err)
	}
	if deployed.Status != "active" {
		t.Fatalf("expected active status, got %q", deployed.Status)
	}

	archived, err := c.ArchivePrompt(ctx, "p1")
	if err != nil {
		t.Fatalf("ArchivePrompt: %v", err)
	}
	if archived.Status != "archived" {
		t.Fatalf("expected archived status, got %q", archived.Status)
	}
}

func TestAgentsRoundtrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/agents":
			_, _ = w.Write([]byte(`[{"id":"a1","name":"greeter","description":"says hello","steps":[{"id":"s1","name":"step1","tool":"echo","config":{"msg":"hi"},"depends_on":null}],"tools":[{"name":"echo","type":"command","config":{"cmd":"echo"}}],"status":"active"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/agents/a1":
			_, _ = w.Write([]byte(`{"id":"a1","name":"greeter","description":"says hello","steps":[{"id":"s1","name":"step1","tool":"echo","config":{"msg":"hi"},"depends_on":null}],"tools":[{"name":"echo","type":"command","config":{"cmd":"echo"}}],"status":"active"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	ctx := context.Background()

	agents, err := c.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != 1 || agents[0].ID != "a1" {
		t.Fatalf("unexpected agents: %+v", agents)
	}

	agent, err := c.GetAgent(ctx, "a1")
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if agent.Name != "greeter" {
		t.Fatalf("unexpected agent: %+v", agent)
	}
}

func TestListProviders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/providers" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"providers":["openai","anthropic"]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	providers, err := c.ListProviders(context.Background())
	if err != nil {
		t.Fatalf("ListProviders: %v", err)
	}
	if len(providers) != 2 || providers[0] != "openai" {
		t.Fatalf("unexpected providers: %v", providers)
	}
}

func TestAPIErrorMessage(t *testing.T) {
	apiErr := &APIError{Status: 400, Message: "bad request"}
	msg := apiErr.Error()
	if !strings.Contains(msg, "bad request") || !strings.Contains(msg, "400") {
		t.Fatalf("unexpected error message: %q", msg)
	}
}

func TestDoTransportError(t *testing.T) {
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, io.ErrUnexpectedEOF
	})
	c := NewWithHTTP("http://example.invalid", "k", &http.Client{Transport: rt})
	_, err := c.Health(context.Background())
	if err == nil {
		t.Fatal("expected error from transport failure")
	}
}

func TestCreatePromptServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal error"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	_, err := c.CreatePrompt(context.Background(), &CreatePromptRequest{
		Name:    "test",
		Content: "test",
	})
	if err == nil {
		t.Fatal("expected error from 5xx response")
	}
}

func TestRunPromptServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal error"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	_, err := c.RunPrompt(context.Background(), "p1", &RunPromptRequest{})
	if err == nil {
		t.Fatal("expected error from 5xx response")
	}
}

func TestCreatePromptWithAllFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/prompts" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req CreatePromptRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		body, _ := json.Marshal(Prompt{
			ID:      "p3",
			Name:    req.Name,
			Content: req.Content,
			Version: 1,
			Status:  "draft",
		})
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	binding := &ProviderBinding{Provider: "openai", Model: "gpt-4"}
	created, err := c.CreatePrompt(context.Background(), &CreatePromptRequest{
		Name:    "complex",
		Content: "hello {{name}}",
		Variables: []Variable{
			{Name: "name", Type: "string", Required: true, Description: "the name"},
		},
		Tags:      []string{"test", "demo"},
		ModelHint: "gpt-4",
		Binding:   binding,
	})
	if err != nil {
		t.Fatalf("CreatePrompt: %v", err)
	}
	if created.Name != "complex" {
		t.Fatalf("unexpected name: %q", created.Name)
	}
}

func TestDecodeErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	ctx := context.Background()

	_, err := c.ListPrompts(ctx)
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("expected decode error for ListPrompts, got: %v", err)
	}

	_, err = c.GetPrompt(ctx, "p1")
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("expected decode error for GetPrompt, got: %v", err)
	}

	_, err = c.CreatePrompt(ctx, &CreatePromptRequest{Name: "n", Content: "c"})
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("expected decode error for CreatePrompt, got: %v", err)
	}

	_, err = c.UpdatePrompt(ctx, "p1", &UpdatePromptRequest{})
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("expected decode error for UpdatePrompt, got: %v", err)
	}

	_, err = c.RunPrompt(ctx, "p1", &RunPromptRequest{})
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("expected decode error for RunPrompt, got: %v", err)
	}

	_, err = c.DeployPrompt(ctx, "p1")
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("expected decode error for DeployPrompt, got: %v", err)
	}

	_, err = c.ArchivePrompt(ctx, "p1")
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("expected decode error for ArchivePrompt, got: %v", err)
	}

	_, err = c.ListAgents(ctx)
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("expected decode error for ListAgents, got: %v", err)
	}

	_, err = c.GetAgent(ctx, "a1")
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("expected decode error for GetAgent, got: %v", err)
	}

	_, err = c.ListProviders(ctx)
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("expected decode error for ListProviders, got: %v", err)
	}

	_, err = c.Health(ctx)
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("expected decode error for Health, got: %v", err)
	}
}

func TestServerErrorForAllEndpoints(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"server error"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	ctx := context.Background()

	_, err := c.ListPrompts(ctx)
	if err == nil || !strings.Contains(err.Error(), "server error") {
		t.Fatalf("expected server error for ListPrompts, got: %v", err)
	}

	_, err = c.GetPrompt(ctx, "p1")
	if err == nil || !strings.Contains(err.Error(), "server error") {
		t.Fatalf("expected server error for GetPrompt, got: %v", err)
	}

	_, err = c.CreatePrompt(ctx, &CreatePromptRequest{Name: "n", Content: "c"})
	if err == nil || !strings.Contains(err.Error(), "server error") {
		t.Fatalf("expected server error for CreatePrompt, got: %v", err)
	}

	_, err = c.UpdatePrompt(ctx, "p1", &UpdatePromptRequest{})
	if err == nil || !strings.Contains(err.Error(), "server error") {
		t.Fatalf("expected server error for UpdatePrompt, got: %v", err)
	}

	err = c.DeletePrompt(ctx, "p1")
	if err == nil || !strings.Contains(err.Error(), "server error") {
		t.Fatalf("expected server error for DeletePrompt, got: %v", err)
	}

	_, err = c.RunPrompt(ctx, "p1", &RunPromptRequest{})
	if err == nil || !strings.Contains(err.Error(), "server error") {
		t.Fatalf("expected server error for RunPrompt, got: %v", err)
	}

	_, err = c.DeployPrompt(ctx, "p1")
	if err == nil || !strings.Contains(err.Error(), "server error") {
		t.Fatalf("expected server error for DeployPrompt, got: %v", err)
	}

	_, err = c.ArchivePrompt(ctx, "p1")
	if err == nil || !strings.Contains(err.Error(), "server error") {
		t.Fatalf("expected server error for ArchivePrompt, got: %v", err)
	}

	_, err = c.ListAgents(ctx)
	if err == nil || !strings.Contains(err.Error(), "server error") {
		t.Fatalf("expected server error for ListAgents, got: %v", err)
	}

	_, err = c.GetAgent(ctx, "a1")
	if err == nil || !strings.Contains(err.Error(), "server error") {
		t.Fatalf("expected server error for GetAgent, got: %v", err)
	}

	_, err = c.ListProviders(ctx)
	if err == nil || !strings.Contains(err.Error(), "server error") {
		t.Fatalf("expected server error for ListProviders, got: %v", err)
	}

	_, err = c.Health(ctx)
	if err == nil || !strings.Contains(err.Error(), "server error") {
		t.Fatalf("expected server error for Health, got: %v", err)
	}
}

func TestDoMarshalBodyError(t *testing.T) {
	c := New("http://example.invalid", "k")
	_, err := c.do(context.Background(), "POST", "/test", make(chan int))
	if err == nil || !strings.Contains(err.Error(), "marshal body") {
		t.Fatalf("expected marshal body error, got: %v", err)
	}
}

func TestDoCreateRequestError(t *testing.T) {
	c := New("http://example.invalid", "k")
	_, err := c.do(context.Background(), "GET", "/test%", nil)
	if err == nil || !strings.Contains(err.Error(), "create request") {
		t.Fatalf("expected create request error, got: %v", err)
	}
}

func TestDoReadResponseError(t *testing.T) {
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(&errReader{err: io.ErrUnexpectedEOF}),
			Header:     make(http.Header),
		}, nil
	})
	c := NewWithHTTP("http://example.invalid", "k", &http.Client{Transport: rt})
	_, err := c.Health(context.Background())
	if err == nil || !strings.Contains(err.Error(), "read response") {
		t.Fatalf("expected read response error, got: %v", err)
	}
}

type errReader struct {
	err error
}

func (r *errReader) Read(_ []byte) (int, error) {
	return 0, r.err
}
