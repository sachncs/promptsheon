package optimizer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
)

const impactHigh = "high"
const cachingDisabled = "disabled"

// AnalyzeVersion analyzes a capability version and returns recommendations.
//
// This is the capability-centric equivalent of AnalyzePrompt.
// It generates all recommendation types (switch model, compress prompt,
// reduce context, enable cache, disable reasoning, upgrade MCP, remove
// tool, split capability, add guardrail, tune policy).
func (o *Optimizer) AnalyzeVersion(_ context.Context, version *capability.Version) ([]capability.Recommendation, error) {
	if version == nil {
		return nil, fmt.Errorf("capability version is required")
	}

	recs := make([]capability.Recommendation, 0, 5)

	recs = append(recs, o.analyzePromptContent(version)...)
	recs = append(recs, o.analyzeModelPolicy(version)...)
	recs = append(recs, o.analyzeContextContract(version)...)
	recs = append(recs, o.analyzeRuntimePolicy(version)...)
	recs = append(recs, o.analyzeTools(version)...)
	recs = append(recs, o.analyzeGuardrails(version)...)

	return recs, nil
}

func (o *Optimizer) analyzePromptContent(version *capability.Version) []capability.Recommendation {
	var recs []capability.Recommendation
	promptText := version.Prompt.Instructions
	if version.Prompt.Template != "" {
		promptText = version.Prompt.Template
	}

	if len(promptText) > 2000 {
		recs = append(recs, capability.Recommendation{
			ID:                  fmt.Sprintf("rec-compress-%d", time.Now().UnixNano()),
			CapabilityVersionID: version.ID,
			Type:                capability.RecommendationCompressPrompt,
			Reason:              fmt.Sprintf("Prompt is %d characters, consider compressing to reduce cost and latency", len(promptText)),
			Confidence:          0.7,
			Impact:              "medium",
			AutoApplicable:      false,
			CreatedAt:           time.Now(),
		})
	}

	if len(version.Prompt.Examples) == 0 && promptText != "" {
		recs = append(recs, capability.Recommendation{
			ID:                  fmt.Sprintf("rec-examples-%d", time.Now().UnixNano()),
			CapabilityVersionID: version.ID,
			Type:                capability.RecommendationCompressPrompt,
			Reason:              "Consider adding few-shot examples to improve output quality",
			Confidence:          0.5,
			Impact:              "medium",
			AutoApplicable:      false,
			CreatedAt:           time.Now(),
		})
	}

	return recs
}

func (o *Optimizer) analyzeModelPolicy(version *capability.Version) []capability.Recommendation {
	var recs []capability.Recommendation
	mp := version.ModelPolicy

	if mp.Requirements.NeedsReasoning && !promptNeedsReasoning(&version.Prompt) {
		recs = append(recs, capability.Recommendation{
			ID:                   fmt.Sprintf("rec-reasoning-%d", time.Now().UnixNano()),
			CapabilityVersionID:  version.ID,
			Type:                 capability.RecommendationDisableReasoning,
			Reason:               "Reasoning is enabled but the prompt may not require it. Disabling can reduce latency and cost.",
			Confidence:           0.6,
			ExpectedSavingsUSD:   0.002,
			ExpectedQualityDelta: 0.0,
			Impact:               impactHigh,
			AutoApplicable:       true,
			CreatedAt:            time.Now(),
		})
	}

	if mp.Requirements.MaxLatencyMs > 0 && mp.Requirements.MaxLatencyMs < 500 {
		recs = append(recs, capability.Recommendation{
			ID:                  fmt.Sprintf("rec-latency-%d", time.Now().UnixNano()),
			CapabilityVersionID: version.ID,
			Type:                capability.RecommendationSwitchModel,
			Reason:              fmt.Sprintf("Latency requirement of %dms may need a faster model", mp.Requirements.MaxLatencyMs),
			Confidence:          0.5,
			Impact:              impactHigh,
			AutoApplicable:      false,
			CreatedAt:           time.Now(),
		})
	}

	return recs
}

func (o *Optimizer) analyzeContextContract(version *capability.Version) []capability.Recommendation {
	var recs []capability.Recommendation
	cc := version.ContextContract

	if cc.MaximumSize > 0 && cc.MaximumSize < 1000 {
		recs = append(recs, capability.Recommendation{
			ID:                  fmt.Sprintf("rec-context-%d", time.Now().UnixNano()),
			CapabilityVersionID: version.ID,
			Type:                capability.RecommendationReduceContext,
			Reason:              fmt.Sprintf("Context limit is very low (%d tokens), consider increasing or compressing", cc.MaximumSize),
			Confidence:          0.4,
			Impact:              "low",
			AutoApplicable:      false,
			CreatedAt:           time.Now(),
		})
	}

	return recs
}

func (o *Optimizer) analyzeRuntimePolicy(version *capability.Version) []capability.Recommendation {
	var recs []capability.Recommendation
	rp := version.RuntimePolicy

	if rp.Caching == "" || rp.Caching == cachingDisabled {
		recs = append(recs, capability.Recommendation{
			ID:                  fmt.Sprintf("rec-cache-%d", time.Now().UnixNano()),
			CapabilityVersionID: version.ID,
			Type:                capability.RecommendationEnableCache,
			Reason:              "Caching is disabled, enabling it can reduce cost and latency for repeated inputs",
			Confidence:          0.8,
			Impact:              impactHigh,
			AutoApplicable:      true,
			CreatedAt:           time.Now(),
		})
	}

	return recs
}

func (o *Optimizer) analyzeTools(version *capability.Version) []capability.Recommendation {
	var recs []capability.Recommendation

	if len(version.Tools) > 3 {
		recs = append(recs, capability.Recommendation{
			ID:                  fmt.Sprintf("rec-tools-%d", time.Now().UnixNano()),
			CapabilityVersionID: version.ID,
			Type:                capability.RecommendationRemoveTool,
			Reason:              fmt.Sprintf("Version has %d tools configured, consider removing unused ones", len(version.Tools)),
			Confidence:          0.3,
			Impact:              "low",
			AutoApplicable:      false,
			CreatedAt:           time.Now(),
		})
	}

	return recs
}

func (o *Optimizer) analyzeGuardrails(version *capability.Version) []capability.Recommendation {
	var recs []capability.Recommendation

	if len(version.Guardrails) == 0 {
		recs = append(recs, capability.Recommendation{
			ID:                  fmt.Sprintf("rec-guardrails-%d", time.Now().UnixNano()),
			CapabilityVersionID: version.ID,
			Type:                capability.RecommendationAddGuardrail,
			Reason:              "No guardrails configured — consider adding pre and post execution guardrails",
			Confidence:          0.9,
			Impact:              impactHigh,
			AutoApplicable:      false,
			CreatedAt:           time.Now(),
		})
	}

	return recs
}

// promptNeedsReasoning is a heuristic that checks if the prompt
// content likely needs reasoning capabilities.
func promptNeedsReasoning(p *capability.Prompt) bool {
	text := strings.ToLower(p.Instructions + " " + p.Template)
	reasoningKeywords := []string{"reason", "explain", "analyze", "compare",
		"evaluate", "synthesize", "logic", "infer", "deduce", "conclude"}
	for _, kw := range reasoningKeywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}
