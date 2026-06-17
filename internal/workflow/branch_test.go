package workflow

import (
	"context"
	"testing"

	"promptsheon/internal/models"
)

func TestConditionEq(t *testing.T) {
	outputs := map[string]any{"status": "success"}
	cond := &models.Condition{Field: "status", Operator: "eq", Value: "success"}
	if !evaluateCondition(cond, outputs) {
		t.Fatal("expected condition to be true")
	}
	cond2 := &models.Condition{Field: "status", Operator: "eq", Value: "failed"}
	if evaluateCondition(cond2, outputs) {
		t.Fatal("expected condition to be false")
	}
}

func TestConditionNeq(t *testing.T) {
	outputs := map[string]any{"status": "success"}
	cond := &models.Condition{Field: "status", Operator: "neq", Value: "failed"}
	if !evaluateCondition(cond, outputs) {
		t.Fatal("expected condition to be true")
	}
}

func TestConditionContains(t *testing.T) {
	outputs := map[string]any{"message": "hello world"}
	cond := &models.Condition{Field: "message", Operator: "contains", Value: "world"}
	if !evaluateCondition(cond, outputs) {
		t.Fatal("expected condition to be true")
	}
}

func TestConditionGt(t *testing.T) {
	outputs := map[string]any{"count": "10"}
	cond := &models.Condition{Field: "count", Operator: "gt", Value: "5"}
	if !evaluateCondition(cond, outputs) {
		t.Fatal("expected 10 > 5")
	}
	cond2 := &models.Condition{Field: "count", Operator: "gt", Value: "15"}
	if evaluateCondition(cond2, outputs) {
		t.Fatal("expected 10 not > 15")
	}
}

func TestConditionExists(t *testing.T) {
	outputs := map[string]any{"key": "value"}
	cond := &models.Condition{Field: "key", Operator: "exists"}
	if !evaluateCondition(cond, outputs) {
		t.Fatal("expected key to exist")
	}
	cond2 := &models.Condition{Field: "missing", Operator: "exists"}
	if evaluateCondition(cond2, outputs) {
		t.Fatal("expected missing key to not exist")
	}
}

func TestBranchingWorkflow(t *testing.T) {
	steps := []models.AgentStep{
		{ID: "check", ToolCalls: []models.ToolCall{{Tool: "json_transform", Input: map[string]any{"data": "ok_status", "operation": "to_json"}}}, OutputKey: "check_result"},
		{ID: "on_success", DependsOn: []string{"check"}, Condition: &models.Condition{Field: "check_result", Operator: "contains", Value: "ok_status"}, ToolCalls: []models.ToolCall{{Tool: "json_transform", Input: map[string]any{"data": "success path", "operation": "to_json"}}}},
		{ID: "on_failure", DependsOn: []string{"check"}, Condition: &models.Condition{Field: "check_result", Operator: "contains", Value: "error_status"}, ToolCalls: []models.ToolCall{{Tool: "json_transform", Input: map[string]any{"data": "failure path", "operation": "to_json"}}}},
	}

	agent := &models.Agent{ID: "branch-test", Steps: steps}
	registry := DefaultRegistry()
	engine := NewEngine(registry)

	result, err := engine.Execute(context.Background(), agent, nil)
	if err != nil {
		t.Fatal(err)
	}

	// "check" and "on_success" should run, "on_failure" should be skipped
	if result.Steps["check"].Status != StatusCompleted {
		t.Fatalf("check should be completed, got %s", result.Steps["check"].Status)
	}
	if result.Steps["on_success"].Status != StatusCompleted {
		t.Fatalf("on_success should be completed, got %s", result.Steps["on_success"].Status)
	}
	if result.Steps["on_failure"].Status != StatusSkipped {
		t.Fatalf("on_failure should be skipped, got %s", result.Steps["on_failure"].Status)
	}
}

func TestPromptCallTool(t *testing.T) {
	tool := &PromptCallTool{}
	if tool.Name() != "prompt_call" {
		t.Fatal("expected name prompt_call")
	}

	output, err := tool.Execute(context.Background(), map[string]any{
		"prompt":    "You are a {{role}}. Answer: {{question}}",
		"variables": map[string]any{"role": "expert", "question": "What is 2+2?"},
	})
	if err != nil {
		t.Fatal(err)
	}

	rendered := output["rendered_prompt"].(string)
	if rendered != "You are a expert. Answer: What is 2+2?" {
		t.Fatalf("unexpected rendered prompt: %s", rendered)
	}
}

func TestPromptCallToolMissingPrompt(t *testing.T) {
	tool := &PromptCallTool{}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
}
