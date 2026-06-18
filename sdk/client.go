// Package sdk provides a Go client for the Promptsheon REST API.
package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a Promptsheon API client.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New creates a new Promptsheon API client.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// APIError represents an error response from the API.
type APIError struct {
	Status  int    `json:"-"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("api error (status %d): %s", e.Status, e.Message)
}

func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr APIError
		json.Unmarshal(data, &apiErr)
		apiErr.Status = resp.StatusCode
		return nil, &apiErr
	}

	return data, nil
}

// --- Prompts ---

// Prompt represents a versioned prompt asset.
type Prompt struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Content     string            `json:"content"`
	Variables   []Variable        `json:"variables"`
	Tags        []string          `json:"tags"`
	ModelHint   string            `json:"model_hint"`
	Binding     *ProviderBinding  `json:"binding,omitempty"`
	Version     int               `json:"version"`
	Status      string            `json:"status"`
	CreatedBy   string            `json:"created_by"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Metadata    map[string]string `json:"metadata"`
}

// Variable defines a template variable for a prompt.
type Variable struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description"`
}

// ProviderBinding specifies which LLM provider and model to use.
type ProviderBinding struct {
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	APIKeyRef string `json:"api_key_ref,omitempty"`
}

// CreatePromptRequest is the request body for creating a prompt.
type CreatePromptRequest struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Content     string           `json:"content"`
	Variables   []Variable       `json:"variables,omitempty"`
	Tags        []string         `json:"tags,omitempty"`
	ModelHint   string           `json:"model_hint,omitempty"`
	Binding     *ProviderBinding `json:"binding,omitempty"`
}

// UpdatePromptRequest is the request body for updating a prompt.
type UpdatePromptRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Content     *string `json:"content,omitempty"`
	Status      *string `json:"status,omitempty"`
}

// RunPromptRequest is the request body for running a prompt.
type RunPromptRequest struct {
	Variables map[string]string `json:"variables,omitempty"`
	Provider  string            `json:"provider,omitempty"`
	Model     string            `json:"model,omitempty"`
}

// RunPromptResponse holds the output from a prompt execution.
type RunPromptResponse struct {
	Content   string `json:"content"`
	Model     string `json:"model"`
	Usage     Usage  `json:"usage"`
	LatencyMs int64  `json:"latency_ms"`
}

// Usage tracks token consumption for a single LLM call.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ListPrompts returns all prompts from the server.
func (c *Client) ListPrompts(ctx context.Context) ([]*Prompt, error) {
	data, err := c.do(ctx, "GET", "/api/v1/prompts", nil)
	if err != nil {
		return nil, err
	}
	var prompts []*Prompt
	if err := json.Unmarshal(data, &prompts); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return prompts, nil
}

// GetPrompt returns a prompt by its ID.
func (c *Client) GetPrompt(ctx context.Context, id string) (*Prompt, error) {
	data, err := c.do(ctx, "GET", "/api/v1/prompts/"+id, nil)
	if err != nil {
		return nil, err
	}
	var p Prompt
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &p, nil
}

// CreatePrompt creates a new prompt on the server.
func (c *Client) CreatePrompt(ctx context.Context, req *CreatePromptRequest) (*Prompt, error) {
	data, err := c.do(ctx, "POST", "/api/v1/prompts", req)
	if err != nil {
		return nil, err
	}
	var p Prompt
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &p, nil
}

// UpdatePrompt updates an existing prompt by ID.
func (c *Client) UpdatePrompt(ctx context.Context, id string, req *UpdatePromptRequest) (*Prompt, error) {
	data, err := c.do(ctx, "PUT", "/api/v1/prompts/"+id, req)
	if err != nil {
		return nil, err
	}
	var p Prompt
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &p, nil
}

// DeletePrompt deletes a prompt by ID.
func (c *Client) DeletePrompt(ctx context.Context, id string) error {
	_, err := c.do(ctx, "DELETE", "/api/v1/prompts/"+id, nil)
	return err
}

// RunPrompt executes a prompt via the configured LLM provider and returns the response.
func (c *Client) RunPrompt(ctx context.Context, id string, req *RunPromptRequest) (*RunPromptResponse, error) {
	data, err := c.do(ctx, "POST", "/api/v1/prompts/"+id+"/run", req)
	if err != nil {
		return nil, err
	}
	var resp RunPromptResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &resp, nil
}

// DeployPrompt deploys an approved prompt to production status.
func (c *Client) DeployPrompt(ctx context.Context, id string) (*Prompt, error) {
	data, err := c.do(ctx, "POST", "/api/v1/prompts/"+id+"/deploy", nil)
	if err != nil {
		return nil, err
	}
	var p Prompt
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &p, nil
}

// ArchivePrompt archives a prompt, moving it to read-only status.
func (c *Client) ArchivePrompt(ctx context.Context, id string) (*Prompt, error) {
	data, err := c.do(ctx, "POST", "/api/v1/prompts/"+id+"/archive", nil)
	if err != nil {
		return nil, err
	}
	var p Prompt
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &p, nil
}

// --- Agents ---

// Agent represents an agent workflow with ordered steps.
type Agent struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Steps       []AgentStep `json:"steps"`
	Tools       []ToolRef   `json:"tools"`
	Status      string      `json:"status"`
}

// AgentStep defines a single step within an agent workflow.
type AgentStep struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Tool      string            `json:"tool"`
	Config    map[string]string `json:"config"`
	DependsOn []string          `json:"depends_on"`
	Condition *Condition        `json:"condition,omitempty"`
}

// Condition defines a branching condition for a workflow step.
type Condition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// ToolRef references a tool configuration for an agent step.
type ToolRef struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Config map[string]string `json:"config"`
}

// ListAgents returns all agents from the server.
func (c *Client) ListAgents(ctx context.Context) ([]*Agent, error) {
	data, err := c.do(ctx, "GET", "/api/v1/agents", nil)
	if err != nil {
		return nil, err
	}
	var agents []*Agent
	if err := json.Unmarshal(data, &agents); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return agents, nil
}

// GetAgent returns an agent by its ID.
func (c *Client) GetAgent(ctx context.Context, id string) (*Agent, error) {
	data, err := c.do(ctx, "GET", "/api/v1/agents/"+id, nil)
	if err != nil {
		return nil, err
	}
	var a Agent
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &a, nil
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
		return nil, fmt.Errorf("decode response: %w", err)
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
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return resp.Providers, nil
}
