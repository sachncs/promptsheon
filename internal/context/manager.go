// Package context provides context management for LLM prompts.
package context

import (
	"errors"
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

// Strategy is the truncation strategy applied when the assembled
// context would exceed the model's token budget.
type Strategy string

const (
	// StrategyTail keeps the most recent messages and drops the oldest.
	StrategyTail Strategy = "tail"
	// StrategyHead keeps the oldest messages and drops the most recent.
	StrategyHead Strategy = "head"
	// StrategyNone returns the context as-is even when it overflows.
	StrategyNone Strategy = "none"
)

// ErrBudgetExhausted is returned when the strategy cannot trim the
// context below the budget without losing the system message. The
// caller should fall back to a smaller model or split the call.
var ErrBudgetExhausted = errors.New("context: token budget exhausted after truncation")

// Inputs is the input shape to Assemble. SystemMessage is pinned at
// the top of every assembled context; Messages are the user /
// assistant history; Budget is the maximum total token count.
type Inputs struct {
	SystemMessage string
	Messages      []AssembledMessage
	Budget        int
	Strategy      Strategy
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

// Assemble builds an AssembledContext from the supplied inputs.
// SystemMessage is always pinned at the top; Messages are then
// ordered by the Strategy ("tail" keeps the most recent,
// "head" keeps the oldest). Truncation stops once the running
// total fits in Budget; if the SystemMessage itself exceeds
// Budget, ErrBudgetExhausted is returned.
func (m *Manager) Assemble(in Inputs) (AssembledContext, error) {
	if in.Strategy == "" {
		in.Strategy = StrategyTail
	}
	if in.Budget <= 0 {
		return AssembledContext{}, errors.New("context: budget must be positive")
	}
	sysTokens := m.estimateTokens(in.SystemMessage)
	if sysTokens > in.Budget {
		return AssembledContext{}, ErrBudgetExhausted
	}
	remaining := in.Budget - sysTokens

	messages := make([]AssembledMessage, len(in.Messages))
	copy(messages, in.Messages)
	switch in.Strategy {
	case StrategyTail:
		// Iterate from the end; keep as many recent messages as fit.
		kept := make([]AssembledMessage, 0, len(messages))
		used := 0
		for i := len(messages) - 1; i >= 0; i-- {
			cost := m.estimateTokens(messages[i].Content)
			if used+cost > remaining {
				break
			}
			used += cost
			kept = append([]AssembledMessage{messages[i]}, kept...)
		}
		messages = kept
	case StrategyHead:
		kept := make([]AssembledMessage, 0, len(messages))
		used := 0
		for _, msg := range messages {
			cost := m.estimateTokens(msg.Content)
			if used+cost > remaining {
				break
			}
			used += cost
			kept = append(kept, msg)
		}
		messages = kept
	case StrategyNone:
		// pass through
	default:
		return AssembledContext{}, errors.New("context: unknown strategy " + string(in.Strategy))
	}

	total := sysTokens
	for _, msg := range messages {
		total += m.estimateTokens(msg.Content)
	}
	return AssembledContext{
		SystemMessage: in.SystemMessage,
		Messages:      messages,
		TokenCount:    total,
		Truncated:     len(messages) < len(in.Messages),
		Strategy:      string(in.Strategy),
	}, nil
}
