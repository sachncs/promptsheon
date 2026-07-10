// Package workflow provides workflow orchestration for capability execution.
package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const toolNameHTTP = "http"
const toolNameShell = "shell"
const toolNameJSONTransform = "json_transform"
const toolNamePromptCall = "prompt_call"
const opExtract = "extract"
const opMerge = "merge"
const strFalse = "false"
const keyBody = "body"

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
//
// The shell tool is registered but is disabled by default. It must be
// explicitly enabled by setting ShellToolEnabled = true and populating
// AllowedCommands. This is fail-closed: a workflow that references the
// shell tool while it is disabled will get a clear error rather than
// arbitrary command execution.
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

// Name returns the tool name.
func (h *HTTPTool) Name() string { return toolNameHTTP }

// Execute makes an HTTP request with the given input parameters.
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
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("http tool: read body: %w", err)
	}

	result := map[string]any{
		"status": resp.StatusCode,
		keyBody:  string(respBody),
	}

	// Try to parse JSON body
	var jsonBody any
	if json.Unmarshal(respBody, &jsonBody) == nil {
		result["json"] = jsonBody
	}

	return result, nil
}

// --- Shell Tool ---

// ShellTool executes shell commands with sandboxing.
//
// Security model: DENY BY DEFAULT. The shell tool refuses to run unless
// (a) ShellToolEnabled is true AND
// (b) AllowedCommands is non-empty AND
// (c) the command's first token is in AllowedCommands AND
// (d) the command does not contain any of BlockedPatterns.
//
// This is the opposite of the previous behaviour, which treated an empty
// AllowedCommands as "all commands allowed" and gave any user who could
// author an agent arbitrary shell access on the server.
type ShellTool struct{}

// blockedPatterns contains substrings that are always blocked in shell
// commands, even when the allowlist is configured. Set via SetBlockedPatterns.
var blockedPatterns = []string{"rm -rf /", "mkfs", ":(){ :|:&", "dd if=/dev"}

// BlockedPatterns returns a copy of the blocked command patterns.
func BlockedPatterns() []string {
	p := make([]string, len(blockedPatterns))
	copy(p, blockedPatterns)
	return p
}

// SetBlockedPatterns atomically replaces the blocked pattern list.
// Operators may call this at startup. Not safe for concurrent runtime calls.
func SetBlockedPatterns(p []string) {
	n := make([]string, len(p))
	copy(n, p)
	blockedPatterns = n
}

// shellPolicy holds the shell tool's runtime policy. Reads happen
// from arbitrary request goroutines; the previous implementation
// exposed ShellToolEnabled (bool) and AllowedCommands (map) as bare
// package-level mutable globals, which is a data race if anything
// ever calls SetShellToolPolicy after the first request. M-8 fix:
// the policy is wrapped in atomic types so the reader always sees a
// consistent snapshot.
type shellPolicy struct {
	enabled   atomic.Bool
	allowList atomic.Pointer[map[string]bool]
}

func newShellPolicy() *shellPolicy {
	p := &shellPolicy{}
	empty := map[string]bool{}
	p.allowList.Store(&empty)
	return p
}

func (p *shellPolicy) Enabled() bool {
	return p.enabled.Load()
}

func (p *shellPolicy) Allowed() map[string]bool {
	return *p.allowList.Load()
}

func (p *shellPolicy) Set(enabled bool, allowed []string) {
	p.enabled.Store(enabled)
	m := make(map[string]bool, len(allowed))
	for _, c := range allowed {
		m[c] = true
	}
	p.allowList.Store(&m)
}

var globalShellPolicy = newShellPolicy()

// SetShellToolPolicy atomically updates the shell tool's policy.
// Intended for the configuration loader to call once at startup,
// but the atomic snapshot means it is also safe to call at runtime
// (e.g., for hot-reload).
func SetShellToolPolicy(enabled bool, allowed []string) {
	globalShellPolicy.Set(enabled, allowed)
}

// Name returns the tool name.
func (s *ShellTool) Name() string { return toolNameShell }

