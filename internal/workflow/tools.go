package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// Tool is the interface for tools that can be executed within a workflow step.
type Tool interface {
	Execute(ctx context.Context, input map[string]any) (map[string]any, error)
	Name() string
}

// Registry manages available tools.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Tools returns all registered tool names.
func (r *Registry) Tools() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// DefaultRegistry creates a registry with the standard tools registered.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(&HTTPTool{})
	r.Register(&ShellTool{})
	r.Register(&JSONTransformTool{})
	r.Register(&PromptCallTool{})
	return r
}

// --- HTTP Tool ---

// HTTPTool makes HTTP requests.
type HTTPTool struct{}

func (h *HTTPTool) Name() string { return "http" }

func (h *HTTPTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	method := strings.ToUpper(toString(input["method"]))
	if method == "" {
		method = "GET"
	}
	url := toString(input["url"])
	if url == "" {
		return nil, fmt.Errorf("http tool: url is required")
	}

	var body io.Reader
	if bodyRaw, ok := input["body"]; ok {
		data, err := json.Marshal(bodyRaw)
		if err != nil {
			return nil, fmt.Errorf("http tool: marshal body: %w", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("http tool: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Add custom headers
	if headers, ok := input["headers"].(map[string]any); ok {
		for k, v := range headers {
			req.Header.Set(k, toString(v))
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http tool: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("http tool: read body: %w", err)
	}

	result := map[string]any{
		"status": resp.StatusCode,
		"body":   string(respBody),
	}

	// Try to parse JSON body
	var jsonBody any
	if json.Unmarshal(respBody, &jsonBody) == nil {
		result["json"] = jsonBody
	}

	return result, nil
}

// --- Shell Tool ---

// ShellTool executes shell commands.
type ShellTool struct{}

func (s *ShellTool) Name() string { return "shell" }

func (s *ShellTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	command := toString(input["command"])
	if command == "" {
		return nil, fmt.Errorf("shell tool: command is required")
	}

	timeout := 30 * time.Second
	if t, ok := input["timeout"]; ok {
		timeout = time.Duration(toFloat64(t) * float64(time.Second))
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	output, err := cmd.CombinedOutput()

	result := map[string]any{
		"output": string(output),
		"exit":   0,
	}
	if err != nil {
		result["error"] = err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			result["exit"] = exitErr.ExitCode()
		}
	}

	return result, nil
}

// --- JSON Transform Tool ---

// JSONTransformTool applies simple transformations to JSON data.
type JSONTransformTool struct{}

func (j *JSONTransformTool) Name() string { return "json_transform" }

func (j *JSONTransformTool) Execute(_ context.Context, input map[string]any) (map[string]any, error) {
	data := input["data"]
	operation := toString(input["operation"])

	switch operation {
	case "extract":
		path := toString(input["path"])
		if path == "" {
			return nil, fmt.Errorf("json_transform: path is required for extract")
		}
		val := extractPath(data, path)
		return map[string]any{"result": val}, nil

	case "merge":
		other := input["merge_with"]
		merged := make(map[string]any)
		if m, ok := data.(map[string]any); ok {
			for k, v := range m {
				merged[k] = v
			}
		}
		if m, ok := other.(map[string]any); ok {
			for k, v := range m {
				merged[k] = v
			}
		}
		return map[string]any{"result": merged}, nil

	case "to_json":
		return map[string]any{"result": data}, nil

	default:
		return map[string]any{"result": data}, nil
	}
}

func extractPath(data any, path string) any {
	parts := strings.Split(path, ".")
	var current any = data
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = m[part]
	}
	return current
}

// --- Helpers ---

func toString(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", val)
	}
}

func toFloat64(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	default:
		return 0
	}
}

// --- Prompt Call Tool ---

// PromptCallTool invokes an LLM with a prompt template and variables.
// It accepts "prompt" (the template text) and "variables" (map of substitution values),
// performs {{variable}} substitution, and returns the rendered prompt.
// The actual LLM call is handled by the engine's LLM provider.
type PromptCallTool struct{}

func (p *PromptCallTool) Name() string { return "prompt_call" }

func (p *PromptCallTool) Execute(_ context.Context, input map[string]any) (map[string]any, error) {
	prompt := toString(input["prompt"])
	if prompt == "" {
		return nil, fmt.Errorf("prompt_call tool: prompt is required")
	}

	// Perform variable substitution
	variables, _ := input["variables"].(map[string]any)
	result := prompt
	for k, v := range variables {
		placeholder := "{{" + k + "}}"
		result = strings.ReplaceAll(result, placeholder, toString(v))
	}

	return map[string]any{
		"rendered_prompt": result,
		"variable_count":  len(variables),
	}, nil
}
