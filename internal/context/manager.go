// Package context provides context assembly and token budget management
// for agent steps. It handles system prompts, conversation history,
// and automatic truncation when token budgets are exceeded.
package context

import (
	"context"
	"fmt"
	"strings"

	"promptsheon/internal/models"
	"promptsheon/internal/store"
)

// TokenEstimateFunc estimates the number of tokens in a string.
type TokenEstimateFunc func(string) int

// DefaultTokenEstimate uses ~1.3 tokens per word heuristic.
func DefaultTokenEstimate(text string) int {
	if text == "" {
		return 0
	}
	words := strings.Fields(text)
	return int(float64(len(words)) * 1.3)
}

// AssembledContext holds the result of context assembly.
type AssembledContext struct {
	SystemMessage string             // rendered system prompt with variable substitution
	Messages      []AssembledMessage // conversation history messages
	TokenCount    int                // total tokens used
	Truncated     bool               // true if truncation was applied
	Strategy      models.TruncationStrategy
}

// AssembledMessage is a single message ready for the LLM.
type AssembledMessage struct {
	Role    string
	Content string
}

// Manager handles context assembly and token budget management.
type Manager struct {
	db             store.Repository
	estimateTokens TokenEstimateFunc
}

// NewManager creates a new context manager.
func NewManager(db store.Repository) *Manager {
	return &Manager{
		db:             db,
		estimateTokens: DefaultTokenEstimate,
	}
}

// NewManagerWithEstimator creates a context manager with a custom token estimator.
func NewManagerWithEstimator(db store.Repository, estimator TokenEstimateFunc) *Manager {
	return &Manager{
		db:             db,
		estimateTokens: estimator,
	}
}

// Assemble loads a context by ID and assembles it with variable substitution.
// The system prompt is rendered with variables, conversation messages are loaded,
// and the result respects the token budget via the configured truncation strategy.
func (m *Manager) Assemble(ctx context.Context, contextID string, variables map[string]string) (*AssembledContext, error) {
	c, err := m.db.GetContext(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("load context: %w", err)
	}

	return m.AssembleFromContext(c, variables)
}

// AssembleFromContext assembles a context that's already loaded.
func (m *Manager) AssembleFromContext(c *models.Context, variables map[string]string) (*AssembledContext, error) {
	result := &AssembledContext{
		Strategy: c.TruncationStrategy,
	}

	// Render system prompt with variable substitution
	systemPrompt := c.SystemPrompt
	for k, v := range variables {
		systemPrompt = strings.ReplaceAll(systemPrompt, "{{"+k+"}}", v)
	}
	result.SystemMessage = systemPrompt
	result.TokenCount = m.estimateTokens(systemPrompt)

	// Add conversation messages (most recent first for sliding window)
	messages := make([]models.ContextMessage, len(c.Messages))
	copy(messages, c.Messages)

	// Calculate total tokens from messages
	messageTokens := 0
	for _, msg := range messages {
		if msg.TokenCount > 0 {
			messageTokens += msg.TokenCount
		} else {
			messageTokens += m.estimateTokens(msg.Content)
		}
	}

	totalTokens := result.TokenCount + messageTokens

	// Apply truncation if over budget
	if c.TokenBudget > 0 && totalTokens > c.TokenBudget {
		messages = m.truncateMessages(messages, c.TokenBudget-result.TokenCount, c.TruncationStrategy)
		result.Truncated = true

		// Recalculate token count after truncation
		messageTokens = 0
		for _, msg := range messages {
			if msg.TokenCount > 0 {
				messageTokens += msg.TokenCount
			} else {
				messageTokens += m.estimateTokens(msg.Content)
			}
		}
		result.TokenCount += messageTokens
	}

	// Convert to assembled messages
	result.Messages = make([]AssembledMessage, len(messages))
	for i, msg := range messages {
		result.Messages[i] = AssembledMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	return result, nil
}

// truncateMessages applies the configured truncation strategy to fit within the token budget.
func (m *Manager) truncateMessages(messages []models.ContextMessage, budget int, strategy models.TruncationStrategy) []models.ContextMessage {
	if budget <= 0 {
		return nil
	}

	switch strategy {
	case models.TruncationSlidingWindow:
		return m.slidingWindow(messages, budget)
	case models.TruncationDropOldest:
		return m.dropOldest(messages, budget)
	default:
		return m.slidingWindow(messages, budget)
	}
}

// slidingWindow keeps the most recent messages that fit within the budget.
func (m *Manager) slidingWindow(messages []models.ContextMessage, budget int) []models.ContextMessage {
	var result []models.ContextMessage
	used := 0

	// Iterate from most recent to oldest, collecting messages
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		tokens := msg.TokenCount
		if tokens == 0 {
			tokens = m.estimateTokens(msg.Content)
		}

		if used+tokens > budget {
			break
		}

		used += tokens
		result = append(result, msg) // collect in reverse order
	}

	// Reverse to restore chronological order
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// dropOldest removes the oldest messages until under budget.
func (m *Manager) dropOldest(messages []models.ContextMessage, budget int) []models.ContextMessage {
	used := 0
	for i, msg := range messages {
		tokens := msg.TokenCount
		if tokens == 0 {
			tokens = m.estimateTokens(msg.Content)
		}
		used += tokens

		if used > budget {
			return messages[i:]
		}
	}
	return messages
}

// EstimateTokens returns the estimated token count for a string.
func (m *Manager) EstimateTokens(text string) int {
	return m.estimateTokens(text)
}
