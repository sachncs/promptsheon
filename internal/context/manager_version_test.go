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
	if string(assembled.Strategy) != "summary" {
		t.Errorf("expected strategy 'summary', got %q", string(assembled.Strategy))
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
