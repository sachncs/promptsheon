package guardrail

import (
	"context"
	"fmt"

	"github.com/sachncs/promptsheon/internal/capability"
)

// CheckVersion runs all guardrails defined in a capability version.
//
// This is the capability-centric equivalent of RunAllStaticChecks.
// It evaluates pre-execution guardrails against the version's configuration.
func (m *Manager) CheckVersion(ctx context.Context, version *capability.Version) error {
	if version == nil {
		return fmt.Errorf("capability version is required")
	}

	for _, g := range version.Guardrails {
		if !m.isRuleEnabled(g.Name) {
			continue
		}

		switch g.Phase {
		case capability.GuardrailPhasePre:
			if err := m.checkPreGuardrail(ctx, &g, version); err != nil {
				return err
			}
		case capability.GuardrailPhaseRuntime:
			// Runtime guardrails are checked during execution
			// and are handled by the execution flow, not here.
		case capability.GuardrailPhasePost:
			// Post-execution guardrails are checked after the LLM
			// call returns, handled by the caller.
		}
	}

	return nil
}

// isRuleEnabled checks if a guardrail rule is active in the manager.
func (m *Manager) isRuleEnabled(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, rule := range m.rules {
		if rule.Name == name {
			return rule.Enabled
		}
	}
	return true // default to enabled if not found in manager
}

// checkPreGuardrail evaluates a single pre-execution guardrail.
func (m *Manager) checkPreGuardrail(_ context.Context, g *capability.Guardrail, version *capability.Version) error {
	// Validate prompt content against guardrail config
	promptText := version.Prompt.Instructions
	if version.Prompt.Template != "" {
		promptText = version.Prompt.Template
	}

	// Check prompt length
	if maxLen, ok := g.Config["max_prompt_length"].(float64); ok {
		if len(promptText) > int(maxLen) {
			return fmt.Errorf("guardrail %s: prompt length %d exceeds max %d",
				g.Name, len(promptText), int(maxLen))
		}
	}

	// Check restricted terms
	if terms, ok := g.Config["restricted_terms"].([]any); ok {
		for _, term := range terms {
			if termStr, ok := term.(string); ok {
				if contains(promptText, termStr) {
					return fmt.Errorf("guardrail %s: restricted term found: %s",
						g.Name, termStr)
				}
			}
		}
	}

	return nil
}

// contains is a simple substring check (avoids importing strings in this file).
func contains(s, substr string) bool {
	return substr == "" || findInString(s, substr)
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
