// Package promptsheon is the public Go SDK for the Promptsheon
// REST API.
//
// v1.0.0 establishes this package as the canonical public surface.
// The //go:build promptsheon fence prevents internal types from
// leaking. Consumers opt in with:
//
//	GOFLAGS=-tags=promptsheon go build ./...
//
// The legacy github.com/sachncs/promptsheon/sdk import path
// was removed in v1.0.0; this package is the only public Go
// surface.
package promptsheon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sachncs/promptsheon/errf"
)

// Client is a Promptsheon API client.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New creates a new Promptsheon API client with a 30-second
// per-request timeout. Callers that need a different timeout or
// want to inject a custom http.RoundTripper (for example to add
// retry middleware or a metrics-instrumented transport) should
// use NewWithHTTP.
func New(baseURL, apiKey string) *Client {
	return NewWithHTTP(baseURL, apiKey, &http.Client{Timeout: 30 * time.Second})
}

// NewWithHTTP creates a new Promptsheon API client using the
// provided http.Client. The caller retains ownership of the
// transport; the SDK does not mutate it. Passing nil falls back
// to http.DefaultClient.
func NewWithHTTP(baseURL, apiKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: httpClient,
	}
}

// APIError represents an error response from the API. The server
// returns a JSON body of the form {"error": "..."} on most 4xx
// and 5xx responses; the SDK decodes that into Message. When the
// body is empty or not JSON, Message is the raw body string so
// the operator can still see what the server said.
type APIError struct {
	Status  int    `json:"-"`
	Message string `json:"error"`
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("api error (status %d): no message body", e.Status)
	}
	return fmt.Sprintf("api error (status %d): %s", e.Status, e.Message)
}

func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, errf.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, errf.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errf.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errf.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, decodeAPIError(resp.StatusCode, data)
	}

	return data, nil
}

// decodeAPIError builds an APIError from the server's error
// body. The server typically returns {"error": "..."}, but
// intermediate proxies or older versions may return plain text
// or a different JSON shape; we try to extract the most useful
// string in each case rather than dropping the body on the
// floor.
func decodeAPIError(status int, body []byte) error {
	apiErr := APIError{Status: status}
	if len(body) == 0 {
		return &apiErr
	}
	// First try the canonical {"error": "..."} shape.
	var wrapper struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &wrapper); err == nil && wrapper.Error != "" {
		apiErr.Message = wrapper.Error
		return &apiErr
	}
	// Fall back to the legacy {"message": "..."} shape some
	// handlers still emit.
	var legacy struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &legacy); err == nil && legacy.Message != "" {
		apiErr.Message = legacy.Message
		return &apiErr
	}
	// Non-JSON body: surface the raw bytes (trimmed) so the
	// operator can see what the server actually said.
	apiErr.Message = strings.TrimSpace(string(body))
	return &apiErr
}

// --- Health ---

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status string `json:"status"`
}

// Health checks the server health status.
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	data, err := c.do(ctx, "GET", "/health", nil)
	if err != nil {
		return nil, err
	}
	var h HealthResponse
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, errf.Errorf("decode response: %w", err)
	}
	return &h, nil
}

// --- Providers ---

// ProviderInfo holds metadata about an LLM provider.
type ProviderInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// ProvidersResponse holds the list of provider names.
type ProvidersResponse struct {
	Providers []string `json:"providers"`
}

// ListProviders returns the names of all configured LLM providers.
func (c *Client) ListProviders(ctx context.Context) ([]string, error) {
	data, err := c.do(ctx, "GET", "/api/v1/providers", nil)
	if err != nil {
		return nil, err
	}
	var resp ProvidersResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, errf.Errorf("decode response: %w", err)
	}
	return resp.Providers, nil
}

// --- Capability lifecycle ---

// Workspace is a Promptsheon tenant workspace.
type Workspace struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Organization string    `json:"organization,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Project is a logical grouping of capabilities.
type Project struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Capability is the unit of business outcome.
type Capability struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Owner       string    `json:"owner,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ArtifactRef is the content-addressed reference to an immutable artifact.
type ArtifactRef struct {
	Kind string `json:"kind"`
	Hash string `json:"hash"`
}

// Manifest is the composition of artifacts that defines a Version.
type Manifest struct {
	Prompt        ArtifactRef   `json:"prompt"`
	ModelPolicy   ArtifactRef   `json:"model_policy"`
	RuntimePolicy ArtifactRef   `json:"runtime_policy"`
	Context       ArtifactRef   `json:"context_contract"`
	Memory        ArtifactRef   `json:"memory"`
	Guardrails    []ArtifactRef `json:"guardrails,omitempty"`
	Tools         []ArtifactRef `json:"tools,omitempty"`
	Knowledge     []ArtifactRef `json:"knowledge_sources,omitempty"`
	MCPServers    []ArtifactRef `json:"mcp_servers,omitempty"`
}

