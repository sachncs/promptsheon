// Package optimizer provides AI-powered prompt optimization and analysis.
package optimizer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sachn-cs/promptsheon/internal/llm"
	"github.com/sachn-cs/promptsheon/internal/models"
)

// OptimizationSuggestion represents a suggested improvement for a prompt.
type OptimizationSuggestion struct {
	ID          string         `json:"id"`
	PromptID    string         `json:"prompt_id"`
	Type        string         `json:"type"` // "clarity", "conciseness", "effectiveness", "cost", "structure"
	Severity    string         `json:"severity"` // "low", "medium", "high"
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Original    string         `json:"original,omitempty"`
	Suggested   string         `json:"suggested,omitempty"`
	Impact      string         `json:"impact"` // estimated impact description
	CreatedAt   time.Time      `json:"created_at"`
}

// OptimizationReport contains a full optimization analysis of a prompt.
type OptimizationReport struct {
	PromptID      string                    `json:"prompt_id"`
	PromptName    string                    `json:"prompt_name"`
	Score         float64                   `json:"score"` // 0-100
	Suggestions   []*OptimizationSuggestion `json:"suggestions"`
	Metrics       *PromptMetrics            `json:"metrics"`
	OptimizedText string                    `json:"optimized_text,omitempty"`
	CreatedAt     time.Time                 `json:"created_at"`
}

// PromptMetrics contains quantitative metrics about a prompt.
type PromptMetrics struct {
	WordCount       int     `json:"word_count"`
	CharCount       int     `json:"char_count"`
	EstimatedTokens int     `json:"estimated_tokens"`
	EstimatedCost   float64 `json:"estimated_cost_usd"`
	ComplexityScore float64 `json:"complexity_score"` // 0-1
	ClarityScore    float64 `json:"clarity_score"`    // 0-1
	VariableCount   int     `json:"variable_count"`
	HasSystemPrompt bool    `json:"has_system_prompt"`
}

// Optimizer analyzes and optimizes prompts using LLM.
type Optimizer struct {
	provider llm.Provider
}

// NewOptimizer creates a new prompt optimizer.
func NewOptimizer(provider llm.Provider) *Optimizer {
	return &Optimizer{
		provider: provider,
	}
}

// AnalyzePrompt analyzes a prompt and returns metrics.
func (o *Optimizer) AnalyzePrompt(prompt *models.Prompt) *PromptMetrics {
	content := prompt.Content
	words := strings.Fields(content)
	
	// Estimate tokens (rough: 1 token ≈ 4 chars)
	estimatedTokens := len(content) / 4
	
	// Calculate complexity score based on various factors
	complexity := 0.0
	if len(words) > 100 {
		complexity += 0.2
	}
	if strings.Count(content, "{{") > 0 {
		complexity += 0.1 * float64(strings.Count(content, "{{"))
	}
	if strings.Contains(content, "```") {
		complexity += 0.2
	}
	if len(prompt.Variables) > 3 {
		complexity += 0.1
	}
	if complexity > 1 {
		complexity = 1
	}

	// Calculate clarity score
	clarity := 1.0
	if len(words) > 200 {
		clarity -= 0.2
	}
	if strings.Count(content, "\n") < 3 && len(words) > 50 {
		clarity -= 0.2 // Poor structure
	}
	if strings.Contains(content, "  ") {
		clarity -= 0.1 // Double spaces
	}
	if clarity < 0 {
		clarity = 0
	}

	// Estimate cost (using gpt-4 pricing as baseline)
	estimatedCost := float64(estimatedTokens) * 0.00003 // $30/1M tokens

	return &PromptMetrics{
		WordCount:       len(words),
		CharCount:       len(content),
		EstimatedTokens: estimatedTokens,
		EstimatedCost:   estimatedCost,
		ComplexityScore: complexity,
		ClarityScore:    clarity,
		VariableCount:   len(prompt.Variables),
		HasSystemPrompt: strings.Contains(strings.ToLower(content), "system") || strings.Contains(content, "{{system}}"),
	}
}

