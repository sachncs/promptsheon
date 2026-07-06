package guardrail

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/sachn-cs/promptsheon/internal/capability"
	"github.com/sachn-cs/promptsheon/internal/metrics"
)

func testManager() *Manager {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	collector := metrics.NewCollector()
	return NewManager(logger, collector)
}

func TestCheckVersion_NoGuardrails(t *testing.T) {
	mgr := testManager()
	ctx := context.Background()

	version := &capability.CapabilityVersion{
		ID:      "ver-1",
		Version: 1,
		Prompt: capability.Prompt{
			Instructions: "test instructions",
		},
		// No guardrails configured
	}

	err := mgr.CheckVersion(ctx, version)
	if err != nil {
		t.Fatalf("CheckVersion: %v", err)
	}
}

func TestCheckVersion_NilVersion(t *testing.T) {
	mgr := testManager()
	err := mgr.CheckVersion(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil version")
	}
}

func TestCheckVersion_PreGuardrailPasses(t *testing.T) {
	mgr := testManager()
	ctx := context.Background()

	version := &capability.CapabilityVersion{
		ID:      "ver-1",
		Version: 1,
		Prompt: capability.Prompt{
			Instructions: "safe content",
		},
		Guardrails: []capability.Guardrail{
			{
				ID:    "gr-1",
				Name:  "length-check",
				Phase: capability.GuardrailPhasePre,
				Config: map[string]any{
					"max_prompt_length": float64(1000),
				},
			},
		},
	}

	err := mgr.CheckVersion(ctx, version)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCheckVersion_PreGuardrailLengthExceeded(t *testing.T) {
	mgr := testManager()
	ctx := context.Background()

	longPrompt := make([]byte, 5000)
	for i := range longPrompt {
		longPrompt[i] = 'a'
	}

	version := &capability.CapabilityVersion{
		ID:      "ver-1",
		Version: 1,
		Prompt: capability.Prompt{
			Instructions: string(longPrompt),
		},
		Guardrails: []capability.Guardrail{
			{
				ID:    "gr-1",
				Name:  "length-check",
				Phase: capability.GuardrailPhasePre,
				Config: map[string]any{
					"max_prompt_length": float64(100),
				},
			},
		},
	}

	err := mgr.CheckVersion(ctx, version)
	if err == nil {
		t.Fatal("expected error for exceeded prompt length")
	}
}

func TestCheckVersion_PreGuardrailRestrictedTerms(t *testing.T) {
	mgr := testManager()
	ctx := context.Background()

	version := &capability.CapabilityVersion{
		ID:      "ver-1",
		Version: 1,
		Prompt: capability.Prompt{
			Instructions: "this contains secret_password in the text",
		},
		Guardrails: []capability.Guardrail{
			{
				ID:    "gr-1",
				Name:  "term-check",
				Phase: capability.GuardrailPhasePre,
				Config: map[string]any{
					"restricted_terms": []any{"secret_password"},
				},
			},
		},
	}

	err := mgr.CheckVersion(ctx, version)
	if err == nil {
		t.Fatal("expected error for restricted term")
	}
}
