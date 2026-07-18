package llm

// ModelPricing defines per-token costs for a model.
type ModelPricing struct {
	Name               string
	PromptPerToken     float64 // cost per prompt token in dollars
	CompletionPerToken float64 // cost per completion token in dollars
}

const (
	modelGPT4o      = "gpt-4o"
	modelGPT4oMini  = "gpt-4o-mini"
	modelGPT4Turbo  = "gpt-4-turbo"
	modelGPT4       = "gpt-4"
	modelGPT35Turbo = "gpt-3.5-turbo"
)

// PricingTable holds per-token USD pricing for known models.
//
// PricingTable is an explicit value owned by the caller; it is not a
// package-level singleton. Construct one with NewPricingTable and pass
// it to whichever components need cost computation (e.g. an
// Instrumented provider wrapper, a metrics aggregator, a CLI driver).
// See ADR-0012 for the rationale.
type PricingTable struct {
	pricing map[string]ModelPricing
}

// NewPricingTable constructs a PricingTable pre-populated with the
// built-in pricing for known models. The returned value is safe for
// concurrent use without further synchronisation.
//
// Callers may register additional entries via Register, including
// overrides for built-in models if pricing data is supplied from
// configuration or a plugin.
func NewPricingTable() *PricingTable {
	pt := &PricingTable{pricing: make(map[string]ModelPricing)}
	// OpenAI
	pt.pricing[modelGPT4o] = ModelPricing{Name: modelGPT4o, PromptPerToken: 2.50 / 1e6, CompletionPerToken: 10.00 / 1e6}
	pt.pricing[modelGPT4oMini] = ModelPricing{Name: modelGPT4oMini, PromptPerToken: 0.15 / 1e6, CompletionPerToken: 0.60 / 1e6}
	pt.pricing[modelGPT4Turbo] = ModelPricing{Name: modelGPT4Turbo, PromptPerToken: 10.00 / 1e6, CompletionPerToken: 30.00 / 1e6}
	pt.pricing[modelGPT4] = ModelPricing{Name: modelGPT4, PromptPerToken: 30.00 / 1e6, CompletionPerToken: 60.00 / 1e6}
	pt.pricing[modelGPT35Turbo] = ModelPricing{Name: modelGPT35Turbo, PromptPerToken: 0.50 / 1e6, CompletionPerToken: 1.50 / 1e6}

	// Anthropic
	pt.pricing[claudeSonnet4] = ModelPricing{Name: claudeSonnet4, PromptPerToken: 3.00 / 1e6, CompletionPerToken: 15.00 / 1e6}
	pt.pricing[claude35Sonnet] = ModelPricing{Name: claude35Sonnet, PromptPerToken: 3.00 / 1e6, CompletionPerToken: 15.00 / 1e6}
	pt.pricing[claude35Haiku] = ModelPricing{Name: claude35Haiku, PromptPerToken: 0.80 / 1e6, CompletionPerToken: 4.00 / 1e6}
	pt.pricing[claude3Opus] = ModelPricing{Name: claude3Opus, PromptPerToken: 15.00 / 1e6, CompletionPerToken: 75.00 / 1e6}

	return pt
}

// Model identifiers (kept as package-level constants so test files
// outside this package do not need to reproduce them).
const (
	claudeSonnet4  = "claude-sonnet-4-20250514"
	claude35Sonnet = "claude-3-5-sonnet-20241022"
	claude35Haiku  = "claude-3-5-haiku-20241022"
	claude3Opus    = "claude-3-opus-20240229"
)

// Register adds or overrides a pricing entry.
func (p *PricingTable) Register(pricing ModelPricing) {
	if pricing.Name == "" {
		return
	}
	p.pricing[pricing.Name] = pricing
}

// Calculate returns the cost in USD for the given token usage on the
// named model. When the model is unknown, Calculate returns 0; callers
// may distinguish "free model" from "unknown" by Lookup.
func (p *PricingTable) Calculate(model string, usage Usage) float64 {
	pricing, ok := p.pricing[model]
	if !ok {
		return 0
	}
	return float64(usage.PromptTokens)*pricing.PromptPerToken +
		float64(usage.CompletionTokens)*pricing.CompletionPerToken
}

// Lookup returns a copy of the pricing for the model, or nil when the
// model is unknown.
func (p *PricingTable) Lookup(model string) *ModelPricing {
	v, ok := p.pricing[model]
	if !ok {
		return nil
	}
	return &v
}
