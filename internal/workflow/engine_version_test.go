package workflow

import (
	"context"
	"testing"

	"github.com/sachncs/promptsheon/internal/capability"
)

func TestExecuteVersion_Basic(t *testing.T) {
	e := NewEngine(NewRegistry())
	ctx := context.Background()

	version := &capability.Version{
		ID:      "ver-1",
		Version: 1,
		Prompt: capability.Prompt{
			Instructions: "Process the input and return a summary",
		},
		RuntimePolicy: capability.RuntimePolicy{
			MaxTokens: 1024,
		},
	}

	input := map[string]any{"text": "hello world"}

	result, err := e.ExecuteVersion(ctx, version, input)
	if err != nil {
		t.Fatalf("ExecuteVersion: %v", err)
	}
	if result.Status != StatusCompleted {
		t.Errorf("expected completed status, got %s", result.Status)
	}
	if result.WorkflowID != "ver-1" {
		t.Errorf("expected workflow ID ver-1")
	}
	if _, ok := result.Steps["main"]; !ok {
		t.Errorf("expected step 'main'")
	}
	// Input should be propagated to outputs
	if result.Outputs["text"] != "hello world" {
		t.Errorf("input not propagated to outputs")
	}
}

func TestExecuteVersion_NilVersion(t *testing.T) {
	e := NewEngine(NewRegistry())
	_, err := e.ExecuteVersion(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error for nil version")
	}
}

func TestExecuteVersion_WithGuardrails(t *testing.T) {
	e := NewEngine(NewRegistry())
	ctx := context.Background()

	// Set up a mock guardrail manager that always passes
	mockMgr := &mockGuardrailChecker{shouldFail: false}
	e.SetGuardrails(mockMgr, nil)

	version := &capability.Version{
		ID:      "ver-1",
		Version: 1,
		Prompt: capability.Prompt{
			Instructions: "test",
		},
		Guardrails: []capability.Guardrail{
			{ID: "gr-1", Name: "test-guard", Phase: capability.GuardrailPhasePre},
		},
	}

	result, err := e.ExecuteVersion(ctx, version, nil)
	if err != nil {
		t.Fatalf("ExecuteVersion: %v", err)
	}
	if result.Status != StatusCompleted {
		t.Errorf("expected completed when guardrails pass, got %s", result.Status)
	}
}

func TestExecuteVersion_GuardrailFailure(t *testing.T) {
	e := NewEngine(NewRegistry())
	ctx := context.Background()

	mockMgr := &mockGuardrailChecker{shouldFail: true}
	e.SetGuardrails(mockMgr, nil)

	version := &capability.Version{
		ID:      "ver-1",
		Version: 1,
		Prompt: capability.Prompt{
			Instructions: "test",
		},
		Guardrails: []capability.Guardrail{
			{ID: "gr-1", Name: "failing-guard", Phase: capability.GuardrailPhasePre},
		},
	}

	result, err := e.ExecuteVersion(ctx, version, nil)
	if err != nil {
		t.Fatalf("ExecuteVersion: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("expected failed status when guardrails fail, got %s", result.Status)
	}
}

// mockGuardrailChecker implements the guardrail check interface expected by the engine.
type mockGuardrailChecker struct {
	shouldFail bool
}

func (m *mockGuardrailChecker) CheckVersion(_ context.Context, _ *capability.Version) error {
	if m.shouldFail {
		return &guardrailError{msg: "guardrail check failed"}
	}
	return nil
}

type guardrailError struct {
	msg string
}

func (e *guardrailError) Error() string { return e.msg }
