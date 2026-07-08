// Package context provides context management for LLM prompts.
package context

import (
	"strings"
)

// TokenEstimateFunc estimates the token count for a given text.
type TokenEstimateFunc func(string) int

// DefaultTokenEstimate estimates tokens based on word count.
func DefaultTokenEstimate(text string) int {
	if text == "" {
		return 0
	}
	words := strings.Fields(text)
	return int(float64(len(words)) * 1.3)
}

// AssembledContext holds the assembled context with messages and token count.
type AssembledContext struct {
	SystemMessage string
	Messages      []AssembledMessage
	TokenCount    int
	Truncated     bool
	Strategy      string
}

// AssembledMessage represents a single message in the assembled context.
type AssembledMessage struct {
	Role    string
	Content string
}

// Manager manages token estimation and context assembly.
type Manager struct {
	estimateTokens TokenEstimateFunc
}

// NewManager creates a new Manager with default token estimation.
func NewManager() *Manager {
	return &Manager{
		estimateTokens: DefaultTokenEstimate,
	}
}

// NewManagerWithEstimator creates a new Manager with a custom estimator.
func NewManagerWithEstimator(estimator TokenEstimateFunc) *Manager {
	return &Manager{
		estimateTokens: estimator,
	}
}

// EstimateTokens estimates the token count for the given text.
func (m *Manager) EstimateTokens(text string) int {
	return m.estimateTokens(text)
}
