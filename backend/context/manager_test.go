package context_test

import (
	"encoding/json"
	"strings"
	"testing"

	px "github.com/sachncs/promptsheon/internal/context"
)

func TestDefaultTokenEstimate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want int
	}{
		{name: "empty string", in: "", want: 0},
		{name: "single word", in: "hello", want: 1},         // 1 * 1.3 = 1.3 -> int = 1
		{name: "three words", in: "the quick fox", want: 3}, // 3 * 1.3 = 3.9 -> 3
		{name: "ten words", in: "one two three four five six seven eight nine ten", want: 13},
		{name: "whitespace only", in: "   \t\n  ", want: 0},
		{name: "unicode word", in: "héllo wörld", want: 2}, // 2 fields
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := px.DefaultTokenEstimate(tc.in)
			if got != tc.want {
				t.Errorf("DefaultTokenEstimate(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestNewManagerUsesDefaultEstimator(t *testing.T) {
	t.Parallel()
	m := px.NewManager()
	got := m.EstimateTokens("a b c d e")
	want := px.DefaultTokenEstimate("a b c d e")
	if got != want {
		t.Errorf("NewManager().EstimateTokens = %d, want %d", got, want)
	}
}

func TestNewManagerWithEstimator(t *testing.T) {
	t.Parallel()
	calls := 0
	estimator := func(s string) int {
		calls++
		return len(s)
	}
	m := px.NewManagerWithEstimator(estimator)
	got := m.EstimateTokens("hello")
	if got != 5 {
		t.Errorf("custom estimator length = %d, want 5", got)
	}
	if calls != 1 {
		t.Errorf("custom estimator call count = %d, want 1", calls)
	}
}

func TestManagerEstimateTokensPassesThrough(t *testing.T) {
	t.Parallel()
	want := 42
	m := px.NewManagerWithEstimator(func(string) int { return want })
	if got := m.EstimateTokens("anything"); got != want {
		t.Errorf("EstimateTokens = %d, want %d", got, want)
	}
}

func TestAssembledContextJSONRoundTrip(t *testing.T) {
	t.Parallel()
	in := px.AssembledContext{
		SystemMessage: "be helpful",
		Messages: []px.AssembledMessage{
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: "hello"},
		},
		TokenCount: 12,
		Truncated:  true,
		Strategy:   "tail",
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"Truncated":true`) {
		t.Errorf("truncated not serialized: %s", b)
	}
	var out px.AssembledContext
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.TokenCount != in.TokenCount || out.Strategy != in.Strategy {
		t.Errorf("round-trip mismatch: got %+v want %+v", out, in)
	}
	if len(out.Messages) != 2 {
		t.Errorf("messages count = %d, want 2", len(out.Messages))
	}
}

func TestAssembleRespectsBudgetTail(t *testing.T) {
	t.Parallel()
	m := px.NewManager()
	in := px.Inputs{
		SystemMessage: "be helpful",
		Messages: []px.AssembledMessage{
			{Role: "user", Content: strings.Repeat("a ", 50)},
			{Role: "assistant", Content: strings.Repeat("b ", 50)},
			{Role: "user", Content: strings.Repeat("c ", 50)},
		},
		Budget:   80, // tight: room for system + last message only
		Strategy: px.StrategyTail,
	}
	out, err := m.Assemble(in)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if !out.Truncated {
		t.Error("expected truncated = true")
	}
	if len(out.Messages) == 0 {
		t.Error("expected at least one message kept")
	}
	if out.TokenCount > in.Budget {
		t.Errorf("TokenCount %d exceeds budget %d", out.TokenCount, in.Budget)
	}
}

func TestAssembleRespectsBudgetHead(t *testing.T) {
	t.Parallel()
	m := px.NewManager()
	in := px.Inputs{
		SystemMessage: "sys",
		Messages: []px.AssembledMessage{
			{Role: "user", Content: "first"},
			{Role: "assistant", Content: "second"},
			{Role: "user", Content: "third"},
		},
		Budget:   6, // sys(1) + first(1) + second(1) = 3
		Strategy: px.StrategyHead,
	}
	out, err := m.Assemble(in)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if len(out.Messages) == 0 {
		t.Fatal("expected at least one message")
	}
	// Head strategy keeps the first message(s).
	if out.Messages[0].Content != "first" {
		t.Errorf("Head strategy should keep first message, got %q", out.Messages[0].Content)
	}
}

func TestAssembleRejectsZeroBudget(t *testing.T) {
	t.Parallel()
	m := px.NewManager()
	if _, err := m.Assemble(px.Inputs{SystemMessage: "x", Budget: 0}); err == nil {
		t.Error("expected error for zero budget")
	}
}

func TestAssembleRejectsUnknownStrategy(t *testing.T) {
	t.Parallel()
	m := px.NewManager()
	if _, err := m.Assemble(px.Inputs{SystemMessage: "x", Budget: 100, Strategy: "middle"}); err == nil {
		t.Error("expected error for unknown strategy")
	}
}

func TestAssembleReturnsErrBudgetExhaustedWhenSystemTooLarge(t *testing.T) {
	t.Parallel()
	m := px.NewManager()
	if _, err := m.Assemble(px.Inputs{
		SystemMessage: strings.Repeat("z ", 1000),
		Budget:        10,
	}); err == nil {
		t.Error("expected ErrBudgetExhausted when system message exceeds budget")
	}
}

func TestAssembleNoneStrategyKeepsEverything(t *testing.T) {
	t.Parallel()
	m := px.NewManager()
	in := px.Inputs{
		SystemMessage: "sys",
		Messages: []px.AssembledMessage{
			{Role: "user", Content: strings.Repeat("x ", 1000)},
		},
		Budget:   5,
		Strategy: px.StrategyNone,
	}
	out, err := m.Assemble(in)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if len(out.Messages) != 1 {
		t.Errorf("none strategy should keep all messages, got %d", len(out.Messages))
	}
	if out.Truncated {
		t.Error("none strategy should not flag truncated")
	}
}
