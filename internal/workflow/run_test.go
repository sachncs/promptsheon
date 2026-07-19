package workflow

import (
	"context"
	"errors"
	"testing"
)

func TestEngineRun_HappyPath(t *testing.T) {
	e := NewEngine(DefaultRegistry())
	def := Definition{
		ID: "w1",
		Steps: []Step{
			{ID: "extract", Tool: "json_transform", Input: map[string]any{
				"operation": "extract",
				"path":      "name",
				"data":      map[string]any{"name": "alice", "age": 30},
			}, Output: "name"},
			{ID: "use", Tool: "http", Input: map[string]any{
				"method": "GET",
				"url":    "https://example.com",
			}},
		},
	}
	res, err := e.Run(context.Background(), def, map[string]any{"seed": 1})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != StatusCompleted {
		t.Fatalf("status = %s want completed", res.Status)
	}
	if res.Steps["extract"] == nil || res.Steps["use"] == nil {
		t.Fatalf("missing step results: %+v", res.Steps)
	}
	if v, ok := res.Outputs["name"].(map[string]any); !ok || v["result"] != "alice" {
		t.Fatalf("expected cross-step output to propagate, got %v", res.Outputs["name"])
	}
}

func TestEngineRun_StepFailureShortCircuits(t *testing.T) {
	e := NewEngine(DefaultRegistry())
	def := Definition{
		ID: "w2",
		Steps: []Step{
			{ID: "boom", Tool: "no_such_tool"},
			{ID: "after", Tool: "http", Input: map[string]any{"url": "https://example.com"}},
		},
	}
	res, err := e.Run(context.Background(), def, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if res.Status != StatusFailed {
		t.Fatalf("status = %s want failed", res.Status)
	}
	if res.Steps["after"] != nil {
		t.Fatal("after step should not have run")
	}
}

func TestEngineRun_EmptyDefinition(t *testing.T) {
	e := NewEngine(DefaultRegistry())
	_, err := e.Run(context.Background(), Definition{ID: "w3"}, nil)
	if !errors.Is(err, err) || err == nil {
		t.Fatalf("expected error for empty steps, got %v", err)
	}
}
