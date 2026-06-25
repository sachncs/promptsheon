package workflow

import (
	"testing"
	"time"
)

func TestFromResultLocal(t *testing.T) {
	now := time.Now()
	finished := now.Add(time.Second)
	result := &WorkflowResult{
		WorkflowID: "wf-1",
		Status:     StatusCompleted,
		Outputs:    map[string]any{"answer": "42"},
		Error:      "",
		StartedAt:  now,
		FinishedAt: finished,
		Steps: map[string]*StepResult{
			"step-a": {
				Status:    StatusCompleted,
				Output:    map[string]any{"x": 1},
				ToolCalls: nil,
				LatencyMs: 100,
			},
		},
	}
	wf := FromResult(result, "agent-1", map[string]any{"input": "hi"})

	if wf.ID != "wf-1" {
		t.Errorf("ID: got %q", wf.ID)
	}
	if wf.AgentID != "agent-1" {
		t.Errorf("AgentID: got %q", wf.AgentID)
	}
	if wf.Status != StatusCompleted {
		t.Errorf("Status: got %q", wf.Status)
	}
	if wf.Output["answer"] != "42" {
		t.Errorf("Output: got %v", wf.Output)
	}
	if wf.CompletedAt == nil || !wf.CompletedAt.Equal(finished) {
		t.Errorf("CompletedAt: got %v", wf.CompletedAt)
	}
	if len(wf.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(wf.Steps))
	}
}

func TestFromResultLocalFailed(t *testing.T) {
	result := &WorkflowResult{
		WorkflowID: "wf-2",
		Status:     StatusFailed,
		Error:      "upstream timeout",
	}
	wf := FromResult(result, "agent-1", nil)
	if wf.Error != "upstream timeout" {
		t.Errorf("Error: got %q", wf.Error)
	}
	if wf.Status != StatusFailed {
		t.Errorf("Status: got %q", wf.Status)
	}
	if len(wf.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(wf.Steps))
	}
}

func TestStepByIDWorkflow(t *testing.T) {
	wf := &Workflow{
		Steps: []*StepState{
			{StepID: "first"},
			{StepID: "second"},
		},
	}
	if s := wf.StepByID("first"); s == nil || s.StepID != "first" {
		t.Errorf("expected first step, got %+v", s)
	}
	if s := wf.StepByID("second"); s == nil || s.StepID != "second" {
		t.Errorf("expected second step, got %+v", s)
	}
	if s := wf.StepByID("missing"); s != nil {
		t.Errorf("expected nil for missing step, got %+v", s)
	}
}

func TestSummaryWorkflow(t *testing.T) {
	wf := &Workflow{
		ID:     "wf-1",
		Status: StatusCompleted,
		Steps:  []*StepState{{StepID: "a"}, {StepID: "b"}},
	}
	got := wf.Summary()
	if got == "" {
		t.Error("expected non-empty summary")
	}
	for _, want := range []string{"wf-1", "completed", "2 steps"} {
		if !contains(got, want) {
			t.Errorf("expected summary to contain %q, got %q", want, got)
		}
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
