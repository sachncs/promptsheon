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
