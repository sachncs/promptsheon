package playground_test

import (
	"context"
	"testing"

	"promptsheon/internal/llm"
	"promptsheon/internal/models"
	"promptsheon/internal/playground"
)

type mockProvider struct{}

func (m *mockProvider) Complete(ctx context.Context, req *llm.Request) (*llm.Response, error) {
	return &llm.Response{
		Content: "Mock response for: " + req.Messages[len(req.Messages)-1].Content,
		Model:   req.Model,
		Usage: models.Usage{
			TotalTokens: 50,
		},
	}, nil
}

func (m *mockProvider) Name() string {
	return "mock"
}

func TestRun(t *testing.T) {
	pg := playground.NewPlayground()
	provider := &mockProvider{}

	req := &playground.RunRequest{
		Content:   "Hello {{name}}",
		Variables: map[string]string{"name": "World"},
		Model:     "gpt-4",
		MaxTokens: 100,
	}

	resp, err := pg.Run(context.Background(), provider, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content == "" {
		t.Error("expected non-empty content")
	}
	if resp.Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", resp.Model)
	}
	if resp.Tokens != 50 {
		t.Errorf("expected 50 tokens, got %d", resp.Tokens)
	}
}

func TestRunEmptyContent(t *testing.T) {
	pg := playground.NewPlayground()
	provider := &mockProvider{}

	req := &playground.RunRequest{
		Content: "",
	}

	_, err := pg.Run(context.Background(), provider, req)
	if err == nil {
		t.Error("expected error for empty content")
	}
}

func TestCompare(t *testing.T) {
	pg := playground.NewPlayground()
	provider := &mockProvider{}

	req := &playground.CompareRequest{
		Prompts: []playground.ComparePrompt{
			{Name: "Prompt 1", Content: "Hello"},
			{Name: "Prompt 2", Content: "World"},
		},
		Model: "gpt-4",
	}

	results, err := pg.Compare(context.Background(), provider, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].Name != "Prompt 1" {
		t.Errorf("expected name Prompt 1, got %s", results[0].Name)
	}
}

func TestGetTemplates(t *testing.T) {
	pg := playground.NewPlayground()

	templates := pg.GetTemplates()
	if len(templates) == 0 {
		t.Error("expected at least one template")
	}

	// Check required fields
	for _, tmpl := range templates {
		if tmpl.ID == "" {
			t.Error("expected non-empty template ID")
		}
		if tmpl.Name == "" {
			t.Error("expected non-empty template name")
		}
		if tmpl.Category == "" {
			t.Error("expected non-empty template category")
		}
	}
}

func TestGetTemplatesByCategory(t *testing.T) {
	pg := playground.NewPlayground()

	utilityTemplates := pg.GetTemplatesByCategory("utility")
	if len(utilityTemplates) == 0 {
		t.Error("expected at least one utility template")
	}

	for _, tmpl := range utilityTemplates {
		if tmpl.Category != "utility" {
			t.Errorf("expected category utility, got %s", tmpl.Category)
		}
	}
}

func TestDefaultTemplates(t *testing.T) {
	templates := playground.DefaultTemplates()

	// Check we have templates for each category
	categories := map[string]bool{}
	for _, tmpl := range templates {
		categories[tmpl.Category] = true
	}

	expectedCategories := []string{"utility", "development", "communication", "creative"}
	for _, cat := range expectedCategories {
		if !categories[cat] {
			t.Errorf("missing template for category %s", cat)
		}
	}
}
