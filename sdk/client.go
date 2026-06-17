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

type Variable struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description"`
}

type ProviderBinding struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	APIKeyRef string `json:"api_key_ref,omitempty"`
}

type CreatePromptRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Content     string            `json:"content"`
	Variables   []Variable        `json:"variables,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	ModelHint   string            `json:"model_hint,omitempty"`
	Binding     *ProviderBinding  `json:"binding,omitempty"`
}

type UpdatePromptRequest struct {
	Name        *string           `json:"name,omitempty"`
	Description *string           `json:"description,omitempty"`
	Content     *string           `json:"content,omitempty"`
	Status      *string           `json:"status,omitempty"`
}

type RunPromptRequest struct {
	Variables map[string]string `json:"variables,omitempty"`
	Provider  string            `json:"provider,omitempty"`
	Model     string            `json:"model,omitempty"`
}

type RunPromptResponse struct {
	Content   string      `json:"content"`
	Model     string      `json:"model"`
	Usage     Usage       `json:"usage"`
	LatencyMs int64       `json:"latency_ms"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (c *Client) ListPrompts(ctx context.Context) ([]*Prompt, error) {
	data, err := c.do(ctx, "GET", "/api/v1/prompts", nil)
	if err != nil {
		return nil, err
	}
	var prompts []*Prompt
	json.Unmarshal(data, &prompts)
	return prompts, nil
}

func (c *Client) GetPrompt(ctx context.Context, id string) (*Prompt, error) {
	data, err := c.do(ctx, "GET", "/api/v1/prompts/"+id, nil)
	if err != nil {
		return nil, err
	}
	var p Prompt
	json.Unmarshal(data, &p)
	return &p, nil
}

func (c *Client) CreatePrompt(ctx context.Context, req *CreatePromptRequest) (*Prompt, error) {
	data, err := c.do(ctx, "POST", "/api/v1/prompts", req)
	if err != nil {
		return nil, err
	}
	var p Prompt
	json.Unmarshal(data, &p)
	return &p, nil
}

func (c *Client) UpdatePrompt(ctx context.Context, id string, req *UpdatePromptRequest) (*Prompt, error) {
	data, err := c.do(ctx, "PUT", "/api/v1/prompts/"+id, req)
	if err != nil {
		return nil, err
	}
	var p Prompt
	json.Unmarshal(data, &p)
	return &p, nil
}

func (c *Client) DeletePrompt(ctx context.Context, id string) error {
	_, err := c.do(ctx, "DELETE", "/api/v1/prompts/"+id, nil)
	return err
}

func (c *Client) RunPrompt(ctx context.Context, id string, req *RunPromptRequest) (*RunPromptResponse, error) {
	data, err := c.do(ctx, "POST", "/api/v1/prompts/"+id+"/run", req)
	if err != nil {
		return nil, err
	}
	var resp RunPromptResponse
	json.Unmarshal(data, &resp)
	return &resp, nil
}

func (c *Client) DeployPrompt(ctx context.Context, id string) (*Prompt, error) {
	data, err := c.do(ctx, "POST", "/api/v1/prompts/"+id+"/deploy", nil)
	if err != nil {
		return nil, err
	}
	var p Prompt
	json.Unmarshal(data, &p)
	return &p, nil
}

func (c *Client) ArchivePrompt(ctx context.Context, id string) (*Prompt, error) {
	data, err := c.do(ctx, "POST", "/api/v1/prompts/"+id+"/archive", nil)
	if err != nil {
		return nil, err
	}
	var p Prompt
	json.Unmarshal(data, &p)
	return &p, nil
}

// --- Agents ---

type Agent struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Steps       []AgentStep `json:"steps"`
	Tools       []ToolRef   `json:"tools"`
	Status      string      `json:"status"`
}

type AgentStep struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Tool       string            `json:"tool"`
	Config     map[string]string `json:"config"`
	DependsOn  []string          `json:"depends_on"`
	Condition  *Condition        `json:"condition,omitempty"`
}

type Condition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type ToolRef struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Config map[string]string `json:"config"`
}

func (c *Client) ListAgents(ctx context.Context) ([]*Agent, error) {
	data, err := c.do(ctx, "GET", "/api/v1/agents", nil)
	if err != nil {
		return nil, err
	}
	var agents []*Agent
	json.Unmarshal(data, &agents)
	return agents, nil
}

func (c *Client) GetAgent(ctx context.Context, id string) (*Agent, error) {
	data, err := c.do(ctx, "GET", "/api/v1/agents/"+id, nil)
	if err != nil {
		return nil, err
	}
	var a Agent
	json.Unmarshal(data, &a)
	return &a, nil
}

// --- Health ---

type HealthResponse struct {
	Status string `json:"status"`
}

func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	data, err := c.do(ctx, "GET", "/health", nil)
	if err != nil {
		return nil, err
	}
	var h HealthResponse
	json.Unmarshal(data, &h)
	return &h, nil
}

// --- Providers ---

type ProviderInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type ProvidersResponse struct {
	Providers []string `json:"providers"`
}

func (c *Client) ListProviders(ctx context.Context) ([]string, error) {
	data, err := c.do(ctx, "GET", "/api/v1/providers", nil)
	if err != nil {
		return nil, err
	}
	var resp ProvidersResponse
	json.Unmarshal(data, &resp)
	return resp.Providers, nil
}
