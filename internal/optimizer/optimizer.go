package optimizer

import (
	"strings"
	"time"

	"github.com/sachncs/promptsheon/internal/llm"
)

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

type OptimizationReport struct {
	PromptID      string                    `json:"prompt_id"`
	PromptName    string                    `json:"prompt_name"`
	Score         float64                   `json:"score"`
	Suggestions   []*OptimizationSuggestion `json:"suggestions"`
	Metrics       *PromptMetrics            `json:"metrics"`
	OptimizedText string                    `json:"optimized_text,omitempty"`
	CreatedAt     time.Time                 `json:"created_at"`
}

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

type Optimizer struct {
	provider llm.Provider
}

func NewOptimizer(provider llm.Provider) *Optimizer {
	return &Optimizer{
		provider: provider,
	}
}

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

func computePromptMetrics(content string, variables []string) *PromptMetrics {
	words := strings.Fields(content)

	estimatedTokens := len(content) / 4

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
	if len(variables) > 3 {
		complexity += 0.1
	}
	if complexity > 1 {
		complexity = 1
	}

	clarity := 1.0
	if len(words) > 200 {
		clarity -= 0.2
	}
	if strings.Count(content, "\n") < 3 && len(words) > 50 {
		clarity -= 0.2
	}
	if strings.Contains(content, "  ") {
		clarity -= 0.1
	}
	if clarity < 0 {
		clarity = 0
	}

	estimatedCost := float64(estimatedTokens) * 0.00003

	return &PromptMetrics{
		WordCount:       len(words),
		CharCount:       len(content),
		EstimatedTokens: estimatedTokens,
		EstimatedCost:   estimatedCost,
		ComplexityScore: complexity,
		ClarityScore:    clarity,
		VariableCount:   len(variables),
		HasSystemPrompt: strings.Contains(strings.ToLower(content), "system") || strings.Contains(content, "{{system}}"),
	}
}
