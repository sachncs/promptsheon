// Package eval provides the evaluation engine for running prompts against
// test datasets, scoring outputs, and detecting hallucinations.
package eval

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sachn-cs/promptsheon/internal/llm"
	"github.com/sachn-cs/promptsheon/internal/models"
)

// Runner executes evaluations by calling an LLM provider for each test case.
type Runner struct {
	provider llm.Provider
	scorers  []Scorer
	halluc   *HallucinationDetector
}

// NewRunner creates an eval runner with the given provider and scorers.
func NewRunner(provider llm.Provider, scorers ...Scorer) *Runner {
	return &Runner{
		provider: provider,
		scorers:  scorers,
		halluc:   NewHallucinationDetector(),
	}
}

// RunConfig holds all inputs for a single evaluation run.
type RunConfig struct {
	PromptHash string
	PromptText string // the actual prompt template text
	Dataset    *models.TestDataset
	Model      string
	MaxTokens  int
}

// Run executes the evaluation and returns a full report.
func (r *Runner) Run(ctx context.Context, cfg *RunConfig) (*models.EvalReport, error) {
	if cfg.PromptText == "" {
		return nil, fmt.Errorf("prompt text is required")
	}
	if cfg.Dataset == nil || len(cfg.Dataset.Cases) == 0 {
		return nil, fmt.Errorf("dataset with at least one test case is required")
	}

	startedAt := time.Now()
	results := make([]*models.EvalResult, 0, len(cfg.Dataset.Cases))

	for _, tc := range cfg.Dataset.Cases {
		result, err := r.runCase(ctx, cfg, &tc)
		if err != nil {
			result = &models.EvalResult{
				ID:         generateID(),
				TestCaseID: tc.ID,
				PromptHash: cfg.PromptHash,
				DatasetID:  cfg.Dataset.ID,
				Model:      cfg.Model,
				Error:      err.Error(),
				CreatedAt:  time.Now(),
			}
		}
		results = append(results, result)
	}

	report := &models.EvalReport{
		PromptHash:  cfg.PromptHash,
		DatasetID:   cfg.Dataset.ID,
		Model:       cfg.Model,
		Results:     results,
		Aggregate:   aggregate(results),
		StartedAt:   startedAt,
		CompletedAt: time.Now(),
	}
	return report, nil
}

func (r *Runner) runCase(ctx context.Context, cfg *RunConfig, tc *models.TestCase) (*models.EvalResult, error) {
	start := time.Now()

	// Build the prompt by substituting variables from test case input.
	prompt := r.buildPrompt(cfg.PromptText, tc)

	// Call LLM
	resp, err := r.provider.Complete(ctx, &llm.Request{
		Model: cfg.Model,
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
		MaxTokens: cfg.MaxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("llm call failed: %w", err)
	}

	output := resp.Content
	latency := time.Since(start)

	// Score the output
	score := r.scoreOutput(output, tc)

	// Detect hallucination
	hallScore := r.halluc.Score(ctx, r.provider, cfg.PromptText, output, tc)

	// Determine pass/fail
	passed := score >= 0.5 // default threshold

	result := &models.EvalResult{
		ID:                 generateID(),
		TestCaseID:         tc.ID,
		PromptHash:         cfg.PromptHash,
		DatasetID:          cfg.Dataset.ID,
		Model:              cfg.Model,
		Output:             output,
		Score:              score,
		LatencyMs:          latency.Milliseconds(),
		TokenUsage:         resp.Usage,
		HallucinationScore: hallScore,
		Passed:             passed,
		CreatedAt:          time.Now(),
	}
	return result, nil
}

func (r *Runner) buildPrompt(template string, tc *models.TestCase) string {
	prompt := template
	for k, v := range tc.Input {
		placeholder := fmt.Sprintf("{{%s}}", k)
		val := fmt.Sprintf("%v", v)
		prompt = strings.ReplaceAll(prompt, placeholder, val)
	}
	return prompt
}

func (r *Runner) scoreOutput(output string, tc *models.TestCase) float64 {
	if len(r.scorers) == 0 {
		return defaultScore(output, tc)
	}
	total := 0.0
	for _, s := range r.scorers {
		total += s.Score(output, tc)
	}
	return total / float64(len(r.scorers))
}

func defaultScore(output string, tc *models.TestCase) float64 {
	if len(tc.ExpectedContains) == 0 && tc.ExpectedOutput == "" {
		return 1.0 // no expectations = pass
	}
	score := 0.0
	checks := 0

	if tc.ExpectedOutput != "" {
		checks++
		if strings.EqualFold(strings.TrimSpace(output), strings.TrimSpace(tc.ExpectedOutput)) {
			score++
		}
	}

	for _, expected := range tc.ExpectedContains {
		checks++
		if strings.Contains(strings.ToLower(output), strings.ToLower(expected)) {
			score++
		}
	}

	if checks == 0 {
		return 1.0
	}
	return score / float64(checks)
}

// aggregate computes the Aggregate stats for a set of results.
func aggregate(results []*models.EvalResult) models.Aggregate {
	if len(results) == 0 {
		return models.Aggregate{}
	}

	var totalScore, totalHallucination, totalLatency float64
	var totalTokens, passedCount int

	for _, r := range results {
		totalScore += r.Score
		totalHallucination += r.HallucinationScore
		totalLatency += float64(r.LatencyMs)
		totalTokens += r.TokenUsage.TotalTokens
		if r.Passed {
			passedCount++
		}
	}

	n := float64(len(results))
	return models.Aggregate{
		TotalCases:       len(results),
		PassedCases:      passedCount,
		PassRate:         safeDivide(float64(passedCount), n),
		AvgScore:         safeDivide(totalScore, n),
		AvgLatencyMs:     safeDivide(totalLatency, n),
		AvgHallucination: safeDivide(totalHallucination, n),
		TotalTokens:      totalTokens,
	}
}

func safeDivide(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func generateID() string {
	return fmt.Sprintf("eval-%d", time.Now().UnixNano())
}
