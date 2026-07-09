package workflow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- Registry tests ---

func TestDefaultRegistry(t *testing.T) {
	r := DefaultRegistry()
	if r == nil {
		t.Fatal("DefaultRegistry returned nil")
	}
	names := r.Tools()
	expected := map[string]bool{"http": true, "shell": true, "json_transform": true, "prompt_call": true}
	for _, n := range names {
		if !expected[n] {
			t.Errorf("unexpected tool %q in default registry", n)
		}
		delete(expected, n)
	}
	for n := range expected {
		t.Errorf("missing tool %q in default registry", n)
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	if _, ok := r.Get("nonexistent"); ok {
		t.Error("expected false for nonexistent tool")
	}

	tool := &HTTPTool{}
	r.Register(tool)
	got, ok := r.Get("http")
	if !ok {
		t.Fatal("expected to find http tool")
	}
	if got.Name() != "http" {
		t.Errorf("expected name http, got %s", got.Name())
	}
}

func TestRegistry_Tools(t *testing.T) {
	r := NewRegistry()
	if len(r.Tools()) != 0 {
		t.Error("expected empty tools list")
	}
	r.Register(&HTTPTool{})
	r.Register(&ShellTool{})
	names := r.Tools()
	if len(names) != 2 {
		t.Errorf("expected 2 tools, got %d", len(names))
	}
}

// --- Engine tests ---

func TestSetContextManager(t *testing.T) {
	e := NewEngine(NewRegistry())
	e.SetContextManager("some-manager")
	// no panic = pass
}

// --- HTTPTool tests ---

func TestHTTPTool_Name(t *testing.T) {
	h := &HTTPTool{}
	if got := h.Name(); got != "http" {
		t.Errorf("expected 'http', got %q", got)
	}
}

func TestHTTPTool_Execute_MissingURL(t *testing.T) {
	h := &HTTPTool{}
	_, err := h.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing url")
	}
}

