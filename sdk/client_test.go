package sdk

import (
	"context"
	"encoding/json"
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
		t.Fatalf("auth header = %q want %q", seenAuth, "Bearer ps_test_secret")
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
		t.Fatalf("expected no auth header, got %q", seenAuth)
	}
}

func TestDecodeAPIErrorCanonicalShape(t *testing.T) {
	err := decodeAPIError(400, []byte(`{"error":"bad input"}`))
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Status != 400 || apiErr.Message != "bad input" {
		t.Fatalf("got %+v", apiErr)
	}
}

func TestDecodeAPIErrorLegacyShape(t *testing.T) {
	err := decodeAPIError(404, []byte(`{"message":"missing"}`))
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Message != "missing" {
		t.Fatalf("got %+v", apiErr)
	}
}

func TestDecodeAPIErrorPlainText(t *testing.T) {
	err := decodeAPIError(500, []byte("internal server error"))
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if !strings.Contains(apiErr.Message, "internal") {
		t.Fatalf("got %+v", apiErr)
	}
}

func TestDecodeAPIErrorEmptyBody(t *testing.T) {
	err := decodeAPIError(500, nil)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Message != "" {
		t.Fatalf("got %+v", apiErr)
	}
}

func TestAPIErrorMessage(t *testing.T) {
	e := &APIError{Status: 409, Message: "conflict"}
	want := "api error (status 409): conflict"
	if e.Error() != want {
		t.Fatalf("got %q want %q", e.Error(), want)
	}
}

func TestAPIErrorEmptyMessage(t *testing.T) {
	e := &APIError{Status: 500}
	want := "api error (status 500): no message body"
	if e.Error() != want {
		t.Fatalf("got %q want %q", e.Error(), want)
	}
}

func TestCreateReleaseRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/releases") {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["environment"] != "prod" {
			t.Fatalf("env = %v want prod", body["environment"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"rel-1","capability_id":"c1","capability_version":1,"environment":"prod","status":"pending","created_by":"alice","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	got, err := c.CreateRelease(context.Background(), "v1", CreateReleaseRequest{Environment: "prod"})
	if err != nil {
		t.Fatalf("CreateRelease: %v", err)
	}
	if got.ID != "rel-1" || got.Environment != "prod" || got.Status != "pending" {
		t.Fatalf("got %+v", got)
	}
}

func TestVoteRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/votes") {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"release_id":"rel-1","votes":[{"identity":"bob","decision":"approve","timestamp":"2026-01-01T00:00:00Z"}],"updated_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	got, err := c.Vote(context.Background(), "rel-1", VoteRequest{Identity: "bob", Decision: "approve"})
	if err != nil {
		t.Fatalf("Vote: %v", err)
	}
	if got.ReleaseID != "rel-1" || len(got.Votes) != 1 || got.Votes[0].Identity != "bob" {
		t.Fatalf("got %+v", got)
	}
}

func TestActivateRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/activate") {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"rel-1","environment":"prod","status":"active","activated_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	got, err := c.Activate(context.Background(), "rel-1")
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if got.Status != "active" {
		t.Fatalf("got %+v", got)
	}
}

func TestActivateQuorumConflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"approval: quorum not satisfied"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	_, err := c.Activate(context.Background(), "rel-1")
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Status != http.StatusConflict {
		t.Fatalf("expected 409 APIError, got %v", err)
	}
}

func TestInvokeRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/invoke") {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"model":"claude"`) {
			t.Fatalf("body did not include model: %s", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"e1","capability_version_id":"v1","timestamp":"2026-01-01T00:00:00Z","model":"claude","provider":""}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	got, err := c.Invoke(context.Background(), "rel-1", InvokeRequest{Model: "claude", Inputs: map[string]any{"q": "hi"}})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if got.Model != "claude" {
		t.Fatalf("got %+v", got)
	}
}

func TestHarnessSDKE2E(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/v1/capabilities/c1/datasets":
			_, _ = w.Write([]byte(`{"id":"ds1","capability_id":"c1","name":"g"}`))
		case r.Method == "GET" && r.URL.Path == "/api/v1/datasets/ds1":
			_, _ = w.Write([]byte(`{"id":"ds1","capability_id":"c1","name":"g","cases":[{"id":"ca1","dataset_id":"ds1","seq":0,"inputs":"hi","expected":"hi"}]}`))
		case r.Method == "PUT" && r.URL.Path == "/api/v1/datasets/ds1/cases":
			_, _ = w.Write([]byte(`{"cases":[{"id":"ca1"}]}`))
		case r.Method == "POST" && r.URL.Path == "/api/v1/capabilities/c1/preconditions":
			_, _ = w.Write([]byte(`{"id":"p1","capability_id":"c1","name":"go-test","command":"go test ./...","timeout_sec":60,"enabled":true}`))
		case r.Method == "POST" && r.URL.Path == "/api/v1/releases/r1/evals":
			_, _ = w.Write([]byte(`{"id":"erun1","release_id":"r1","dataset_id":"ds1","scorer":"exact_match","score":1.0,"passed":1,"failed":0,"total":1,"status":"passed"}`))
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	d, err := c.CreateDataset(context.Background(), "c1", CreateDatasetRequest{Name: "g"})
	if err != nil {
		t.Fatalf("CreateDataset: %v", err)
	}
	if d.ID != "ds1" {
		t.Fatalf("dataset id = %q", d.ID)
	}

	got, err := c.GetDataset(context.Background(), "ds1")
	if err != nil {
		t.Fatalf("GetDataset: %v", err)
	}
	if len(got.Cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(got.Cases))
	}

	if err := c.PutCases(context.Background(), "ds1", []DatasetCase{{Seq: 0}}); err != nil {
		t.Fatalf("PutCases: %v", err)
	}

	p, err := c.CreatePrecondition(context.Background(), "c1", CreatePreconditionRequest{
		Name: "go-test", Command: "go test ./...", TimeoutSec: 60,
	})
	if err != nil {
		t.Fatalf("CreatePrecondition: %v", err)
	}
	if p.Command != "go test ./..." {
		t.Fatalf("command = %q", p.Command)
	}

	run, err := c.RunEval(context.Background(), "r1", RunEvalRequest{DatasetID: "ds1", Scorer: "exact_match"})
	if err != nil {
		t.Fatalf("RunEval: %v", err)
	}
	if run.Status != "passed" || run.Score != 1.0 {
		t.Fatalf("run = %+v", run)
	}
}

func TestApproveAndInvokeConvenience(t *testing.T) {
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/votes"):
			_, _ = w.Write([]byte(`{"release_id":"rel-1","votes":[{"identity":"bob","decision":"approve"}],"updated_at":"2026-01-01T00:00:00Z"}`))
		case strings.HasSuffix(r.URL.Path, "/activate"):
			_, _ = w.Write([]byte(`{"id":"rel-1","status":"active","environment":"prod"}`))
		case strings.HasSuffix(r.URL.Path, "/invoke"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"e1","model":"claude"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	if _, err := c.ApproveAndInvoke(context.Background(), "rel-1", "bob", InvokeRequest{Model: "claude"}); err != nil {
		t.Fatalf("ApproveAndInvoke: %v", err)
	}
	if len(calls) != 3 || calls[0] != "POST /api/v1/releases/rel-1/votes" ||
		calls[1] != "POST /api/v1/releases/rel-1/activate" ||
		calls[2] != "POST /api/v1/releases/rel-1/invoke" {
		t.Fatalf("call order: %v", calls)
	}
}
