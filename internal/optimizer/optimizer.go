// Package optimizer provides prompt optimization and recommendations.
package optimizer

import (
	"time"

	"github.com/sachncs/promptsheon/internal/llm"
)

// OptimizationSuggestion represents a single prompt optimization suggestion.
type OptimizationSuggestion struct {
	ID          string    `json:"id"`
	PromptID    string    `json:"prompt_id"`
	Type        string    `json:"type"`
	Severity    string    `json:"severity"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Original    string    `json:"original,omitempty"`
	Suggested   string    `json:"suggested,omitempty"`
	Impact      string    `json:"impact"`
	CreatedAt   time.Time `json:"created_at"`
}

// OptimizationReport contains the full results of a prompt optimization.
type OptimizationReport struct {
	PromptID      string                    `json:"prompt_id"`
	PromptName    string                    `json:"prompt_name"`
	Score         float64                   `json:"score"`
	Suggestions   []*OptimizationSuggestion `json:"suggestions"`
	Metrics       *PromptMetrics            `json:"metrics"`
	OptimizedText string                    `json:"optimized_text,omitempty"`
	CreatedAt     time.Time                 `json:"created_at"`
}

// PromptMetrics contains computed metrics for a prompt.
type PromptMetrics struct {
	WordCount       int     `json:"word_count"`
	CharCount       int     `json:"char_count"`
	EstimatedTokens int     `json:"estimated_tokens"`
	EstimatedCost   float64 `json:"estimated_cost_usd"`
	ComplexityScore float64 `json:"complexity_score"`
	ClarityScore    float64 `json:"clarity_score"`
	VariableCount   int     `json:"variable_count"`
	HasSystemPrompt bool    `json:"has_system_prompt"`
}

// Optimizer provides prompt optimization and analysis.
type Optimizer struct {
	provider llm.Provider
}

// NewOptimizer creates a new Optimizer with the given provider.
func NewOptimizer(provider llm.Provider) *Optimizer {
	return &Optimizer{
		provider: provider,
	}
}

// GetOptimizationTips returns general tips for writing effective prompts.
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
