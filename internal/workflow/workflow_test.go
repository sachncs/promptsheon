package workflow

import (
	"context"
	"fmt"
	"testing"

	"promptsheon/internal/models"
)

func TestValidateDAG(t *testing.T) {
	// Valid DAG
	steps := []models.AgentStep{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"a", "b"}},
	}
	if err := validateDAG(steps); err != nil {
		t.Fatalf("expected valid DAG, got %v", err)
	}

	// Cycle
	cyclic := []models.AgentStep{
		{ID: "a", DependsOn: []string{"c"}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"b"}},
	}
	if err := validateDAG(cyclic); err == nil {
		t.Fatal("expected cycle detection")
	}
}

func TestTopologicalLevels(t *testing.T) {
	steps := []models.AgentStep{
		{ID: "a", DependsOn: []string{}},
		{ID: "b", DependsOn: []string{}},
		{ID: "c", DependsOn: []string{"a"}},
		{ID: "d", DependsOn: []string{"a", "b"}},
		{ID: "e", DependsOn: []string{"c", "d"}},
	}

	levels := topologicalLevels(steps)
	if len(levels) != 3 {
		t.Fatalf("expected 3 levels, got %d", len(levels))
	}
	// Level 0: a, b (independent)
	if len(levels[0]) != 2 {
		t.Fatalf("expected 2 steps in level 0, got %d", len(levels[0]))
	}
	// Level 1: c, d (depend on a/b)
	if len(levels[1]) != 2 {
		t.Fatalf("expected 2 steps in level 1, got %d", len(levels[1]))
	}
	// Level 2: e (depends on c, d)
	if len(levels[2]) != 1 {
		t.Fatalf("expected 1 step in level 2, got %d", len(levels[2]))
	}
}

func TestEngineSequential(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&mockTool{output: map[string]any{"result": "ok"}})
	engine := NewEngine(registry)

	agent := &models.Agent{
		ID: "agent-1",
		Steps: []models.AgentStep{
			{ID: "step-1", ToolCalls: []models.ToolCall{{Tool: "mock"}}},
			{ID: "step-2", DependsOn: []string{"step-1"}, ToolCalls: []models.ToolCall{{Tool: "mock"}}},
		},
	}

	result, err := engine.Execute(context.Background(), agent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", result.Status)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 step results, got %d", len(result.Steps))
	}
}

func TestEngineParallel(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&mockTool{output: map[string]any{"result": "ok"}})
	engine := NewEngine(registry)

	agent := &models.Agent{
		ID: "agent-parallel",
		Steps: []models.AgentStep{
			{ID: "a", ToolCalls: []models.ToolCall{{Tool: "mock"}}},
			{ID: "b", ToolCalls: []models.ToolCall{{Tool: "mock"}}},
			{ID: "c", DependsOn: []string{"a", "b"}, ToolCalls: []models.ToolCall{{Tool: "mock"}}},
		},
	}

	result, err := engine.Execute(context.Background(), agent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", result.Status)
	}
}

func TestEngineStepFailure(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&mockTool{err: fmt.Errorf("tool exploded")})
	engine := NewEngine(registry)

	agent := &models.Agent{
		ID: "agent-fail",
		Steps: []models.AgentStep{
			{ID: "fail-step", ToolCalls: []models.ToolCall{{Tool: "mock"}}},
			{ID: "downstream", DependsOn: []string{"fail-step"}, ToolCalls: []models.ToolCall{{Tool: "mock"}}},
		},
	}

	result, err := engine.Execute(context.Background(), agent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Fatalf("expected failed, got %s", result.Status)
	}
	if result.Steps["downstream"].Status != StatusSkipped {
		t.Fatalf("expected downstream skipped, got %s", result.Steps["downstream"].Status)
	}
}

func TestEngineInputOutput(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&mockTool{output: map[string]any{"value": 42}})
	engine := NewEngine(registry)

	agent := &models.Agent{
		ID: "agent-io",
		Steps: []models.AgentStep{
			{ID: "step-1", ToolCalls: []models.ToolCall{{Tool: "mock"}}, OutputKey: "step1_output"},
		},
	}

	input := map[string]any{"question": "what is 6*7?"}
	result, err := engine.Execute(context.Background(), agent, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Outputs["question"] != "what is 6*7?" {
		t.Fatal("expected input preserved in outputs")
	}
}

func TestEngineEmptyWorkflow(t *testing.T) {
	registry := NewRegistry()
	engine := NewEngine(registry)

	agent := &models.Agent{ID: "empty", Steps: []models.AgentStep{}}
	_, err := engine.Execute(context.Background(), agent, nil)
	if err == nil {
		t.Fatal("expected error for empty workflow")
	}
}

func TestEngineCycle(t *testing.T) {
	registry := NewRegistry()
	engine := NewEngine(registry)

	agent := &models.Agent{
		ID: "cyclic",
		Steps: []models.AgentStep{
			{ID: "a", DependsOn: []string{"b"}},
			{ID: "b", DependsOn: []string{"a"}},
		},
	}

	_, err := engine.Execute(context.Background(), agent, nil)
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestToolRegistry(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{})

	tool, ok := r.Get("mock")
	if !ok {
		t.Fatal("expected to find mock tool")
	}
	if tool.Name() != "mock" {
		t.Fatalf("expected mock, got %s", tool.Name())
	}

	_, ok = r.Get("nonexistent")
	if ok {
		t.Fatal("expected not found")
	}

	names := r.Tools()
	if len(names) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(names))
	}
}

func TestFromResult(t *testing.T) {
	result := &WorkflowResult{
		WorkflowID: "wf-1",
		Status:     StatusCompleted,
		Steps: map[string]*StepResult{
			"s1": {StepID: "s1", Status: StatusCompleted, Output: map[string]any{"k": "v"}},
		},
		Outputs: map[string]any{"final": "result"},
	}

	wf := FromResult(result, "agent-1", map[string]any{"input": "val"})
	if wf.AgentID != "agent-1" {
		t.Fatalf("expected agent-1, got %s", wf.AgentID)
	}
	if len(wf.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(wf.Steps))
	}
}

// --- mock tool for tests ---

type mockTool struct {
	output map[string]any
	err    error
}

func (m *mockTool) Name() string { return "mock" }

func (m *mockTool) Execute(_ context.Context, input map[string]any) (map[string]any, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.output, nil
}
