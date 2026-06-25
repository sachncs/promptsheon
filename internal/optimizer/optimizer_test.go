package optimizer_test

import (
	"context"
	"testing"

	"github.com/sachn-cs/promptsheon/internal/llm"
	"github.com/sachn-cs/promptsheon/internal/models"
	"github.com/sachn-cs/promptsheon/internal/optimizer"
)

func TestAnalyzePrompt(t *testing.T) {
	opt := optimizer.NewOptimizer(nil)

	tests := []struct {
		name    string
		prompt  *models.Prompt
		wantMin float64
		wantMax float64
	}{
		{
			name: "simple prompt",
			prompt: &models.Prompt{
				Content: "Hello world",
			},
			wantMin: 0,
			wantMax: 1,
		},
		{
			name: "complex prompt with variables",
			prompt: &models.Prompt{
				Content:   "Summarize the following text: {{text}}. Make it concise and focus on key points.",
				Variables: []models.Variable{{Name: "text"}},
			},
			wantMin: 0,
			wantMax: 1,
		},
		{
			name: "long prompt",
			prompt: &models.Prompt{
				Content: "This is a very long prompt that exceeds many words. " +
					"Lorem ipsum dolor sit amet, consectetur adipiscing elit. " +
					"Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.",
			},
			wantMin: 0,
			wantMax: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := opt.AnalyzePrompt(tt.prompt)
			if metrics == nil {
				t.Fatal("expected metrics, got nil")
			}
			if metrics.WordCount == 0 {
				t.Error("expected non-zero word count")
			}
			if metrics.CharCount == 0 {
				t.Error("expected non-zero char count")
			}
			if metrics.EstimatedTokens == 0 {
				t.Error("expected non-zero estimated tokens")
			}
			if metrics.ComplexityScore < tt.wantMin || metrics.ComplexityScore > tt.wantMax {
				t.Errorf("complexity score %f out of range [%f, %f]", metrics.ComplexityScore, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestOptimizationTips(t *testing.T) {
	tips := optimizer.GetOptimizationTips()
	if len(tips) == 0 {
		t.Error("expected at least one tip")
	}
	for _, tip := range tips {
		if tip == "" {
			t.Error("expected non-empty tip")
		}
	}
}

type mockProvider struct{}

func (m *mockProvider) Complete(ctx context.Context, req *llm.Request) (*llm.Response, error) {
	return &llm.Response{
		Content: `{"score": 85, "suggestions": [{"type": "clarity", "severity": "low", "title": "Test"}]}`,
	}, nil
}

func (m *mockProvider) Name() string { return "mock" }

func TestOptimizePrompt(t *testing.T) {
	provider := &mockProvider{}
	opt := optimizer.NewOptimizer(provider)

	prompt := &models.Prompt{
		ID:        "test-prompt",
		Name:      "Test Prompt",
		Content:   "Summarize this text: {{text}}",
		Variables: []models.Variable{{Name: "text"}},
	}

	report, err := opt.OptimizePrompt(context.Background(), prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report == nil {
		t.Fatal("expected report, got nil")
	}
	if report.PromptID != "test-prompt" {
		t.Errorf("expected prompt ID test-prompt, got %s", report.PromptID)
	}
	if report.PromptName != "Test Prompt" {
		t.Errorf("expected prompt name Test Prompt, got %s", report.PromptName)
	}
	if report.Metrics == nil {
		t.Error("expected metrics, got nil")
	}
	if report.Score < 0 || report.Score > 100 {
		t.Errorf("score %f out of range [0, 100]", report.Score)
	}
}

func TestBatchOptimize(t *testing.T) {
	provider := &mockProvider{}
	opt := optimizer.NewOptimizer(provider)

	prompts := []*models.Prompt{
		{ID: "p1", Name: "P1", Content: "first"},
		{ID: "p2", Name: "P2", Content: "second"},
		{ID: "p3", Name: "P3", Content: "third"},
	}
	reports, err := opt.BatchOptimize(context.Background(), prompts)
	if err != nil {
		t.Fatalf("BatchOptimize: %v", err)
	}
	if len(reports) != len(prompts) {
		t.Errorf("expected %d reports, got %d", len(prompts), len(reports))
	}
	for i, r := range reports {
		if r == nil {
			t.Errorf("report %d is nil", i)
			continue
		}
		if r.PromptID != prompts[i].ID {
			t.Errorf("report %d: expected id %q, got %q", i, prompts[i].ID, r.PromptID)
		}
	}
}
