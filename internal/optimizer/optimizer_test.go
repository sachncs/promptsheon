package optimizer_test

import (
	"context"
	"testing"

	"github.com/sachncs/promptsheon/internal/optimizer"
	"github.com/sachncs/promptsheon/internal/optimizer/rules"
)

func TestGetOptimizationTipsHasContent(t *testing.T) {
	t.Parallel()
	tips := optimizer.GetOptimizationTips()
	if len(tips) == 0 {
		t.Fatal("expected non-empty tips")
	}
	for i, tip := range tips {
		if tip == "" {
			t.Errorf("tip %d is empty", i)
		}
	}
}

func TestComputeMetrics(t *testing.T) {
	t.Parallel()
	text := "You are a helpful assistant. Use {{name}} in the response."
	m := optimizer.ComputeMetrics(text)
	if m.WordCount == 0 {
		t.Error("WordCount should be positive")
	}
	if m.VariableCount != 1 {
		t.Errorf("VariableCount = %d, want 1", m.VariableCount)
	}
	if !m.HasSystemPrompt {
		t.Error("HasSystemPrompt should be true")
	}
	if m.EstimatedTokens == 0 {
		t.Error("EstimatedTokens should be positive")
	}
}

func TestAnalyzeProducesReport(t *testing.T) {
	t.Parallel()
	o := optimizer.NewOptimizer()
	report, err := o.Analyze(context.Background(), "p1", "greeting", "Be helpful.", nil)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if report.PromptID != "p1" {
		t.Errorf("PromptID = %q, want p1", report.PromptID)
	}
	if report.Score != 1.0 {
		t.Errorf("Score with no observations = %f, want 1.0", report.Score)
	}
}

func TestAnalyzeUsesObservations(t *testing.T) {
	t.Parallel()
	o := optimizer.NewOptimizer()
	obs := []rules.Observation{{
		CapabilityID:      "c1",
		CapabilityVersion: "v1",
		Environment:       "prod",
		WindowExecutions:  100,
		P95LatencyMS:      5000, // breach
		AvgCostUSDMicro:   50000,
		HallucinationRate: 0.05,
		SuccessRate:       0.95,
	}}
	report, err := o.Analyze(context.Background(), "p1", "slow", "Be helpful.", obs)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(report.Suggestions) == 0 {
		t.Error("expected at least one suggestion for high-latency observation")
	}
	if report.Score >= 1.0 {
		t.Errorf("Score = %f, expected less than 1.0 when suggestions present", report.Score)
	}
}

func TestReportJSONShape(t *testing.T) {
	t.Parallel()
	r := optimizer.OptimizationReport{PromptID: "p1", PromptName: "g", Score: 0.5}
	b, err := r.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !containsJSON(string(b), `"prompt_id":"p1"`) {
		t.Errorf("expected prompt_id in JSON: %s", b)
	}
}

func containsJSON(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