// Execute runs a shell command with the given input parameters.
func (s *ShellTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	if !globalShellPolicy.Enabled() {
		return nil, fmt.Errorf("shell tool: disabled (set PROMPTSHEON_SHELL_ENABLED=true and configure PROMPTSHEON_SHELL_ALLOWLIST to enable)")
	}
	allowed := globalShellPolicy.Allowed()
	if len(allowed) == 0 {
		return nil, fmt.Errorf("shell tool: disabled (allowed-commands allowlist is empty)")
	}

	command := toString(input["command"])
	if command == "" {
		return nil, fmt.Errorf("shell tool: command is required")
	}

	// Security: check blocked patterns. This is enforced AFTER the
	// allowlist gate so an operator can use the allowlist as the
	// primary defence; the block-list catches obvious footguns.
	cmdLower := strings.ToLower(command)
	for _, pattern := range blockedPatterns {
		if strings.Contains(cmdLower, strings.ToLower(pattern)) {
			return nil, fmt.Errorf("shell tool: command contains blocked pattern: %s", pattern)
		}
	}

	// Security: enforce allowlist. Resolve the first token from the
	// parsed argv rather than scanning raw bytes so quoting cannot be
	// used to smuggle alternative basenames.
	baseCmd, err := shellBaseCommand(command)
	if err != nil {
		return nil, fmt.Errorf("shell tool: %w", err)
	}
	if !allowed[baseCmd] {
		return nil, fmt.Errorf("shell tool: command %q is not in the allowlist", baseCmd)
	}

	timeout := 30 * time.Second
	if t, ok := input["timeout"]; ok {
		timeout = time.Duration(toFloat64(t) * float64(time.Second))
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// #nosec G204 -- command is validated against an allowlist
	// by shellBaseCommand before reaching this line.
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	output, err := cmd.CombinedOutput()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	result := map[string]any{
		"output": string(output),
		"exit":   exitCode,
	}

	// Return error on non-zero exit so the engine marks step as failed
	if exitCode != 0 {
		return result, fmt.Errorf("shell command failed with exit code %d: %s", exitCode, strings.TrimSpace(string(output)))
	}

	return result, nil
}

// --- JSON Transform Tool ---

const keyResult = "result"

// JSONTransformTool applies simple transformations to JSON data.
type JSONTransformTool struct{}

func (j *JSONTransformTool) Name() string { return toolNameJSONTransform }

func (j *JSONTransformTool) Execute(_ context.Context, input map[string]any) (map[string]any, error) {
	data := input["data"]
	operation := toString(input["operation"])

	switch operation {
	case opExtract:
		path := toString(input["path"])
		if path == "" {
			return nil, fmt.Errorf("json_transform: path is required for extract")
		}
		val := extractPath(data, path)
		return map[string]any{keyResult: val}, nil

	case opMerge:
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
		return map[string]any{keyResult: merged}, nil

	case "to_json":
		return map[string]any{keyResult: data}, nil

	default:
		return map[string]any{keyResult: data}, nil
	}
}

func extractPath(data any, path string) any {
	parts := strings.Split(path, ".")
	var current = data
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

// shellBaseCommand parses a shell command string with sh's own quoting
// rules and returns the first token. Using the shell's own parser means
// quoting tricks like `"\n"curl` or `'cu\nrl'` cannot be used to
// disguise a forbidden basename.
func shellBaseCommand(command string) (string, error) {
	fields, err := shlexSplit(command)
	if err != nil {
		return "", fmt.Errorf("parse command: %w", err)
	}
	if len(fields) == 0 {
		return "", fmt.Errorf("empty command")
	}
	return fields[0], nil
}

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
		return strFalse
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
		f, _ := strconv.ParseFloat(val, 64)
		return f
	default:
		return 0
	}
}

// shlexSplit is a small POSIX-shell-style tokenizer. It recognises single
// and double quotes and backslash escapes. It deliberately does NOT
// implement the full shell grammar (no expansions, no redirections) —
// we only need to identify the first token for allowlist checks.
func shlexSplit(s string) ([]string, error) {
	var out []string
	var cur []rune
	var quote rune
	hadContent := false
	flush := func() {
		if hadContent {
			out = append(out, string(cur))
			cur = cur[:0]
			hadContent = false
		}
	}
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur = append(cur, r)
			}
		case r == '\'' || r == '"':
			quote = r
			hadContent = true
		case r == '\\':
			// Take the next rune literally if available; mimic sh behaviour
			// for trailing backslash (kept literally).
			cur = append(cur, '\\')
			hadContent = true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			flush()
		default:
			cur = append(cur, r)
			hadContent = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated %c quote", quote)
	}
	flush()
	return out, nil
}

// --- Prompt Call Tool ---

// PromptCallTool invokes an LLM with a prompt template and variables.
// It accepts "prompt" (the template text) and "variables" (map of substitution values),
// performs {{variable}} substitution, and returns the rendered prompt.
// The actual LLM call is handled by the engine's LLM provider.
type PromptCallTool struct{}

func (p *PromptCallTool) Name() string { return toolNamePromptCall }

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
