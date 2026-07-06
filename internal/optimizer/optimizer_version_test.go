package optimizer

import (
	"context"
	"testing"

	"github.com/sachn-cs/promptsheon/internal/capability"
)

func TestAnalyzeVersion_Nil(t *testing.T) {
	o := NewOptimizer(nil)
	_, err := o.AnalyzeVersion(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil version")
	}
}

func TestAnalyzeVersion_EmptyVersion(t *testing.T) {
	o := NewOptimizer(nil)
	ctx := context.Background()

	version := &capability.CapabilityVersion{
		ID:      "ver-1",
		Version: 1,
		Prompt: capability.Prompt{
			Instructions: "short prompt",
		},
	}

	recs, err := o.AnalyzeVersion(ctx, version)
	if err != nil {
		t.Fatalf("AnalyzeVersion: %v", err)
	}

	// Should have at least a guardrail recommendation (no guardrails configured)
	if len(recs) == 0 {
		t.Fatal("expected at least one recommendation")
	}

	// Check guardrail recommendation is present
	hasGuardrailRec := false
	for _, r := range recs {
		if r.Type == capability.RecommendationAddGuardrail {
			hasGuardrailRec = true
			break
		}
	}
	if !hasGuardrailRec {
		t.Errorf("expected guardrail recommendation for version with no guardrails")
	}
}

func TestAnalyzeVersion_AllRecommendationTypes(t *testing.T) {
	o := NewOptimizer(nil)
	ctx := context.Background()

	version := &capability.CapabilityVersion{
		ID:      "ver-1",
		Version: 1,
		Prompt: capability.Prompt{
			Instructions: makeLongString(2500),
		},
		ModelPolicy: capability.ModelPolicy{
			Requirements: capability.ModelRequirements{
				NeedsReasoning: true,
				MaxLatencyMs:   200,
			},
		},
		ContextContract: capability.ContextContract{
			MaximumSize: 500,
		},
		RuntimePolicy: capability.RuntimePolicy{
			Caching: "disabled",
		},
		Tools: []capability.Tool{
			{ID: "t1"}, {ID: "t2"}, {ID: "t3"}, {ID: "t4"},
		},
	}

	recs, err := o.AnalyzeVersion(ctx, version)
	if err != nil {
		t.Fatalf("AnalyzeVersion: %v", err)
	}

	if len(recs) < 5 {
		t.Errorf("expected at least 5 recommendations, got %d", len(recs))
	}

	// Verify all expected types appear
	typeCounts := make(map[capability.RecommendationType]int)
	for _, r := range recs {
		typeCounts[r.Type]++
	}

	expectedTypes := []capability.RecommendationType{
		capability.RecommendationCompressPrompt,
		capability.RecommendationDisableReasoning,
		capability.RecommendationSwitchModel,
		capability.RecommendationReduceContext,
		capability.RecommendationEnableCache,
		capability.RecommendationRemoveTool,
		capability.RecommendationAddGuardrail,
	}

	for _, et := range expectedTypes {
		if typeCounts[et] == 0 {
			t.Errorf("missing recommendation type: %s", et)
		}
	}
}

func TestPromptNeedsReasoning(t *testing.T) {
	tests := []struct {
		name     string
		prompt   *capability.Prompt
		expected bool
	}{
		{
			name:     "explicit reasoning keyword",
			prompt:   &capability.Prompt{Instructions: "Analyze this data and explain your reasoning"},
			expected: true,
		},
		{
			name:     "no reasoning keywords",
			prompt:   &capability.Prompt{Instructions: "Translate to French"},
			expected: false,
		},
		{
			name:     "reasoning in template",
			prompt:   &capability.Prompt{Instructions: "Do task", Template: "Compare {{.a}} and {{.b}}"},
			expected: true,
		},
		{
			name:     "empty prompt",
			prompt:   &capability.Prompt{Instructions: ""},
			expected: false,
		},
		{
			name:     "deduce keyword",
			prompt:   &capability.Prompt{Instructions: "Deduce the answer from context"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := promptNeedsReasoning(tt.prompt)
			if got != tt.expected {
				t.Errorf("promptNeedsReasoning() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAnalyzeModelPolicy_ReasoningButNotNeeded(t *testing.T) {
	o := NewOptimizer(nil)
	version := &capability.CapabilityVersion{
		ID: "ver-1",
		Prompt: capability.Prompt{
			Instructions: "Translate to Spanish",
		},
		ModelPolicy: capability.ModelPolicy{
			Requirements: capability.ModelRequirements{
				NeedsReasoning: true,
			},
		},
	}

	recs := o.analyzeModelPolicy(version)
	hasDisableReasoning := false
	for _, r := range recs {
		if r.Type == capability.RecommendationDisableReasoning {
			hasDisableReasoning = true
			if !r.AutoApplicable {
				t.Errorf("disable reasoning should be auto-applicable")
			}
			if r.Confidence != 0.6 {
				t.Errorf("expected confidence 0.6, got %f", r.Confidence)
			}
			break
		}
	}
	if !hasDisableReasoning {
		t.Errorf("expected disable reasoning recommendation")
	}
}

func TestAnalyzeRuntimePolicy_CacheDisabled(t *testing.T) {
	o := NewOptimizer(nil)
	version := &capability.CapabilityVersion{
		ID: "ver-1",
		RuntimePolicy: capability.RuntimePolicy{
			Caching: "disabled",
		},
	}

	recs := o.analyzeRuntimePolicy(version)
	hasCaching := false
	for _, r := range recs {
		if r.Type == capability.RecommendationEnableCache {
			hasCaching = true
			if r.Confidence != 0.8 {
				t.Errorf("expected confidence 0.8 for cache recommendation")
			}
			break
		}
	}
	if !hasCaching {
		t.Errorf("expected enable cache recommendation")
	}
}

func TestAnalyzeTools_TooMany(t *testing.T) {
	o := NewOptimizer(nil)
	version := &capability.CapabilityVersion{
		ID: "ver-1",
		Tools: []capability.Tool{
			{ID: "t1"}, {ID: "t2"}, {ID: "t3"}, {ID: "t4"},
		},
	}

	recs := o.analyzeTools(version)
	hasRemove := false
	for _, r := range recs {
		if r.Type == capability.RecommendationRemoveTool {
			hasRemove = true
			break
		}
	}
	if !hasRemove {
		t.Errorf("expected remove tool recommendation for 4+ tools")
	}
}

func TestAnalyzeGuardrails_Missing(t *testing.T) {
	o := NewOptimizer(nil)
	version := &capability.CapabilityVersion{ID: "ver-1"}

	recs := o.analyzeGuardrails(version)
	hasGuardrail := false
	for _, r := range recs {
		if r.Type == capability.RecommendationAddGuardrail {
			hasGuardrail = true
			if r.Confidence != 0.9 {
				t.Errorf("expected confidence 0.9 for guardrail recommendation")
			}
			break
		}
	}
	if !hasGuardrail {
		t.Errorf("expected add guardrail recommendation")
	}
}

func makeLongString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}
