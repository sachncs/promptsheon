package llm

// ModelPricing defines per-token costs for a model.
type ModelPricing struct {
	Name               string
	PromptPerToken     float64 // cost per prompt token in dollars
	CompletionPerToken float64 // cost per completion token in dollars
}

const modelGPT4o = "gpt-4o"
const modelGPT4oMini = "gpt-4o-mini"
const modelGPT4 = "gpt-4"
const modelLlama3 = "llama3"

// Pricing table for known models. Prices per token (USD).
var pricingTable = map[string]ModelPricing{
	// OpenAI
	modelGPT4o:      {Name: modelGPT4o, PromptPerToken: 2.50 / 1e6, CompletionPerToken: 10.00 / 1e6},
	modelGPT4oMini:  {Name: modelGPT4oMini, PromptPerToken: 0.15 / 1e6, CompletionPerToken: 0.60 / 1e6},
	"gpt-4-turbo":   {Name: "gpt-4-turbo", PromptPerToken: 10.00 / 1e6, CompletionPerToken: 30.00 / 1e6},
	modelGPT4:       {Name: modelGPT4, PromptPerToken: 30.00 / 1e6, CompletionPerToken: 60.00 / 1e6},
	"gpt-3.5-turbo": {Name: "gpt-3.5-turbo", PromptPerToken: 0.50 / 1e6, CompletionPerToken: 1.50 / 1e6},

	// Anthropic
	"claude-sonnet-4-20250514":   {Name: "claude-sonnet-4-20250514", PromptPerToken: 3.00 / 1e6, CompletionPerToken: 15.00 / 1e6},
	"claude-3-5-sonnet-20241022": {Name: "claude-3-5-sonnet-20241022", PromptPerToken: 3.00 / 1e6, CompletionPerToken: 15.00 / 1e6},
	"claude-3-5-haiku-20241022":  {Name: "claude-3-5-haiku-20241022", PromptPerToken: 0.80 / 1e6, CompletionPerToken: 4.00 / 1e6},
	"claude-3-opus-20240229":     {Name: "claude-3-opus-20240229", PromptPerToken: 15.00 / 1e6, CompletionPerToken: 75.00 / 1e6},

	// Ollama (local — zero cost)
	modelLlama3: {Name: modelLlama3, PromptPerToken: 0, CompletionPerToken: 0},
	"llama3.1":  {Name: "llama3.1", PromptPerToken: 0, CompletionPerToken: 0},
	"mistral":   {Name: "mistral", PromptPerToken: 0, CompletionPerToken: 0},
	"codellama": {Name: "codellama", PromptPerToken: 0, CompletionPerToken: 0},
}

// CalculateCost returns the cost in USD for the given token usage.
func CalculateCost(model string, usage Usage) float64 {
	pricing, ok := pricingTable[model]
	if !ok {
		return 0
	}
	return float64(usage.PromptTokens)*pricing.PromptPerToken +
		float64(usage.CompletionTokens)*pricing.CompletionPerToken
}

// GetPricing returns the pricing for a model, or nil if unknown.
func GetPricing(model string) *ModelPricing {
	p, ok := pricingTable[model]
	if !ok {
		return nil
	}
	return &p
}
