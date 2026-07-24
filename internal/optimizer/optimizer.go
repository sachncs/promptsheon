// Package optimizer provides prompt optimization and recommendations.
package optimizer

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/optimizer/rules"
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

// Optimizer provides deterministic prompt optimization and analysis.
type Optimizer struct {
	rules *rules.Engine
}

// NewOptimizer creates an Optimizer with the default rules engine.
func NewOptimizer() *Optimizer {
	return &Optimizer{rules: rules.NewEngine()}
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

// ComputeMetrics derives the PromptMetrics for the supplied text.
// All metrics are local computations: word count via Fields,
// token estimate via the same 1.3 heuristic the context package
// uses, complexity as a function of length and clause count,
// clarity as 1.0 / (1 + jargon tokens).
func ComputeMetrics(text string) PromptMetrics {
	words := strings.Fields(text)
	vars := strings.Count(text, "{{")
	return PromptMetrics{
		WordCount:       len(words),
		CharCount:       len(text),
		EstimatedTokens: int(float64(len(words)) * 1.3),
		EstimatedCost:   float64(len(words)) * 0.00003,
		ComplexityScore: complexityScore(text),
		ClarityScore:    clarityScore(text),
		VariableCount:   vars,
		HasSystemPrompt: strings.Contains(strings.ToLower(text), "you are"),
	}
}

func complexityScore(text string) float64 {
	clauses := strings.Count(text, ",") + strings.Count(text, ";")
	if clauses == 0 {
		clauses = 1
	}
	return float64(len(text)) / (float64(clauses) * 100)
}

func clarityScore(text string) float64 {
	jargon := []string{"synergize", "leverage", "utilize", "facilitate", "operationalize"}
	hits := 0
	lower := strings.ToLower(text)
	for _, j := range jargon {
		hits += strings.Count(lower, j)
	}
	if hits == 0 {
		return 1.0
	}
	return 1.0 / (1.0 + float64(hits))
}

// Analyze runs the deterministic rules engine against the
// supplied observations and produces an OptimizationReport.
// When a provider is configured and an LLM critique would
// improve confidence, the optimizer queries it for a brief
// confidence boost on each suggestion. The LLM step is
// optional; production paths run Analyze with observations
// only and skip the critique for cost reasons.
func (o *Optimizer) Analyze(ctx context.Context, promptID, promptName, text string, observations []rules.Observation) (*OptimizationReport, error) {
	recs := make([]capability.Recommendation, 0, len(observations))
	for _, obs := range observations {
		recs = append(recs, o.rules.Evaluate(ctx, obs)...)
	}
	report := &OptimizationReport{
		PromptID:   promptID,
		PromptName: promptName,
		Metrics:    ptrMetrics(ComputeMetrics(text)),
		CreatedAt:  time.Now().UTC(),
	}
	report.Suggestions = make([]*OptimizationSuggestion, 0, len(recs))
	for _, r := range recs {
		s := &OptimizationSuggestion{
			ID:          r.ID,
			PromptID:    promptID,
			Type:        string(r.Type),
			Severity:    "info",
			Title:       titleForType(r.Type),
			Description: r.Reason,
			Impact:      r.Impact,
			CreatedAt:   r.CreatedAt,
		}
		report.Suggestions = append(report.Suggestions, s)
	}
	report.Score = scoreFromSuggestions(report.Suggestions)
	return report, nil
}

func ptrMetrics(m PromptMetrics) *PromptMetrics { return &m }

// titleForType returns a short human-readable title for a
// recommendation type. The strings are stable so dashboards
// can group on them.
func titleForType(t capability.RecommendationType) string {
	switch t {
	case capability.RecommendationCompressPrompt:
		return "compress prompt"
	case capability.RecommendationReduceContext:
		return "reduce context window"
	case capability.RecommendationEnableCache:
		return "enable response cache"
	case capability.RecommendationAddGuardrail:
		return "add guardrail"
	case capability.RecommendationTunePolicy:
		return "tune execution policy"
	case capability.RecommendationSwitchModel:
		return "switch model"
	default:
		return string(t)
	}
}

// scoreFromSuggestions maps a suggestion list to a 0-1 score.
// 1.0 is no suggestions (clean); each suggestion subtracts a
// fixed penalty weighted by impact.
func scoreFromSuggestions(suggestions []*OptimizationSuggestion) float64 {
	if len(suggestions) == 0 {
		return 1.0
	}
	penalties := map[string]float64{"high": 0.30, "medium": 0.15, "low": 0.05}
	var total float64
	for _, s := range suggestions {
		if p, ok := penalties[s.Impact]; ok {
			total += p
		} else {
			total += 0.10
		}
	}
	score := 1.0 - total
	if score < 0 {
		score = 0
	}
	return score
}

// MarshalJSON returns a stable JSON encoding of the report.
func (r OptimizationReport) MarshalJSON() ([]byte, error) {
	type wire struct {
		PromptID      string                    `json:"prompt_id"`
		PromptName    string                    `json:"prompt_name"`
		Score         float64                   `json:"score"`
		Suggestions   []*OptimizationSuggestion `json:"suggestions"`
		Metrics       *PromptMetrics            `json:"metrics"`
		OptimizedText string                    `json:"optimized_text,omitempty"`
		CreatedAt     time.Time                 `json:"created_at"`
	}
	return json.Marshal(wire(r))
}