// Version is an immutable build of a Capability.
type Version struct {
	ID           string    `json:"id"`
	CapabilityID string    `json:"capability_id"`
	Version      int       `json:"version"`
	Manifest     Manifest  `json:"manifest"`
	ManifestHash string    `json:"manifest_hash,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	CreatedBy    string    `json:"created_by"`
}

// Release is the approved pointer from a Capability Version to a target Environment.
type Release struct {
	ID                string     `json:"id"`
	CapabilityID      string     `json:"capability_id"`
	CapabilityVersion int        `json:"capability_version"`
	Manifest          Manifest   `json:"manifest"`
	Environment       string     `json:"environment"`
	Status            string     `json:"status"`
	ApprovedBy        []string   `json:"approved_by,omitempty"`
	SupersededBy      string     `json:"superseded_by,omitempty"`
	ReplacesReleaseID string     `json:"replaces_release_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	CreatedBy         string     `json:"created_by"`
	ActivatedAt       *time.Time `json:"activated_at,omitempty"`
	SupersededAt      *time.Time `json:"superseded_at,omitempty"`
}

// Vote is one identity's recorded position on a Release.
type Vote struct {
	Identity  string    `json:"identity"`
	Decision  string    `json:"decision"`
	Reason    string    `json:"reason,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// Approval is the trail of votes against a Release.
type Approval struct {
	ReleaseID string    `json:"release_id"`
	Votes     []Vote    `json:"votes"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Execution is the recorded outcome of a single Release invocation.
type Execution struct {
	ID                  string         `json:"id"`
	CapabilityVersionID string         `json:"capability_version_id"`
	Timestamp           time.Time      `json:"timestamp"`
	Inputs              map[string]any `json:"inputs,omitempty"`
	Outputs             map[string]any `json:"outputs,omitempty"`
	Model               string         `json:"model"`
	Provider            string         `json:"provider"`
}

// CreateWorkspaceRequest is the request body for creating a Workspace.
type CreateWorkspaceRequest struct {
	Name         string `json:"name"`
	Organization string `json:"organization,omitempty"`
}

// CreateCapabilityRequest is the request body for creating a Capability.
type CreateCapabilityRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Owner       string `json:"owner,omitempty"`
}

// AddVersionRequest is the request body for creating a Version.
type AddVersionRequest struct {
	Version  int      `json:"version"`
	Manifest Manifest `json:"manifest"`
}

// CreateReleaseRequest is the request body for creating a Release.
type CreateReleaseRequest struct {
	Environment string `json:"environment"`
}

// VoteRequest is the request body for recording a vote.
type VoteRequest struct {
	Identity string `json:"identity,omitempty"`
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
}

// InvokeRequest is the request body for invoking a Release.
type InvokeRequest struct {
	Inputs   map[string]any `json:"inputs,omitempty"`
	Model    string         `json:"model"`
	Provider string         `json:"provider"`
}

// --- Workspace / Project / Capability ---

func (c *Client) CreateWorkspace(ctx context.Context, req CreateWorkspaceRequest) (*Workspace, error) {
	data, err := c.do(ctx, "POST", "/api/v1/workspaces", req)
	if err != nil {
		return nil, err
	}
	var w Workspace
	if err := json.Unmarshal(data, &w); err != nil {
		return nil, err
	}
	return &w, nil
}

func (c *Client) CreateCapability(ctx context.Context, projectID string, req CreateCapabilityRequest) (*Capability, error) {
	data, err := c.do(ctx, "POST", "/api/v1/projects/"+projectID+"/capabilities", req)
	if err != nil {
		return nil, err
	}
	var cap Capability
	if err := json.Unmarshal(data, &cap); err != nil {
		return nil, err
	}
	return &cap, nil
}

