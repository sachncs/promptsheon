package context

import (
	"strings"
)

type TokenEstimateFunc func(string) int

func DefaultTokenEstimate(text string) int {
	if text == "" {
		return 0
	}
	words := strings.Fields(text)
	return int(float64(len(words)) * 1.3)
}

type AssembledContext struct {
	SystemMessage string
	Messages      []AssembledMessage
	TokenCount    int
	Truncated     bool
	Strategy      string
}

type AssembledMessage struct {
	Role    string
	Content string
}

type Manager struct {
	estimateTokens TokenEstimateFunc
}

func NewManager() *Manager {
	return &Manager{
		estimateTokens: DefaultTokenEstimate,
	}
}

func NewManagerWithEstimator(estimator TokenEstimateFunc) *Manager {
	return &Manager{
		estimateTokens: estimator,
	}
}

func (m *Manager) EstimateTokens(text string) int {
	return m.estimateTokens(text)
}
