// Package playground provides an interactive prompt testing environment.
package playground

import (
	"context"
	"fmt"
	"time"

	"github.com/sachncs/promptsheon/internal/llm"
)

// Template is a pre-built prompt template for the playground.
type Template struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	Content     string            `json:"content"`
	Variables   []string          `json:"variables"`
	Examples    map[string]string `json:"examples,omitempty"`
}

// RunRequest represents a request to run a prompt in the playground.
type RunRequest struct {
	PromptID    string            `json:"prompt_id,omitempty"`
	Content     string            `json:"content"`
	Variables   map[string]string `json:"variables"`
	Model       string            `json:"model"`
	SystemMsg   string            `json:"system_message,omitempty"`
	MaxTokens   int               `json:"max_tokens"`
	Temperature float64           `json:"temperature"`
}

// RunResponse represents the result of a playground run.
type RunResponse struct {
	Content   string    `json:"content"`
	Model     string    `json:"model"`
	Tokens    int       `json:"tokens"`
	LatencyMs int64     `json:"latency_ms"`
	Cost      float64   `json:"cost_usd"`
	Timestamp time.Time `json:"timestamp"`
}

// CompareRequest represents a request to compare multiple prompts.
type CompareRequest struct {
	Prompts   []ComparePrompt   `json:"prompts"`
	Model     string            `json:"model"`
	Variables map[string]string `json:"variables"`
}

// ComparePrompt is a prompt to compare.
type ComparePrompt struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

// CompareResult contains results for a single prompt comparison.
type CompareResult struct {
	Name     string       `json:"name"`
	Response *RunResponse `json:"response"`
	Rank     int          `json:"rank"`
}

// Playground provides prompt testing capabilities.
type Playground struct {
	templates []*Template
}

// NewPlayground creates a new playground instance.
func NewPlayground() *Playground {
	return &Playground{
		templates: DefaultTemplates(),
	}
}

// Run executes a prompt in the playground.
func (p *Playground) Run(ctx context.Context, provider llm.Provider, req *RunRequest) (*RunResponse, error) {
	if req.PromptID == "" && req.Content == "" {
		return nil, fmt.Errorf("prompt content or ID required")
	}

	content := req.Content

	// Replace variables
	for k, v := range req.Variables {
		placeholder := fmt.Sprintf("{{%s}}", k)
		content = replaceAll(content, placeholder, v)
	}

	// Build messages
	messages := []llm.Message{}
	if req.SystemMsg != "" {
		messages = append(messages, llm.Message{Role: "system", Content: req.SystemMsg})
	}
	messages = append(messages, llm.Message{Role: "user", Content: content})

	// Set defaults
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 500
	}
	temp := req.Temperature
	if temp <= 0 {
		temp = 0.7
	}
	model := req.Model
	if model == "" {
		model = "gpt-4"
	}

	start := time.Now()

	resp, err := provider.Complete(ctx, &llm.Request{
		Model:       model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temp,
	})
	if err != nil {
		return nil, err
	}

	latency := time.Since(start).Milliseconds()

	return &RunResponse{
		Content:   resp.Content,
		Model:     model,
		Tokens:    resp.Usage.TotalTokens,
		LatencyMs: latency,
		Cost:      calculateCost(model, resp.Usage.TotalTokens),
		Timestamp: time.Now(),
	}, nil
}

// Compare runs multiple prompts and compares results.
func (p *Playground) Compare(ctx context.Context, provider llm.Provider, req *CompareRequest) ([]*CompareResult, error) {
	results := make([]*CompareResult, len(req.Prompts))

	for i, prompt := range req.Prompts {
		runReq := &RunRequest{
			Content:   prompt.Content,
			Model:     req.Model,
			Variables: req.Variables,
		}

		resp, err := p.Run(ctx, provider, runReq)
		if err != nil {
			return nil, err
		}

		results[i] = &CompareResult{
			Name:     prompt.Name,
			Response: resp,
			Rank:     i + 1,
		}
	}

	return results, nil
}

// GetTemplates returns available playground templates.
func (p *Playground) GetTemplates() []*Template {
	return p.templates
}

// GetTemplatesByCategory returns templates filtered by category.
func (p *Playground) GetTemplatesByCategory(category string) []*Template {
	var filtered []*Template
	for _, t := range p.templates {
		if t.Category == category {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func calculateCost(model string, tokens int) float64 {
	// Simplified cost calculation
	switch model {
	case "gpt-4":
		return float64(tokens) * 0.00003
	case "gpt-4-turbo":
		return float64(tokens) * 0.00001
	case "gpt-3.5-turbo":
		return float64(tokens) * 0.000002
	default:
		return float64(tokens) * 0.00001
	}
}

func replaceAll(s, old, newStr string) string {
	result := s
	for {
		idx := indexOf(result, old)
		if idx == -1 {
			return result
		}
		result = result[:idx] + newStr + result[idx+len(old):]
	}
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// DefaultTemplates returns a set of default prompt templates.
func DefaultTemplates() []*Template {
	return []*Template{
		{
			ID:          "summarizer",
			Name:        "Text Summarizer",
			Description: "Summarizes text into key points",
			Category:    "utility",
			Content:     "Summarize the following text into 3-5 key points:\n\n{{text}}",
			Variables:   []string{"text"},
		},
		{
			ID:          "translator",
			Name:        "Translator",
			Description: "Translates text to another language",
			Category:    "utility",
			Content:     "Translate the following text to {{language}}:\n\n{{text}}",
			Variables:   []string{"text", "language"},
		},
		{
			ID:          "code-reviewer",
			Name:        "Code Reviewer",
			Description: "Reviews code and suggests improvements",
			Category:    "development",
			Content:     "Review the following {{language}} code and suggest improvements:\n\n```{{language}}\n{{code}}\n```",
			Variables:   []string{"language", "code"},
		},
		{
			ID:          "email-writer",
			Name:        "Email Writer",
			Description: "Writes professional emails",
			Category:    "communication",
			Content:     "Write a professional email with the following details:\n\nTo: {{to}}\nSubject: {{subject}}\nKey points: {{points}}",
			Variables:   []string{"to", "subject", "points"},
		},
		{
			ID:          "brainstorm",
			Name:        "Brainstorm",
			Description: "Generates ideas for a given topic",
			Category:    "creative",
			Content:     "Generate 10 creative ideas for: {{topic}}\n\nContext: {{context}}",
			Variables:   []string{"topic", "context"},
		},
	}
}