func (c *Client) AddVersion(ctx context.Context, capabilityID string, req AddVersionRequest) (*Version, error) {
	data, err := c.do(ctx, "POST", "/api/v1/capabilities/"+capabilityID+"/versions", req)
	if err != nil {
		return nil, err
	}
	var v Version
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// --- Release + Approval ---

// CreateRelease creates a Pending Release for a Version.
func (c *Client) CreateRelease(ctx context.Context, versionID string, req CreateReleaseRequest) (*Release, error) {
	data, err := c.do(ctx, "POST", "/api/v1/versions/"+versionID+"/releases", req)
	if err != nil {
		return nil, err
	}
	var r Release
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// GetRelease returns a Release by id.
func (c *Client) GetRelease(ctx context.Context, id string) (*Release, error) {
	data, err := c.do(ctx, "GET", "/api/v1/releases/"+id, nil)
	if err != nil {
		return nil, err
	}
	var r Release
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// ListReleases returns all Releases for a Capability.
func (c *Client) ListReleases(ctx context.Context, capabilityID string) ([]*Release, error) {
	data, err := c.do(ctx, "GET", "/api/v1/capabilities/"+capabilityID+"/releases", nil)
	if err != nil {
		return nil, err
	}
	var rs []*Release
	if err := json.Unmarshal(data, &rs); err != nil {
		return nil, err
	}
	return rs, nil
}

// Vote records a vote on a Release.
func (c *Client) Vote(ctx context.Context, releaseID string, req VoteRequest) (*Approval, error) {
	data, err := c.do(ctx, "POST", "/api/v1/releases/"+releaseID+"/votes", req)
	if err != nil {
		return nil, err
	}
	var a Approval
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// Activate transitions a Pending Release to Active.
func (c *Client) Activate(ctx context.Context, releaseID string) (*Release, error) {
	data, err := c.do(ctx, "POST", "/api/v1/releases/"+releaseID+"/activate", nil)
	if err != nil {
		return nil, err
	}
	var r Release
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// Rollback moves a Release to RolledBack.
func (c *Client) Rollback(ctx context.Context, releaseID string) (*Release, error) {
	data, err := c.do(ctx, "POST", "/api/v1/releases/"+releaseID+"/rollback", nil)
	if err != nil {
		return nil, err
	}
	var r Release
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// Invoke runs the Release through the configured LLM providers.
func (c *Client) Invoke(ctx context.Context, releaseID string, req InvokeRequest) (*Execution, error) {
	data, err := c.do(ctx, "POST", "/api/v1/releases/"+releaseID+"/invoke", req)
	if err != nil {
		return nil, err
	}
	var e Execution
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// Approval returns the vote trail for a Release.
func (c *Client) Approval(ctx context.Context, releaseID string) (*Approval, error) {
	data, err := c.do(ctx, "GET", "/api/v1/releases/"+releaseID+"/approval", nil)
	if err != nil {
		return nil, err
	}
	var a Approval
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// ApproveAndInvoke is the convenience flow used by the README: vote
// as a non-creator identity, activate, then invoke. The voter
// identity must differ from the Release's CreatedBy; otherwise the
// maker-checker policy rejects the activation.
func (c *Client) ApproveAndInvoke(ctx context.Context, releaseID, voterIdentity string, invokeReq InvokeRequest) (*Execution, error) {
	if _, err := c.Vote(ctx, releaseID, VoteRequest{Identity: voterIdentity, Decision: "approve"}); err != nil {
		return nil, errf.Errorf("vote: %w", err)
	}
	if _, err := c.Activate(ctx, releaseID); err != nil {
		return nil, errf.Errorf("activate: %w", err)
	}
	return c.Invoke(ctx, releaseID, invokeReq)
}

// --- Harness engineering: Datasets, Preconditions, Evals ---

// Dataset mirrors the server's harness.Dataset.
type Dataset struct {
	ID           string `json:"id"`
	CapabilityID string `json:"capability_id"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// DatasetCase mirrors harness.DatasetCase.
type DatasetCase struct {
	ID          string          `json:"id"`
	DatasetID   string          `json:"dataset_id"`
	Seq         int             `json:"seq"`
	Inputs      json.RawMessage `json:"inputs"`
	Expected    json.RawMessage `json:"expected"`
	Description string          `json:"description,omitempty"`
}

// DatasetWithCases is the GET /datasets/{id} response shape: dataset
// + its cases.
type DatasetWithCases struct {
	Dataset
	Cases []DatasetCase `json:"cases"`
}

// Precondition mirrors harness.Precondition.
type Precondition struct {
	ID           string `json:"id"`
	CapabilityID string `json:"capability_id"`
	Name         string `json:"name"`
	Command      string `json:"command"`
	TimeoutSec   int    `json:"timeout_sec"`
	Enabled      bool   `json:"enabled"`
	CreatedAt    string `json:"created_at"`
}

// PreconditionFailure describes one failing precondition in a 409
// response body when Activate is blocked.
type PreconditionFailure struct {
	Name   string `json:"name"`
	Output string `json:"output"`
}

// CreateDatasetRequest is the request body for creating a Dataset.
// Cases may be omitted; callers can add them via PutCases.
type CreateDatasetRequest struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Cases       []DatasetCase `json:"cases,omitempty"`
}

// CreateDataset creates a Dataset (optionally seeded with cases).
func (c *Client) CreateDataset(ctx context.Context, capabilityID string, req CreateDatasetRequest) (*Dataset, error) {
	data, err := c.do(ctx, "POST", "/api/v1/capabilities/"+capabilityID+"/datasets", req)
	if err != nil {
		return nil, err
	}
	var d Dataset
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// ListDatasets returns all Datasets attached to a Capability.
func (c *Client) ListDatasets(ctx context.Context, capabilityID string) ([]*Dataset, error) {
	data, err := c.do(ctx, "GET", "/api/v1/capabilities/"+capabilityID+"/datasets", nil)
	if err != nil {
		return nil, err
	}
	var ds []*Dataset
	if err := json.Unmarshal(data, &ds); err != nil {
		return nil, err
	}
	return ds, nil
}

// GetDataset returns a Dataset with its cases.
func (c *Client) GetDataset(ctx context.Context, id string) (*DatasetWithCases, error) {
	data, err := c.do(ctx, "GET", "/api/v1/datasets/"+id, nil)
	if err != nil {
		return nil, err
	}
	var d DatasetWithCases
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// PutCases atomically replaces a Dataset's cases.
func (c *Client) PutCases(ctx context.Context, id string, cases []DatasetCase) error {
	_, err := c.do(ctx, "PUT", "/api/v1/datasets/"+id+"/cases", map[string]any{"cases": cases})
	return err
}

// DeleteDataset removes a Dataset (cascades to cases).
func (c *Client) DeleteDataset(ctx context.Context, id string) error {
	return c.delete(ctx, "/api/v1/datasets/"+id)
}

// CreatePreconditionRequest is the request body for adding a
// Precondition to a Capability.
type CreatePreconditionRequest struct {
	Name       string `json:"name"`
	Command    string `json:"command"`
	TimeoutSec int    `json:"timeout_sec,omitempty"`
	Enabled    *bool  `json:"enabled,omitempty"`
}

// CreatePrecondition attaches a precondition hook.
func (c *Client) CreatePrecondition(ctx context.Context, capabilityID string, req CreatePreconditionRequest) (*Precondition, error) {
	data, err := c.do(ctx, "POST", "/api/v1/capabilities/"+capabilityID+"/preconditions", req)
	if err != nil {
		return nil, err
	}
	var p Precondition
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, errf.Errorf("decode precondition: %w", err)
	}
	return &p, nil
}

// UpdatePreconditionRequest is the partial-update body for
// PUT /api/v1/preconditions/{id}. Only the fields present in
// the body are touched; missing fields keep their previous
// value. capability_id is immutable on update.
type UpdatePreconditionRequest struct {
	Name       *string `json:"name,omitempty"`
	Command    *string `json:"command,omitempty"`
	TimeoutSec *int    `json:"timeout_sec,omitempty"`
	Enabled    *bool   `json:"enabled,omitempty"`
}

// UpdatePrecondition applies a partial mutation to an
// existing precondition. Returns 404 when the id is unknown.
func (c *Client) UpdatePrecondition(ctx context.Context, id string, req UpdatePreconditionRequest) (*Precondition, error) {
	data, err := c.do(ctx, "PUT", "/api/v1/preconditions/"+id, req)
	if err != nil {
		return nil, err
	}
	var p Precondition
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, errf.Errorf("decode precondition: %w", err)
	}
	return &p, nil
}

// ListPreconditions returns all preconditions for a Capability.
func (c *Client) ListPreconditions(ctx context.Context, capabilityID string) ([]*Precondition, error) {
	data, err := c.do(ctx, "GET", "/api/v1/capabilities/"+capabilityID+"/preconditions", nil)
	if err != nil {
		return nil, err
	}
	var ps []*Precondition
	if err := json.Unmarshal(data, &ps); err != nil {
		return nil, err
	}
	return ps, nil
}

// DeletePrecondition removes a precondition.
func (c *Client) DeletePrecondition(ctx context.Context, id string) error {
	return c.delete(ctx, "/api/v1/preconditions/"+id)
}

// EvalRun mirrors harness.EvalRun.
type EvalRun struct {
	ID         string  `json:"id"`
	ReleaseID  string  `json:"release_id"`
	DatasetID  string  `json:"dataset_id"`
	Scorer     string  `json:"scorer"`
	Score      float64 `json:"score"`
	Passed     int     `json:"passed"`
	Failed     int     `json:"failed"`
	Total      int     `json:"total"`
	Status     string  `json:"status"`
	StartedAt  string  `json:"started_at"`
	FinishedAt string  `json:"finished_at,omitempty"`
}

// EvalResult mirrors harness.EvalResult.
type EvalResult struct {
	ID        string          `json:"id"`
	RunID     string          `json:"run_id"`
	CaseID    string          `json:"case_id"`
	Seq       int             `json:"seq"`
	Passed    bool            `json:"passed"`
	Actual    json.RawMessage `json:"actual"`
	Error     string          `json:"error,omitempty"`
	LatencyMs int64           `json:"latency_ms"`
}

// EvalRunWithResults is the GET /evals/{id} response shape: run + results.
type EvalRunWithResults struct {
	Run     EvalRun      `json:"run"`
	Results []EvalResult `json:"results"`
}

// RunEvalRequest is the request body for POST /releases/{id}/evals.
type RunEvalRequest struct {
	DatasetID string `json:"dataset_id"`
	Scorer    string `json:"scorer,omitempty"`
}

// RunEval invokes an eval against a Release. Returns the persisted
// EvalRun. A failed run returns the EvalRun + a 422-mapped error;
// callers can inspect run.Failed to drive their own UI.
func (c *Client) RunEval(ctx context.Context, releaseID string, req RunEvalRequest) (*EvalRun, error) {
	data, err := c.do(ctx, "POST", "/api/v1/releases/"+releaseID+"/evals", req)
	if err != nil {
		return nil, err
	}
	var r EvalRun
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// ListEvals returns all eval runs for a Release.
func (c *Client) ListEvals(ctx context.Context, releaseID string) ([]*EvalRun, error) {
	data, err := c.do(ctx, "GET", "/api/v1/releases/"+releaseID+"/evals", nil)
	if err != nil {
		return nil, err
	}
	var rs []*EvalRun
	if err := json.Unmarshal(data, &rs); err != nil {
		return nil, err
	}
	return rs, nil
}

// GetEval returns an eval run with its per-case results.
func (c *Client) GetEval(ctx context.Context, id string) (*EvalRunWithResults, error) {
	data, err := c.do(ctx, "GET", "/api/v1/evals/"+id, nil)
	if err != nil {
		return nil, err
	}
	var r EvalRunWithResults
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// delete is a small helper that swallows the body's []byte result
// and returns just the error. Callers must supply a context so
// cancellation and tracing propagate end-to-end.
func (c *Client) delete(ctx context.Context, path string) error {
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}

// --- API Keys (API-SDK-1) ---
//
// The endpoints exist in the OpenAPI spec but were missing from
// the Go SDK before this commit. Add them so SDK consumers can
// manage keys without dropping down to raw HTTP.

// APIKey mirrors models.APIKey. The server returns the key
// value (one-time) on create; SDK callers MUST persist it
// before the response goes out of scope.
type APIKey struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Name      string     `json:"name"`
	Key       string     `json:"key,omitempty"`
	KeyPrefix string     `json:"key_prefix,omitempty"`
	Role      string     `json:"role"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// CreateAPIKeyRequest is the request body for POST /api/v1/apikeys.
type CreateAPIKeyRequest struct {
	Name      string     `json:"name"`
	UserID    string     `json:"user_id,omitempty"`
	Role      string     `json:"role,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// CreateAPIKey mints a new API key. The returned APIKey.Key is
// the only time the plaintext key is ever returned; SDK callers
// should store it immediately and surface it to the user.
func (c *Client) CreateAPIKey(ctx context.Context, req CreateAPIKeyRequest) (*APIKey, error) {
	data, err := c.do(ctx, "POST", "/api/v1/apikeys", req)
	if err != nil {
		return nil, err
	}
	var k APIKey
	if err := json.Unmarshal(data, &k); err != nil {
		return nil, errf.Errorf("decode api key: %w", err)
	}
	return &k, nil
}

// ListAPIKeys returns the API keys for a user. userID may be
// empty to list every key (admin-only).
func (c *Client) ListAPIKeys(ctx context.Context, userID string) ([]*APIKey, error) {
	path := "/api/v1/apikeys"
	if userID != "" {
		path += "?user_id=" + url.QueryEscape(userID)
	}
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var ks []*APIKey
	if err := json.Unmarshal(data, &ks); err != nil {
		return nil, errf.Errorf("decode api keys: %w", err)
	}
	return ks, nil
}

// RevokeAPIKey removes a key by id. Idempotent on already-revoked
// keys (the server returns 400; SDK callers can ignore that case).
func (c *Client) RevokeAPIKey(ctx context.Context, id string) error {
	return c.delete(ctx, "/api/v1/apikeys/"+id)
}

// --- OAuth (API-SDK-1) ---
//
// The server handles the OAuth flow in two steps: GET
// /api/v1/auth/{provider}/login (sets the state cookie and
// returns a redirect), and GET /api/v1/auth/{provider}/callback
// (consumes the state, exchanges the code, mints a key). The
// SDK exposes the login URL so a CLI / UI can open the user's
// browser; the callback exchange is normally handled by the
// browser hitting the server directly.

// OAuthLoginURL returns the URL a browser should be redirected
// to in order to begin the OAuth flow for `provider`. The
// caller must follow the redirect (or open it in a browser);
// the SDK cannot complete the flow on the caller's behalf
// because the state cookie has to be set on the daemon's
// domain.
func (c *Client) OAuthLoginURL(provider string) string {
	return c.baseURL + "/api/v1/auth/" + provider + "/login"
}

// is returned as json.RawMessage because typing each response
// shape would balloon this PR by ~30 more files; a follow-up
// PR introduces typed responses for domain endpoints (workspaces,
// capabilities, versions, etc.) and typed errors for the rest.
// The existing typed methods (Health, CreateWorkspace, ...) are
// preserved unchanged.
//
// Adding a new route to the daemon without a corresponding entry
// in scripts/genopenapi/main.go's operationIDMap fails the
// contract test (tests/contract/contract_test.go). Add the route,
// add the operationId, regenerate the spec via make openapi,
// and add a method here.

// Version calls GET /api/v1/version.
func (c *Client) Version(ctx context.Context) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/version", nil)
}

// ListWorkspaces calls GET /api/v1/workspaces.
func (c *Client) ListWorkspaces(ctx context.Context) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/workspaces", nil)
}

// GetWorkspace calls GET /api/v1/workspaces/{id}.
func (c *Client) GetWorkspace(ctx context.Context, id string) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/workspaces/" + id, nil)
}

// UpdateWorkspace calls PUT /api/v1/workspaces/{id}.
func (c *Client) UpdateWorkspace(ctx context.Context, id string, req any) (json.RawMessage, error) {
		return c.do(ctx, "PUT", "/api/v1/workspaces/" + id, req)
}

// DeleteWorkspace calls DELETE /api/v1/workspaces/{id}.
func (c *Client) DeleteWorkspace(ctx context.Context, id string) error {
		if _, err := c.do(ctx, "DELETE", "/api/v1/workspaces/" + id, nil); err != nil {
		return err
	}
	return nil
}

// GetWorkspaceObservation calls GET /api/v1/workspaces/{id}/observation.
func (c *Client) GetWorkspaceObservation(ctx context.Context, id string) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/workspaces/" + id + "/observation", nil)
}

// ListProjects calls GET /api/v1/workspaces/{workspace_id}/projects.
func (c *Client) ListProjects(ctx context.Context, workspaceId string) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/workspaces/" + workspaceId + "/projects", nil)
}

// CreateProject calls POST /api/v1/workspaces/{workspace_id}/projects.
func (c *Client) CreateProject(ctx context.Context, workspaceId string, req any) (json.RawMessage, error) {
		return c.do(ctx, "POST", "/api/v1/workspaces/" + workspaceId + "/projects", req)
}

// GetProject calls GET /api/v1/projects/{id}.
func (c *Client) GetProject(ctx context.Context, id string) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/projects/" + id, nil)
}

// UpdateProject calls PUT /api/v1/projects/{id}.
func (c *Client) UpdateProject(ctx context.Context, id string, req any) (json.RawMessage, error) {
		return c.do(ctx, "PUT", "/api/v1/projects/" + id, req)
}

// DeleteProject calls DELETE /api/v1/projects/{id}.
func (c *Client) DeleteProject(ctx context.Context, id string) error {
		if _, err := c.do(ctx, "DELETE", "/api/v1/projects/" + id, nil); err != nil {
		return err
	}
	return nil
}

// ListCapabilities calls GET /api/v1/projects/{project_id}/capabilities.
func (c *Client) ListCapabilities(ctx context.Context, projectId string) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/projects/" + projectId + "/capabilities", nil)
}

// GetCapability calls GET /api/v1/capabilities/{id}.
func (c *Client) GetCapability(ctx context.Context, id string) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/capabilities/" + id, nil)
}

// UpdateCapability calls PUT /api/v1/capabilities/{id}.
func (c *Client) UpdateCapability(ctx context.Context, id string, req any) (json.RawMessage, error) {
		return c.do(ctx, "PUT", "/api/v1/capabilities/" + id, req)
}

// DeleteCapability calls DELETE /api/v1/capabilities/{id}.
func (c *Client) DeleteCapability(ctx context.Context, id string) error {
		if _, err := c.do(ctx, "DELETE", "/api/v1/capabilities/" + id, nil); err != nil {
		return err
	}
	return nil
}

// GetCapabilityContract calls GET /api/v1/capabilities/{id}/contract.
func (c *Client) GetCapabilityContract(ctx context.Context, id string) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/capabilities/" + id + "/contract", nil)
}

// UpdateCapabilityContract calls PUT /api/v1/capabilities/{id}/contract.
func (c *Client) UpdateCapabilityContract(ctx context.Context, id string, req any) (json.RawMessage, error) {
		return c.do(ctx, "PUT", "/api/v1/capabilities/" + id + "/contract", req)
}

// DiffCapabilities calls GET /api/v1/capabilities/{id}/diff.
func (c *Client) DiffCapabilities(ctx context.Context, id string) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/capabilities/" + id + "/diff", nil)
}

// GetCapabilityReputation calls GET /api/v1/capabilities/{id}/reputation.
func (c *Client) GetCapabilityReputation(ctx context.Context, id string) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/capabilities/" + id + "/reputation", nil)
}

// UpdateSelfEvolveConfig calls PUT /api/v1/capabilities/{id}/self-evolve.
func (c *Client) UpdateSelfEvolveConfig(ctx context.Context, id string, req any) (json.RawMessage, error) {
		return c.do(ctx, "PUT", "/api/v1/capabilities/" + id + "/self-evolve", req)
}

// ListVersions calls GET /api/v1/capabilities/{capability_id}/versions.
func (c *Client) ListVersions(ctx context.Context, capabilityId string) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/capabilities/" + capabilityId + "/versions", nil)
}

// GetLatestVersion calls GET /api/v1/capabilities/{capability_id}/versions/latest.
func (c *Client) GetLatestVersion(ctx context.Context, capabilityId string) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/capabilities/" + capabilityId + "/versions/latest", nil)
}

// GetVersion calls GET /api/v1/versions/{id}.
func (c *Client) GetVersion(ctx context.Context, id string) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/versions/" + id, nil)
}

// ListExecutions calls GET /api/v1/versions/{version_id}/executions.
func (c *Client) ListExecutions(ctx context.Context, versionId string) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/versions/" + versionId + "/executions", nil)
}

// CreateExecution calls POST /api/v1/versions/{version_id}/executions.
func (c *Client) CreateExecution(ctx context.Context, versionId string, req any) (json.RawMessage, error) {
		return c.do(ctx, "POST", "/api/v1/versions/" + versionId + "/executions", req)
}

// GetExecution calls GET /api/v1/executions/{id}.
func (c *Client) GetExecution(ctx context.Context, id string) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/executions/" + id, nil)
}

// ListUsers calls GET /api/v1/users.
func (c *Client) ListUsers(ctx context.Context) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/users", nil)
}

// CreateUser calls POST /api/v1/users.
func (c *Client) CreateUser(ctx context.Context, req any) (json.RawMessage, error) {
		return c.do(ctx, "POST", "/api/v1/users", req)
}

// GetUser calls GET /api/v1/users/{id}.
func (c *Client) GetUser(ctx context.Context, id string) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/users/" + id, nil)
}

// UpdateUser calls PUT /api/v1/users/{id}.
func (c *Client) UpdateUser(ctx context.Context, id string, req any) (json.RawMessage, error) {
		return c.do(ctx, "PUT", "/api/v1/users/" + id, req)
}

// DeleteUser calls DELETE /api/v1/users/{id}.
func (c *Client) DeleteUser(ctx context.Context, id string) error {
		if _, err := c.do(ctx, "DELETE", "/api/v1/users/" + id, nil); err != nil {
		return err
	}
	return nil
}

// ListAudit calls GET /api/v1/audit.
func (c *Client) ListAudit(ctx context.Context) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/audit", nil)
}

// ExportAudit calls GET /api/v1/audit/export.
func (c *Client) ExportAudit(ctx context.Context) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/audit/export", nil)
}

// VerifyAuditChain calls GET /api/v1/audit/verify.
func (c *Client) VerifyAuditChain(ctx context.Context) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/audit/verify", nil)
}

// LogsStream calls GET /api/v1/logs/stream.
func (c *Client) LogsStream(ctx context.Context) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/logs/stream", nil)
}

// MetricsSummary calls GET /api/v1/metrics/summary.
func (c *Client) MetricsSummary(ctx context.Context) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/metrics/summary", nil)
}

// MetricsDashboard calls GET /api/v1/metrics/dashboard.
func (c *Client) MetricsDashboard(ctx context.Context) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/metrics/dashboard", nil)
}

// Metrics calls GET /api/v1/metrics.
func (c *Client) Metrics(ctx context.Context) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/metrics", nil)
}

// GetProvider calls GET /api/v1/providers/{name}.
func (c *Client) GetProvider(ctx context.Context, name string) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/providers/" + name, nil)
}

// TestProvider calls POST /api/v1/providers/{name}/test.
func (c *Client) TestProvider(ctx context.Context, name string, req any) (json.RawMessage, error) {
		return c.do(ctx, "POST", "/api/v1/providers/" + name + "/test", req)
}

// ListVaultKeys calls GET /api/v1/vault/keys.
func (c *Client) ListVaultKeys(ctx context.Context) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/vault/keys", nil)
}

// SaveVaultKey calls POST /api/v1/vault/keys.
func (c *Client) SaveVaultKey(ctx context.Context, req any) (json.RawMessage, error) {
		return c.do(ctx, "POST", "/api/v1/vault/keys", req)
}

// DeleteVaultKey calls DELETE /api/v1/vault/keys/{id}.
func (c *Client) DeleteVaultKey(ctx context.Context, id string) error {
		if _, err := c.do(ctx, "DELETE", "/api/v1/vault/keys/" + id, nil); err != nil {
		return err
	}
	return nil
}

// ListAlertRules calls GET /api/v1/alerts/rules.
func (c *Client) ListAlertRules(ctx context.Context) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/alerts/rules", nil)
}

// CreateAlertRule calls POST /api/v1/alerts/rules.
func (c *Client) CreateAlertRule(ctx context.Context, req any) (json.RawMessage, error) {
		return c.do(ctx, "POST", "/api/v1/alerts/rules", req)
}

// GetAlertRule calls GET /api/v1/alerts/rules/{id}.
func (c *Client) GetAlertRule(ctx context.Context, id string) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/alerts/rules/" + id, nil)
}

// UpdateAlertRule calls PUT /api/v1/alerts/rules/{id}.
func (c *Client) UpdateAlertRule(ctx context.Context, id string, req any) (json.RawMessage, error) {
		return c.do(ctx, "PUT", "/api/v1/alerts/rules/" + id, req)
}

// DeleteAlertRule calls DELETE /api/v1/alerts/rules/{id}.
func (c *Client) DeleteAlertRule(ctx context.Context, id string) error {
		if _, err := c.do(ctx, "DELETE", "/api/v1/alerts/rules/" + id, nil); err != nil {
		return err
	}
	return nil
}

// LinkAlertRuleGroup calls POST /api/v1/alerts/rules/{rule_id}/groups/{group_id}.
func (c *Client) LinkAlertRuleGroup(ctx context.Context, ruleId string, groupId string, req any) (json.RawMessage, error) {
		return c.do(ctx, "POST", "/api/v1/alerts/rules/" + ruleId + "/groups/" + groupId, req)
}

// UnlinkAlertRuleGroup calls DELETE /api/v1/alerts/rules/{rule_id}/groups/{group_id}.
func (c *Client) UnlinkAlertRuleGroup(ctx context.Context, ruleId string, groupId string) error {
		if _, err := c.do(ctx, "DELETE", "/api/v1/alerts/rules/" + ruleId + "/groups/" + groupId, nil); err != nil {
		return err
	}
	return nil
}

// ListActiveAlerts calls GET /api/v1/alerts/active.
func (c *Client) ListActiveAlerts(ctx context.Context) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/alerts/active", nil)
}

// ResolveAlert calls PUT /api/v1/alerts/active/{id}/resolve.
func (c *Client) ResolveAlert(ctx context.Context, id string, req any) (json.RawMessage, error) {
		return c.do(ctx, "PUT", "/api/v1/alerts/active/" + id + "/resolve", req)
}

// ListNotificationGroups calls GET /api/v1/alerts/notifications.
func (c *Client) ListNotificationGroups(ctx context.Context) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/alerts/notifications", nil)
}

// AddNotificationGroup calls POST /api/v1/alerts/notifications.
func (c *Client) AddNotificationGroup(ctx context.Context, req any) (json.RawMessage, error) {
		return c.do(ctx, "POST", "/api/v1/alerts/notifications", req)
}

// ListWebhooks calls GET /api/v1/webhooks.
func (c *Client) ListWebhooks(ctx context.Context) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/webhooks", nil)
}

// CreateWebhook calls POST /api/v1/webhooks.
func (c *Client) CreateWebhook(ctx context.Context, req any) (json.RawMessage, error) {
		return c.do(ctx, "POST", "/api/v1/webhooks", req)
}

// DeleteWebhook calls DELETE /api/v1/webhooks/{id}.
func (c *Client) DeleteWebhook(ctx context.Context, id string) error {
		if _, err := c.do(ctx, "DELETE", "/api/v1/webhooks/" + id, nil); err != nil {
		return err
	}
	return nil
}

// RunWorkflow calls POST /api/v1/workflows/run.
func (c *Client) RunWorkflow(ctx context.Context, req any) (json.RawMessage, error) {
		return c.do(ctx, "POST", "/api/v1/workflows/run", req)
}

// ReasoningCompile calls POST /api/v1/reasoning/compile.
func (c *Client) ReasoningCompile(ctx context.Context, req any) (json.RawMessage, error) {
		return c.do(ctx, "POST", "/api/v1/reasoning/compile", req)
}

// CatalogSearch calls GET /api/v1/catalog/capabilities.
func (c *Client) CatalogSearch(ctx context.Context) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/catalog/capabilities", nil)
}

// OAuthCallback calls GET /api/v1/auth/{provider}/callback.
func (c *Client) OAuthCallback(ctx context.Context, provider string) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/auth/" + provider + "/callback", nil)
}

// ListSettings calls GET /api/v1/settings.
func (c *Client) ListSettings(ctx context.Context) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/settings", nil)
}

// GetSetting calls GET /api/v1/settings/{key}.
func (c *Client) GetSetting(ctx context.Context, key string) (json.RawMessage, error) {
		return c.do(ctx, "GET", "/api/v1/settings/" + key, nil)
}

// SetSetting calls PUT /api/v1/settings/{key}.
func (c *Client) SetSetting(ctx context.Context, key string, req any) (json.RawMessage, error) {
		return c.do(ctx, "PUT", "/api/v1/settings/" + key, req)
}

// DeleteSetting calls DELETE /api/v1/settings/{key}.
func (c *Client) DeleteSetting(ctx context.Context, key string) error {
		if _, err := c.do(ctx, "DELETE", "/api/v1/settings/" + key, nil); err != nil {
		return err
	}
	return nil
}

// Setup calls POST /api/v1/setup.
func (c *Client) Setup(ctx context.Context, req any) (json.RawMessage, error) {
		return c.do(ctx, "POST", "/api/v1/setup", req)
}


