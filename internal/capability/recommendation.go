package capability

import "time"

// RecommendationType identifies the kind of optimization suggested.
type RecommendationType string

const (
	// RecommendationSwitchModel suggests switching to a different model.
	RecommendationSwitchModel      RecommendationType = "switch_model"
	// RecommendationCompressPrompt suggests compressing the prompt.
	RecommendationCompressPrompt   RecommendationType = "compress_prompt"
	// RecommendationReduceContext suggests reducing context.
	RecommendationReduceContext    RecommendationType = "reduce_context"
	// RecommendationEnableCache suggests enabling caching.
	RecommendationEnableCache      RecommendationType = "enable_cache"
	// RecommendationDisableReasoning suggests disabling reasoning.
	RecommendationDisableReasoning RecommendationType = "disable_reasoning"
	// RecommendationUpgradeMCP suggests upgrading MCP server.
	RecommendationUpgradeMCP       RecommendationType = "upgrade_mcp"
	// RecommendationRemoveTool suggests removing a tool.
	RecommendationRemoveTool       RecommendationType = "remove_tool"
	// RecommendationSplitCapability suggests splitting the capability.
	RecommendationSplitCapability  RecommendationType = "split_capability"
	// RecommendationAddGuardrail suggests adding a guardrail.
	RecommendationAddGuardrail     RecommendationType = "add_guardrail"
	// RecommendationTunePolicy suggests tuning the policy.
	RecommendationTunePolicy       RecommendationType = "tune_policy"
)

// Recommendation suggests an improvement to a capability version.
//
// This is one of the most valuable objects in the system — it closes the
// feedback loop by converting production telemetry and evaluation results
// into actionable suggestions that drive the next version.
type Recommendation struct {
	ID                   string             `json:"id"`
	CapabilityVersionID  string             `json:"capability_version_id"`
	Type                 RecommendationType `json:"type"`
	Reason               string             `json:"reason"`
	Confidence           float64            `json:"confidence"`
	ExpectedSavingsUSD   float64            `json:"expected_savings_usd,omitempty"`
	ExpectedQualityDelta float64            `json:"expected_quality_delta,omitempty"`
	Impact               string             `json:"impact"` // "low", "medium", "high"
	AutoApplicable       bool               `json:"auto_applicable"`
	CreatedAt            time.Time          `json:"created_at"`
	AppliedAt            *time.Time         `json:"applied_at,omitempty"`
}
