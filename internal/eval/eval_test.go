package eval

import (
	"context"
	"testing"

	"promptsheon/internal/llm"
	"promptsheon/internal/models"
)

func TestRunnerBasic(t *testing.T) {
	provider := llm.NewMock("The answer is 42")
	runner := NewRunner(provider, ContainsScorer{})

	dataset := &models.TestDataset{
		ID:   "ds-1",
		Name: "test",
		Cases: []models.TestCase{
			{ID: "tc-1", Input: map[string]any{"q": "what"}, ExpectedContains: []string{"42"}},
			{ID: "tc-2", Input: map[string]any{"q": "why"}, ExpectedContains: []string{"42"}},
		},
	}

	report, err := runner.Run(context.Background(), &RunConfig{
		PromptHash: "hash-abc",
		PromptText: "Answer the question: {{q}}",
		Dataset:    dataset,
		Model:      "test-model",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Aggregate.TotalCases != 2 {
		t.Fatalf("expected 2 cases, got %d", report.Aggregate.TotalCases)
	}
	if report.Aggregate.PassRate != 1.0 {
		t.Fatalf("expected 100%% pass rate, got %f", report.Aggregate.PassRate)
	}
	// 2 test cases × 1 call each + 2 × 2 hallucination checks = 6 total
	if provider.CallCount() != 6 {
		t.Fatalf("expected 6 LLM calls, got %d", provider.CallCount())
	}
}

func TestRunnerFailingCases(t *testing.T) {
	provider := llm.NewMock("I don't know")
	runner := NewRunner(provider, ContainsScorer{})

	dataset := &models.TestDataset{
		ID: "ds-fail",
		Cases: []models.TestCase{
			{ID: "tc-1", ExpectedContains: []string{"42"}},
		},
	}

	report, err := runner.Run(context.Background(), &RunConfig{
		PromptHash: "hash-fail",
		PromptText: "test",
		Dataset:    dataset,
		Model:      "m",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Aggregate.PassRate != 0.0 {
		t.Fatalf("expected 0%% pass rate, got %f", report.Aggregate.PassRate)
	}
}

func TestRunnerEmptyExpectations(t *testing.T) {
	provider := llm.NewMock("anything")
	runner := NewRunner(provider)

	dataset := &models.TestDataset{
		ID: "ds-empty",
		Cases: []models.TestCase{
			{ID: "tc-1", Input: map[string]any{"q": "hi"}},
		},
	}

	report, err := runner.Run(context.Background(), &RunConfig{
		PromptHash: "h",
		PromptText: "Answer: {{q}}",
		Dataset:    dataset,
		Model:      "m",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Aggregate.PassRate != 1.0 {
		t.Fatalf("expected 100%% pass rate with no expectations, got %f", report.Aggregate.PassRate)
	}
}

func TestRunnerPromptVariableSubstitution(t *testing.T) {
	provider := llm.NewMock("ok")
	runner := NewRunner(provider)

	dataset := &models.TestDataset{
		ID: "ds-vars",
		Cases: []models.TestCase{
			{ID: "tc-1", Input: map[string]any{"name": "Alice", "age": "30"}},
		},
	}

	_, err := runner.Run(context.Background(), &RunConfig{
		PromptHash: "h",
		PromptText: "Hello {{name}}, you are {{age}}.",
		Dataset:    dataset,
		Model:      "m",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// First call is the actual eval, subsequent calls are hallucination checks
	call := provider.Calls[0]
	if call.Messages[0].Content != "Hello Alice, you are 30." {
		t.Fatalf("unexpected prompt substitution: %q", call.Messages[0].Content)
	}
}

func TestBatchRunner(t *testing.T) {
	provider := llm.NewMock("batch result")
	runner := NewRunner(provider)
	batch := NewBatchRunner(runner)

	datasets := []*models.TestDataset{
		{ID: "ds-1", Cases: []models.TestCase{{ID: "tc-1", ExpectedContains: []string{"batch"}}}},
		{ID: "ds-2", Cases: []models.TestCase{{ID: "tc-2", ExpectedContains: []string{"batch"}}}},
		{ID: "ds-3", Cases: []models.TestCase{{ID: "tc-3", ExpectedContains: []string{"batch"}}}},
	}

	report, err := batch.RunBatch(context.Background(), &BatchConfig{
		PromptHash: "batch-hash",
		PromptText: "test",
		Datasets:   datasets,
		Model:      "m",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(report.Results))
	}
	if report.Aggregate.TotalCases != 3 {
		t.Fatalf("expected 3 total cases, got %d", report.Aggregate.TotalCases)
	}
	// 3 datasets × (1 eval + 2 hallucination checks) = 9
	if provider.CallCount() != 9 {
		t.Fatalf("expected 9 LLM calls, got %d", provider.CallCount())
	}
}

func TestBatchRunnerConcurrency(t *testing.T) {
	provider := llm.NewMock("concurrent")
	runner := NewRunner(provider)
	batch := NewBatchRunner(runner)

	datasets := make([]*models.TestDataset, 10)
	for i := range datasets {
		datasets[i] = &models.TestDataset{
			ID:    "ds-" + string(rune('a'+i)),
			Cases: []models.TestCase{{ID: "tc-" + string(rune('a'+i)), ExpectedContains: []string{"concurrent"}}},
		}
	}

	report, err := batch.RunBatch(context.Background(), &BatchConfig{
		PromptHash:  "conc-hash",
		PromptText:  "test",
		Datasets:    datasets,
		Model:       "m",
		Concurrency: 3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Results) != 10 {
		t.Fatalf("expected 10 results, got %d", len(report.Results))
	}
}

func TestMultiModelComparison(t *testing.T) {
	provider := llm.NewMock("multi")
	runner := NewRunner(provider)
	batch := NewBatchRunner(runner)

	dataset := &models.TestDataset{
		ID: "ds-multi",
		Cases: []models.TestCase{
			{ID: "tc-1", ExpectedContains: []string{"multi"}},
		},
	}

	report, err := batch.RunMultiModel(context.Background(), "hash", "test prompt", dataset, []string{"gpt-4", "gpt-3.5"}, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Reports) != 2 {
		t.Fatalf("expected 2 model reports, got %d", len(report.Reports))
	}
	if _, ok := report.Reports["gpt-4"]; !ok {
		t.Fatal("expected report for gpt-4")
	}
	if _, ok := report.Reports["gpt-3.5"]; !ok {
		t.Fatal("expected report for gpt-3.5")
	}
}

func TestScorers(t *testing.T) {
	tc := &models.TestCase{
		ExpectedOutput:   "hello",
		ExpectedContains: []string{"ell", "hel"},
	}

	// Exact match
	em := ExactMatchScorer{}
	if em.Score("hello", tc) != 1.0 {
		t.Fatal("expected 1.0 for exact match")
	}
	if em.Score("world", tc) != 0.0 {
		t.Fatal("expected 0.0 for non-match")
	}

	// Contains
	c := ContainsScorer{}
	if c.Score("hello world", tc) != 1.0 {
		t.Fatal("expected 1.0 for contains match")
	}
	if c.Score("xyz", tc) != 0.0 {
		t.Fatal("expected 0.0 for no match")
	}

	// PassThru
	pt := PassThruScorer{}
	if pt.Score("anything", tc) != 1.0 {
		t.Fatal("expected 1.0 for pass thru")
	}

	// Composite
	comp := CompositeScorer{
		Scorers: []WeightedScorer{
			{Scorer: ExactMatchScorer{}, Weight: 1},
			{Scorer: ContainsScorer{}, Weight: 1},
		},
	}
	score := comp.Score("hello", tc)
	if score != 1.0 {
		t.Fatalf("expected 1.0 for composite, got %f", score)
	}
}

func TestCompareReports(t *testing.T) {
	a := &models.EvalReport{
		Aggregate: models.Aggregate{PassRate: 0.5, AvgScore: 0.6, AvgLatencyMs: 100},
		Results: []*models.EvalResult{
			{TestCaseID: "tc-1", Score: 0.5, Passed: true},
			{TestCaseID: "tc-2", Score: 0.3, Passed: false},
		},
	}
	b := &models.EvalReport{
		Aggregate: models.Aggregate{PassRate: 1.0, AvgScore: 0.9, AvgLatencyMs: 80},
		Results: []*models.EvalResult{
			{TestCaseID: "tc-1", Score: 0.9, Passed: true},
			{TestCaseID: "tc-2", Score: 0.8, Passed: true},
			{TestCaseID: "tc-3", Score: 1.0, Passed: true},
		},
	}

	comp := CompareReports(a, b)
	if comp.Diff.PassRateDelta != 0.5 {
		t.Fatalf("expected pass rate delta 0.5, got %f", comp.Diff.PassRateDelta)
	}
	if len(comp.Diff.TestCaseDiffs) != 3 {
		t.Fatalf("expected 3 test case diffs, got %d", len(comp.Diff.TestCaseDiffs))
	}
}

func TestHallucinationExtraction(t *testing.T) {
	refs := extractReferences("This is [1] a test [2] with refs [abc]")
	if len(refs) != 3 {
		t.Fatalf("expected 3 refs, got %d: %v", len(refs), refs)
	}
	if refs[0] != "1" || refs[1] != "2" || refs[2] != "abc" {
		t.Fatalf("unexpected refs: %v", refs)
	}
}
