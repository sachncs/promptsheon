package capability

import "time"

// RecommendationType identifies the kind of optimization suggested.
type RecommendationType string

const (
	RecommendationSwitchModel       RecommendationType = "switch_model"
	RecommendationCompressPrompt    RecommendationType = "compress_prompt"
	RecommendationReduceContext     RecommendationType = "reduce_context"
	RecommendationEnableCache       RecommendationType = "enable_cache"
	RecommendationDisableReasoning  RecommendationType = "disable_reasoning"
	RecommendationUpgradeMCP        RecommendationType = "upgrade_mcp"
	RecommendationRemoveTool        RecommendationType = "remove_tool"
	RecommendationSplitCapability   RecommendationType = "split_capability"
	RecommendationAddGuardrail      RecommendationType = "add_guardrail"
	RecommendationTunePolicy        RecommendationType = "tune_policy"
)

// Recommendation suggests an improvement to a capability version.
//
// This is one of the most valuable objects in the system — it closes the
// feedback loop by converting production telemetry and evaluation results
// into actionable suggestions that drive the next version.
type Recommendation struct {
	ID                   string             `json:"id"`
	CapabilityVersionID  string             `json:"capability_version_id"`
	Type                 RecommendationType  `json:"type"`
	Reason               string             `json:"reason"`
	Confidence           float64            `json:"confidence"`
	ExpectedSavingsUSD   float64            `json:"expected_savings_usd,omitempty"`
	ExpectedQualityDelta float64            `json:"expected_quality_delta,omitempty"`
	Impact               string             `json:"impact"` // "low", "medium", "high"
	AutoApplicable       bool               `json:"auto_applicable"`
	CreatedAt            time.Time          `json:"created_at"`
	AppliedAt            *time.Time         `json:"applied_at,omitempty"`
}