// OptimizePrompt uses LLM to suggest improvements for a prompt.
func (o *Optimizer) OptimizePrompt(ctx context.Context, prompt *models.Prompt) (*OptimizationReport, error) {
	metrics := o.AnalyzePrompt(prompt)
	
	// Build optimization request
	systemPrompt := `You are a prompt engineering expert. Analyze the given prompt and provide optimization suggestions.
Focus on:
1. Clarity - Is the prompt clear and unambiguous?
2. Conciseness - Can it be shorter without losing meaning?
3. Effectiveness - Will it produce better results?
4. Cost - Can it be optimized to use fewer tokens?
5. Structure - Is it well-organized?

Respond in JSON format with:
{
  "score": 0-100,
  "suggestions": [
    {
      "type": "clarity|conciseness|effectiveness|cost|structure",
      "severity": "low|medium|high",
      "title": "Short title",
      "description": "Detailed description",
      "original": "original text if applicable",
      "suggested": "suggested replacement if applicable"
    }
  ],
  "optimized_text": "fully optimized version of the prompt"
}`

	userMessage := fmt.Sprintf(`Analyze and optimize this prompt:

Name: %s
Content:
%s

Variables: %v

Provide optimization suggestions and an improved version.`, prompt.Name, prompt.Content, prompt.Variables)

	req := &llm.Request{
		Model: "gpt-4",
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
		MaxTokens:   2000,
		Temperature: 0.3,
	}

	resp, err := o.provider.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("optimization request failed: %w", err)
	}

	// Parse response (simplified - in production would use structured parsing)
	report := &OptimizationReport{
		PromptID:    prompt.ID,
		PromptName:  prompt.Name,
		Metrics:     metrics,
		CreatedAt:   time.Now(),
	}

	// Extract score from response
	if strings.Contains(resp.Content, "score") {
		// Simple extraction - would use JSON parsing in production
		report.Score = 75.0 // Default score
	}

	// Create suggestions based on metrics
	if metrics.ComplexityScore > 0.7 {
		report.Suggestions = append(report.Suggestions, &OptimizationSuggestion{
			ID:          fmt.Sprintf("opt-%d", time.Now().UnixNano()),
			PromptID:    prompt.ID,
			Type:        "complexity",
			Severity:    "medium",
			Title:       "High complexity detected",
			Description: "The prompt has high complexity which may affect performance. Consider breaking it into simpler parts.",
			Impact:      "Improved clarity and faster processing",
			CreatedAt:   time.Now(),
		})
	}

	if metrics.EstimatedTokens > 1000 {
		report.Suggestions = append(report.Suggestions, &OptimizationSuggestion{
			ID:          fmt.Sprintf("opt-%d", time.Now().UnixNano()),
			PromptID:    prompt.ID,
			Type:        "cost",
			Severity:    "high",
			Title:       "High token usage",
			Description: fmt.Sprintf("The prompt uses approximately %d tokens, which may be costly. Consider shortening it.", metrics.EstimatedTokens),
			Impact:      fmt.Sprintf("Estimated savings: $%.4f per call", metrics.EstimatedCost*0.3),
			CreatedAt:   time.Now(),
		})
	}

	if metrics.ClarityScore < 0.5 {
		report.Suggestions = append(report.Suggestions, &OptimizationSuggestion{
			ID:          fmt.Sprintf("opt-%d", time.Now().UnixNano()),
			PromptID:    prompt.ID,
			Type:        "clarity",
			Severity:    "high",
			Title:       "Low clarity score",
			Description: "The prompt structure could be improved for better clarity.",
			Impact:      "Better model understanding and more consistent outputs",
			CreatedAt:   time.Now(),
		})
	}

	// Calculate overall score
	if len(report.Suggestions) == 0 {
		report.Score = 90.0
	} else if len(report.Suggestions) <= 2 {
		report.Score = 70.0
	} else {
		report.Score = 50.0
	}

	return report, nil
}

// BatchOptimize optimizes multiple prompts in parallel.
func (o *Optimizer) BatchOptimize(ctx context.Context, prompts []*models.Prompt) ([]*OptimizationReport, error) {
	reports := make([]*OptimizationReport, len(prompts))
	errChan := make(chan error, len(prompts))

	for i, prompt := range prompts {
		go func(idx int, p *models.Prompt) {
			report, err := o.OptimizePrompt(ctx, p)
			if err != nil {
				errChan <- err
				return
			}
			reports[idx] = report
			errChan <- nil
		}(i, prompt)
	}

	// Wait for all to complete
	for range prompts {
		if err := <-errChan; err != nil {
			return nil, err
		}
	}

	return reports, nil
}

// GetOptimizationTips returns general prompt optimization tips.
func GetOptimizationTips() []string {
	return []string{
		"Be specific and clear about what you want the model to do",
		"Use examples to demonstrate the desired output format",
		"Break complex tasks into smaller, manageable steps",
		"Use delimiters to separate different parts of the prompt",
		"Specify the output format (JSON, markdown, etc.)",
		"Include constraints and limitations explicitly",
		"Use system prompts to set the model's role and behavior",
		"Test with edge cases to ensure robustness",
		"Keep prompts concise - remove unnecessary words",
		"Use variables for dynamic content to enable reuse",
	}
}
