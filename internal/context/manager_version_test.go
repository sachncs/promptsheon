package context

import (
	"testing"

	"github.com/sachncs/promptsheon/internal/capability"
)

func TestAssembleFromContract_Nil(t *testing.T) {
	m := NewManager()
	_, err := m.AssembleFromContract(nil)
	if err == nil {
		t.Fatal("expected error for nil contract")
	}
}

func TestAssembleFromContract_Empty(t *testing.T) {
	m := NewManager()
	contract := &capability.ContextContract{}
	assembled, err := m.AssembleFromContract(contract)
	if err != nil {
		t.Fatalf("AssembleFromContract: %v", err)
	}
	if assembled == nil {
		t.Fatal("expected non-nil assembled context")
	}
}

func TestAssembleFromContract_WithRequired(t *testing.T) {
	m := NewManager()
	contract := &capability.ContextContract{
		RequiredContext: []capability.ContextRef{
			{Key: "user_input", Source: "session"},
			{Key: "document", Source: "knowledge"},
		},
	}
	assembled, err := m.AssembleFromContract(contract)
	if err != nil {
		t.Fatalf("AssembleFromContract: %v", err)
	}
	if assembled == nil {
		t.Fatal("expected non-nil assembled context")
	}
}

func TestAssembleFromContract_EmptyKey(t *testing.T) {
	m := NewManager()
	contract := &capability.ContextContract{
		RequiredContext: []capability.ContextRef{
			{Key: ""},
		},
	}
	_, err := m.AssembleFromContract(contract)
	if err == nil {
		t.Fatal("expected error for empty context key")
	}
}

func TestAssembleFromContract_MaximumSize(t *testing.T) {
	m := NewManager()
	contract := &capability.ContextContract{
		MaximumSize: 4096,
	}
	assembled, err := m.AssembleFromContract(contract)
	if err != nil {
		t.Fatalf("AssembleFromContract: %v", err)
	}
	if assembled.TokenCount != 4096 {
		t.Errorf("expected token count 4096, got %d", assembled.TokenCount)
	}
}

func TestAssembleFromContract_CompressionStrategy(t *testing.T) {
	m := NewManager()
	contract := &capability.ContextContract{
		CompressionStrategy: "summary",
	}
	assembled, err := m.AssembleFromContract(contract)
	if err != nil {
		t.Fatalf("AssembleFromContract: %v", err)
	}
	if assembled.Strategy != "summary" {
		t.Errorf("expected strategy 'summary', got %q", assembled.Strategy)
	}
}

func TestDefaultTokenEstimate_Empty(t *testing.T) {
	if got := DefaultTokenEstimate(""); got != 0 {
		t.Errorf("DefaultTokenEstimate(\"\") = %d, want 0", got)
	}
}

func TestDefaultTokenEstimate_SpacesOnly(t *testing.T) {
	if got := DefaultTokenEstimate("   "); got != 0 {
		t.Errorf("DefaultTokenEstimate(\"   \") = %d, want 0", got)
	}
}

func TestDefaultTokenEstimate_Words(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{input: "hello", want: 1},
		{input: "hello world", want: 2},
		{input: "one two three four five", want: 6},
		{input: "a b c d e f g h i j", want: 13},
	}
	for _, tt := range tests {
		if got := DefaultTokenEstimate(tt.input); got != tt.want {
			t.Errorf("DefaultTokenEstimate(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestNewManagerWithEstimator(t *testing.T) {
	custom := func(s string) int { return len(s) }
	m := NewManagerWithEstimator(custom)
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if got := m.EstimateTokens("hello"); got != 5 {
		t.Errorf("EstimateTokens with custom estimator = %d, want 5", got)
	}
}

func TestEstimateTokens_Default(t *testing.T) {
	m := NewManager()
	if got := m.EstimateTokens("hello world"); got != 2 {
		t.Errorf("EstimateTokens(\"hello world\") = %d, want 2", got)
	}
}

func TestEstimateTokens_Empty(t *testing.T) {
	m := NewManager()
	if got := m.EstimateTokens(""); got != 0 {
		t.Errorf("EstimateTokens(\"\") = %d, want 0", got)
	}
}

func TestAssembleFromContract_ForbiddenContext(t *testing.T) {
	m := NewManager()
	contract := &capability.ContextContract{
		ForbiddenContext: []string{"password", "secret"},
		RequiredContext: []capability.ContextRef{
			{Key: "safe_data", Source: "session"},
		},
	}
	assembled, err := m.AssembleFromContract(contract)
	if err != nil {
		t.Fatalf("AssembleFromContract: %v", err)
	}
	if assembled == nil {
		t.Fatal("expected non-nil assembled context")
	}
}