func TestHTTPTool_Execute_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "ok"}`))
	}))
	defer ts.Close()

	h := &HTTPTool{}
	result, err := h.Execute(context.Background(), map[string]any{"url": ts.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != 200 {
		t.Errorf("expected status 200, got %v", result["status"])
	}
	if _, ok := result["json"]; !ok {
		t.Error("expected json key in result")
	}
}

func TestHTTPTool_Execute_PostWithBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": 1}`))
	}))
	defer ts.Close()

	h := &HTTPTool{}
	result, err := h.Execute(context.Background(), map[string]any{
		"url":    ts.URL,
		"method": "POST",
		"body":   map[string]any{"name": "test"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != 201 {
		t.Errorf("expected status 201, got %v", result["status"])
	}
}

func TestHTTPTool_Execute_BodyMarshalError(t *testing.T) {
	h := &HTTPTool{}
	_, err := h.Execute(context.Background(), map[string]any{
		"url":  "http://example.com",
		"body": map[string]any{"ch": make(chan int)},
	})
	if err == nil {
		t.Fatal("expected error for unserializable body")
	}
}

func TestHTTPTool_Execute_RequestCreationError(t *testing.T) {
	h := &HTTPTool{}
	_, err := h.Execute(context.Background(), map[string]any{
		"url": "http://[::1]:namedport",
	})
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestHTTPTool_Execute_DoError(t *testing.T) {
	h := &HTTPTool{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := h.Execute(ctx, map[string]any{
		"url": "http://127.0.0.1:1",
	})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestHTTPTool_Execute_WithHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token123" {
			t.Errorf("expected Authorization header")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	h := &HTTPTool{}
	_, err := h.Execute(context.Background(), map[string]any{
		"url":     ts.URL,
		"headers": map[string]any{"Authorization": "Bearer token123"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- ShellTool tests ---

func TestShellTool_Name(t *testing.T) {
	s := &ShellTool{}
	if got := s.Name(); got != "shell" {
		t.Errorf("expected 'shell', got %q", got)
	}
}

func TestSetShellToolPolicy(t *testing.T) {
	saved := globalShellPolicy
	defer func() { globalShellPolicy = saved }()

	p := newShellPolicy()
	globalShellPolicy = p

	SetShellToolPolicy(true, []string{"ls", "echo"})
	if !p.Enabled() {
		t.Error("expected enabled")
	}
	allowed := p.Allowed()
	if !allowed["ls"] || !allowed["echo"] {
		t.Error("expected ls and echo in allowlist")
	}
}

func TestShellTool_DisabledByEmptyAllowlist(t *testing.T) {
	saved := globalShellPolicy
	defer func() { globalShellPolicy = saved }()

	p := newShellPolicy()
	p.Set(true, nil)
	globalShellPolicy = p

	tool := &ShellTool{}
	_, err := tool.Execute(context.Background(), map[string]any{"command": "ls"})
	if err == nil {
		t.Fatal("expected error for empty allowlist")
	}
}

func TestShellTool_BlockedPattern(t *testing.T) {
	saved := globalShellPolicy
	defer func() { globalShellPolicy = saved }()

	p := newShellPolicy()
	p.Set(true, []string{"rm"})
	globalShellPolicy = p

	tool := &ShellTool{}
	_, err := tool.Execute(context.Background(), map[string]any{"command": "rm -rf /"})
	if err == nil {
		t.Fatal("expected error for blocked pattern")
	}
}

func TestShellTool_CommandNotInAllowlist(t *testing.T) {
	saved := globalShellPolicy
	defer func() { globalShellPolicy = saved }()

	p := newShellPolicy()
	p.Set(true, []string{"echo"})
	globalShellPolicy = p

	tool := &ShellTool{}
	_, err := tool.Execute(context.Background(), map[string]any{"command": "ls"})
	if err == nil {
		t.Fatal("expected error for command not in allowlist")
	}
}

func TestShellTool_EmptyCommand(t *testing.T) {
	saved := globalShellPolicy
	defer func() { globalShellPolicy = saved }()

	p := newShellPolicy()
	p.Set(true, []string{"echo"})
	globalShellPolicy = p

	tool := &ShellTool{}
	_, err := tool.Execute(context.Background(), map[string]any{"command": ""})
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestShellTool_Success(t *testing.T) {
	saved := globalShellPolicy
	defer func() { globalShellPolicy = saved }()

	p := newShellPolicy()
	p.Set(true, []string{"echo"})
	globalShellPolicy = p

	tool := &ShellTool{}
	result, err := tool.Execute(context.Background(), map[string]any{"command": "echo hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["exit"] != 0 {
		t.Errorf("expected exit code 0, got %v", result["exit"])
	}
	if result["output"] != "hello\n" {
		t.Errorf("expected 'hello\\n', got %q", result["output"])
	}
}

func TestShellTool_UnterminatedQuote(t *testing.T) {
	saved := globalShellPolicy
	defer func() { globalShellPolicy = saved }()

	p := newShellPolicy()
	p.Set(true, []string{"echo"})
	globalShellPolicy = p

	tool := &ShellTool{}
	_, err := tool.Execute(context.Background(), map[string]any{"command": `echo "unterminated`})
	if err == nil {
		t.Fatal("expected error for unterminated quote")
	}
}

func TestShellTool_NonZeroExit(t *testing.T) {
	saved := globalShellPolicy
	defer func() { globalShellPolicy = saved }()

	p := newShellPolicy()
	p.Set(true, []string{"false"})
	globalShellPolicy = p

	tool := &ShellTool{}
	result, err := tool.Execute(context.Background(), map[string]any{"command": "false"})
	if err == nil {
		t.Fatal("expected error for failed command")
	}
	if result["exit"] != 1 {
		t.Errorf("expected exit code 1, got %v", result["exit"])
	}
}

func TestShellTool_WithTimeout(t *testing.T) {
	saved := globalShellPolicy
	defer func() { globalShellPolicy = saved }()

	p := newShellPolicy()
	p.Set(true, []string{"echo"})
	globalShellPolicy = p

	tool := &ShellTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"command": "echo hello",
		"timeout": float64(5),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["exit"] != 0 {
		t.Errorf("expected exit code 0, got %v", result["exit"])
	}
}

// --- JSONTransformTool tests ---

func TestJSONTransformTool_Name(t *testing.T) {
	j := &JSONTransformTool{}
	if got := j.Name(); got != "json_transform" {
		t.Errorf("expected 'json_transform', got %q", got)
	}
}

func TestJSONTransformTool_Extract(t *testing.T) {
	j := &JSONTransformTool{}
	result, err := j.Execute(context.Background(), map[string]any{
		"data":      map[string]any{"a": map[string]any{"b": "c"}},
		"operation": "extract",
		"path":      "a.b",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["result"] != "c" {
		t.Errorf("expected 'c', got %v", result["result"])
	}
}

func TestJSONTransformTool_ExtractMissingPath(t *testing.T) {
	j := &JSONTransformTool{}
	_, err := j.Execute(context.Background(), map[string]any{
		"data":      map[string]any{},
		"operation": "extract",
		"path":      "",
	})
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestJSONTransformTool_Merge(t *testing.T) {
	j := &JSONTransformTool{}
	result, err := j.Execute(context.Background(), map[string]any{
		"data":       map[string]any{"a": float64(1)},
		"operation":  "merge",
		"merge_with": map[string]any{"b": float64(2)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	merged, ok := result["result"].(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	if merged["a"] != float64(1) || merged["b"] != float64(2) {
		t.Errorf("merge result unexpected: %v", merged)
	}
}

func TestJSONTransformTool_MergeNonMapData(t *testing.T) {
	j := &JSONTransformTool{}
	result, err := j.Execute(context.Background(), map[string]any{
		"data":       "not a map",
		"operation":  "merge",
		"merge_with": map[string]any{"b": float64(2)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	merged, ok := result["result"].(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	if merged["b"] != float64(2) {
		t.Errorf("expected b=2, got %v", merged["b"])
	}
}

func TestJSONTransformTool_ToJSON(t *testing.T) {
	j := &JSONTransformTool{}
	result, err := j.Execute(context.Background(), map[string]any{
		"data":      map[string]any{"key": "val"},
		"operation": "to_json",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["result"] == nil {
		t.Error("expected non-nil result for to_json")
	}
}

func TestJSONTransformTool_Default(t *testing.T) {
	j := &JSONTransformTool{}
	result, err := j.Execute(context.Background(), map[string]any{
		"data":      "raw",
		"operation": "unknown",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["result"] != "raw" {
		t.Errorf("expected 'raw', got %v", result["result"])
	}
}

// --- extractPath tests ---

func TestExtractPath(t *testing.T) {
	data := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": "deep",
			},
		},
	}
	if got := extractPath(data, "a.b.c"); got != "deep" {
		t.Errorf("expected 'deep', got %v", got)
	}
}

func TestExtractPath_NonMapIntermediate(t *testing.T) {
	if got := extractPath(map[string]any{"a": "leaf"}, "a.b"); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestExtractPath_MissingKey(t *testing.T) {
	if got := extractPath(map[string]any{"a": map[string]any{}}, "a.b"); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestExtractPath_NilData(t *testing.T) {
	if got := extractPath(nil, "a"); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// --- shellBaseCommand tests ---

func TestShellBaseCommand(t *testing.T) {
	cmd, err := shellBaseCommand("echo hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "echo" {
		t.Errorf("expected 'echo', got %q", cmd)
	}
}

func TestShellBaseCommand_QuotedBasename(t *testing.T) {
	cmd, err := shellBaseCommand(`"echo" hello`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "echo" {
		t.Errorf("expected 'echo', got %q", cmd)
	}
}

func TestShellBaseCommand_Empty(t *testing.T) {
	_, err := shellBaseCommand("")
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

// --- toString tests ---

func TestToString(t *testing.T) {
	tests := []struct {
		input any
		want  string
	}{
		{nil, ""},
		{"hello", "hello"},
		{float64(3.14), "3.14"},
		{true, "true"},
		{false, "false"},
		{42, "42"},
		{[]int{1, 2}, "[1 2]"},
	}
	for _, tt := range tests {
		if got := toString(tt.input); got != tt.want {
			t.Errorf("toString(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- toFloat64 tests ---

func TestToFloat64(t *testing.T) {
	tests := []struct {
		input any
		want  float64
	}{
		{float64(3.14), 3.14},
		{42, float64(42)},
		{"3.14", 3.14},
		{"invalid", float64(0)},
		{true, 0},
		{nil, 0},
	}
	for _, tt := range tests {
		if got := toFloat64(tt.input); got != tt.want {
			t.Errorf("toFloat64(%v) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// --- shlexSplit tests ---

func TestShlexSplit(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"echo hello world", []string{"echo", "hello", "world"}},
		{"  spaced  ", []string{"spaced"}},
		{"", nil},
		{`echo "hello world"`, []string{"echo", "hello world"}},
		{`echo 'single quoted'`, []string{"echo", "single quoted"}},
		{`echo hello\ world`, []string{"echo", "hello\\", "world"}},
		{"line1\nline2", []string{"line1", "line2"}},
	}
	for _, tt := range tests {
		got, err := shlexSplit(tt.input)
		if err != nil {
			t.Errorf("shlexSplit(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if len(got) != len(tt.want) {
			t.Errorf("shlexSplit(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("shlexSplit(%q) = %v, want %v", tt.input, got, tt.want)
				break
			}
		}
	}
}

func TestShlexSplit_UnterminatedQuote(t *testing.T) {
	_, err := shlexSplit(`echo "unterminated`)
	if err == nil {
		t.Fatal("expected error for unterminated quote")
	}
}

// --- PromptCallTool tests ---

func TestPromptCallTool_Name(t *testing.T) {
	p := &PromptCallTool{}
	if got := p.Name(); got != "prompt_call" {
		t.Errorf("expected 'prompt_call', got %q", got)
	}
}

func TestPromptCallTool_Execute_MissingPrompt(t *testing.T) {
	p := &PromptCallTool{}
	_, err := p.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
}

func TestPromptCallTool_Execute_Basic(t *testing.T) {
	p := &PromptCallTool{}
	result, err := p.Execute(context.Background(), map[string]any{
		"prompt": "Hello, world!",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["rendered_prompt"] != "Hello, world!" {
		t.Errorf("expected 'Hello, world!', got %q", result["rendered_prompt"])
	}
	if result["variable_count"] != 0 {
		t.Errorf("expected 0 variables, got %v", result["variable_count"])
	}
}

func TestPromptCallTool_Execute_WithVariables(t *testing.T) {
	p := &PromptCallTool{}
	result, err := p.Execute(context.Background(), map[string]any{
		"prompt":    "Hello {{name}}, you are {{age}} years old",
		"variables": map[string]any{"name": "Alice", "age": float64(30)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "Hello Alice, you are 30 years old"
	if result["rendered_prompt"] != expected {
		t.Errorf("expected %q, got %q", expected, result["rendered_prompt"])
	}
	if result["variable_count"] != 2 {
		t.Errorf("expected 2 variables, got %v", result["variable_count"])
	}
}
