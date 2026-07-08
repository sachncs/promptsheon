package capability

// ModelPolicy defines requirements and constraints for model selection.
//
// This is NOT "Use GPT-5.5". Instead, the policy expresses what the
// capability needs (reasoning, vision, JSON mode, latency, cost, etc.)
// and Promptsheon selects the best provider and model at runtime.
type ModelPolicy struct {
	Requirements      ModelRequirements `json:"requirements"`
	SelectionStrategy SelectionStrategy `json:"selection_strategy"`
	ProviderHints     []string          `json:"provider_hints,omitempty"`
}

// ModelRequirements describes what the capability needs from a model.
type ModelRequirements struct {
	NeedsReasoning bool    `json:"needs_reasoning"`
	NeedsVision    bool    `json:"needs_vision"`
	NeedsJSON      bool    `json:"needs_json"`
	MaxLatencyMs   int     `json:"max_latency_ms,omitempty"`
	MaxCostUSD     float64 `json:"max_cost_usd,omitempty"`
	NeedsStreaming bool    `json:"needs_streaming"`
	NeedsToolUse   bool    `json:"needs_tool_use"`
}

// SelectionStrategy determines how a model is chosen for execution.
type SelectionStrategy string

const (
	// SelectionStrategyCostOptimized selects the most cost-effective model.
	SelectionStrategyCostOptimized SelectionStrategy = "cost_optimized"
	// SelectionStrategyLatencyOptimized selects the lowest-latency model.
	SelectionStrategyLatencyOptimized SelectionStrategy = "latency_optimized"
	// SelectionStrategyQualityOptimized selects the highest-quality model.
	SelectionStrategyQualityOptimized SelectionStrategy = "quality_optimized"
	// SelectionStrategyManual uses explicitly specified model selection.
	SelectionStrategyManual SelectionStrategy = "manual"
)
