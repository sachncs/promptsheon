package context

import (
	"strings"
	"testing"

	"github.com/sachn-cs/promptsheon/internal/models"
)

// newManagerWithEstimator builds a Manager with a deterministic
// estimator so test assertions don't depend on the
// word-counting heuristic in DefaultTokenEstimate. Each token
// is roughly 4 characters; the test passes "foo" which is 3
// chars so the per-word estimator returns a non-zero value.
func newManagerWithEstimator(t *testing.T) *Manager {
	t.Helper()
	// 1 token per word is the simplest possible estimator. It
	// is not realistic but it gives exact test math: a string
	// with N words has exactly N tokens.
	return NewManagerWithEstimator(nil, func(s string) int {
		return len(strings.Fields(s))
	})
}

func TestDefaultTokenEstimate(t *testing.T) {
	// Pin the heuristic so a future change to the formula is
	// visible to test output.
	if got := DefaultTokenEstimate(""); got != 0 {
		t.Errorf("empty string: got %d, want 0", got)
	}
	if got := DefaultTokenEstimate("hello world"); got == 0 {
		t.Error("expected non-zero estimate for non-empty string")
	}
	// Three words × 1.3 = 3.9 → int 3
	got := DefaultTokenEstimate("one two three")
	if got < 3 || got > 4 {
		t.Errorf("DefaultTokenEstimate: got %d, want 3 or 4", got)
	}
}

func TestAssembleFromContextNoTruncation(t *testing.T) {
	m := newManagerWithEstimator(t)
	c := &models.Context{
		SystemPrompt:      "You are a helpful {{role}} assistant.",
		TokenBudget:       100,
		TruncationStrategy: models.TruncationSlidingWindow,
		Messages: []models.ContextMessage{
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: "hello there"},
		},
	}
	got, err := m.AssembleFromContext(c, map[string]string{"role": "coding"})
	if err != nil {
		t.Fatalf("AssembleFromContext: %v", err)
	}
	if !strings.Contains(got.SystemMessage, "coding") {
		t.Errorf("expected variable substitution, got %q", got.SystemMessage)
	}
	if got.SystemMessage != "You are a helpful coding assistant." {
		t.Errorf("unexpected system message: %q", got.SystemMessage)
	}
	if got.Truncated {
		t.Error("expected Truncated=false with a 100-token budget")
	}
	if len(got.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(got.Messages))
	}
}

func TestAssembleFromContextSlidingWindowTruncation(t *testing.T) {
	m := newManagerWithEstimator(t)
	c := &models.Context{
		SystemPrompt:      "sys",
		TokenBudget:       5, // tight budget
		TruncationStrategy: models.TruncationSlidingWindow,
		Messages: []models.ContextMessage{
			{Role: "user", Content: "one two three four"},        // 4 tokens
			{Role: "assistant", Content: "five six seven eight"},  // 4 tokens
			{Role: "user", Content: "nine ten eleven twelve"},     // 4 tokens
		},
	}
	got, err := m.AssembleFromContext(c, nil)
	if err != nil {
		t.Fatalf("AssembleFromContext: %v", err)
	}
	if !got.Truncated {
		t.Error("expected Truncated=true with 5-token budget")
	}
	// 1 token for the system prompt leaves 4 for messages.
	// Sliding window keeps the most recent set that fits.
	if len(got.Messages) == 0 {
		t.Fatal("expected at least one message after truncation")
	}
	// Each message is 4 tokens; with a 4-token message budget
	// we should keep exactly the most recent one.
	if len(got.Messages) > 1 {
		t.Errorf("expected at most 1 message after truncation, got %d", len(got.Messages))
	}
	if len(got.Messages) > 0 && got.Messages[0].Content != "nine ten eleven twelve" {
		t.Errorf("expected most recent message, got %q", got.Messages[0].Content)
	}
}

func TestAssembleFromContextDropOldestTruncation(t *testing.T) {
	m := newManagerWithEstimator(t)
	c := &models.Context{
		SystemPrompt:      "sys",
		TokenBudget:       5,
		TruncationStrategy: models.TruncationDropOldest,
		Messages: []models.ContextMessage{
			{Role: "user", Content: "a b c d"},        // 4 tokens
			{Role: "assistant", Content: "e f g h"},   // 4 tokens
			{Role: "user", Content: "i j k l"},        // 4 tokens
		},
	}
	got, err := m.AssembleFromContext(c, nil)
	if err != nil {
		t.Fatalf("AssembleFromContext: %v", err)
	}
	if !got.Truncated {
		t.Error("expected Truncated=true")
	}
	// dropOldest walks the slice from the start, accumulating
	// token usage until it exceeds the budget, then returns
	// the tail. With a 4-token message budget:
	//   i=0: used=4, not > 4, continue
	//   i=1: used=8, > 4, return messages[1:]
	// So we expect the last 2 messages.
	if len(got.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(got.Messages))
	}
	if got.Messages[0].Content != "e f g h" {
		t.Errorf("expected first message 'e f g h', got %q", got.Messages[0].Content)
	}
}

func TestTruncateMessagesZeroBudget(t *testing.T) {
	m := newManagerWithEstimator(t)
	c := &models.Context{
		TokenBudget: 0,
		Messages: []models.ContextMessage{
			{Role: "user", Content: "hello"},
		},
	}
	got, err := m.AssembleFromContext(c, nil)
	if err != nil {
		t.Fatalf("AssembleFromContext: %v", err)
	}
	// 0 budget means "no limit" in the current implementation.
	// The messages should pass through unchanged.
	if len(got.Messages) != 1 {
		t.Errorf("expected 1 message with 0 budget, got %d", len(got.Messages))
	}
}

func TestEstimateTokensUsesConfiguredFunc(t *testing.T) {
	m := NewManagerWithEstimator(nil, func(s string) int { return len(s) })
	if got := m.EstimateTokens("hello"); got != 5 {
		t.Errorf("EstimateTokens: got %d, want 5", got)
	}
}

func TestAssembleFromContextEmptyMessages(t *testing.T) {
	m := newManagerWithEstimator(t)
	c := &models.Context{
		SystemPrompt: "sys",
		TokenBudget:  100,
		Messages:     nil,
	}
	got, err := m.AssembleFromContext(c, nil)
	if err != nil {
		t.Fatalf("AssembleFromContext: %v", err)
	}
	if len(got.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(got.Messages))
	}
	if got.Truncated {
		t.Error("Truncated should be false on empty input")
	}
}

func TestAssembleFromContextMessageTokenCountOverridesEstimator(t *testing.T) {
	m := newManagerWithEstimator(t)
	c := &models.Context{
		SystemPrompt: "sys",
		TokenBudget:  10,
		Messages: []models.ContextMessage{
			{Role: "user", Content: "hi", TokenCount: 2},
		},
	}
	got, err := m.AssembleFromContext(c, nil)
	if err != nil {
		t.Fatalf("AssembleFromContext: %v", err)
	}
	// TokenCount overrides the estimator; total budget is
	// 1 (sys) + 2 (msg) = 3, under 10, so no truncation.
	if got.Truncated {
		t.Error("expected Truncated=false with 2-token message and 10 budget")
	}
}
