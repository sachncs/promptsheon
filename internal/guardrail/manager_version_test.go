package guardrail

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/metrics"
)

func testManager() *Manager {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	collector := metrics.NewCollector()
	return NewManager(logger, collector)
}

func TestCheckVersion_NoGuardrails(t *testing.T) {
	mgr := testManager()
	ctx := context.Background()

	version := &capability.Version{
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

	version := &capability.Version{
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

	version := &capability.Version{
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

func TestCheckVersion_RuntimePhase(t *testing.T) {
	mgr := testManager()
	ctx := context.Background()

	version := &capability.Version{
		ID:      "ver-1",
		Version: 1,
		Prompt: capability.Prompt{
			Instructions: "test instructions",
		},
		Guardrails: []capability.Guardrail{
			{
				ID:    "gr-1",
				Name:  "runtime-check",
				Phase: capability.GuardrailPhaseRuntime,
			},
		},
	}

	err := mgr.CheckVersion(ctx, version)
	if err != nil {
		t.Fatalf("expected no error for runtime phase, got: %v", err)
	}
}

func TestCheckVersion_PostPhase(t *testing.T) {
	mgr := testManager()
	ctx := context.Background()

	version := &capability.Version{
		ID:      "ver-1",
		Version: 1,
		Prompt: capability.Prompt{
			Instructions: "test instructions",
		},
		Guardrails: []capability.Guardrail{
			{
				ID:    "gr-1",
				Name:  "post-check",
				Phase: capability.GuardrailPhasePost,
			},
		},
	}

	err := mgr.CheckVersion(ctx, version)
	if err != nil {
		t.Fatalf("expected no error for post phase, got: %v", err)
	}
}

func TestCheckVersion_RuleDisabled(t *testing.T) {
	mgr := testManager()
	ctx := context.Background()

	mgr.mu.Lock()
	mgr.rules["length-check"] = &Rule{
		ID:      "length-check",
		Name:    "length-check",
		Enabled: false,
	}
	mgr.mu.Unlock()

	version := &capability.Version{
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
		t.Fatalf("expected no error for disabled rule, got: %v", err)
	}
}

func TestCheckVersion_RuleNotInManagerDefaultsToEnabled(t *testing.T) {
	mgr := testManager()
	ctx := context.Background()

	version := &capability.Version{
		ID:      "ver-1",
		Version: 1,
		Prompt: capability.Prompt{
			Instructions: "safe content",
		},
		Guardrails: []capability.Guardrail{
			{
				ID:    "gr-1",
				Name:  "unknown-rule",
				Phase: capability.GuardrailPhasePre,
				Config: map[string]any{
					"max_prompt_length": float64(1000),
				},
			},
		},
	}

	err := mgr.CheckVersion(ctx, version)
	if err != nil {
		t.Fatalf("expected no error for rule not in manager, got: %v", err)
	}
}

func TestCheckVersion_TemplateUsedInsteadOfInstructions(t *testing.T) {
	mgr := testManager()
	ctx := context.Background()

	version := &capability.Version{
		ID:      "ver-1",
		Version: 1,
		Prompt: capability.Prompt{
			Instructions: "short",
			Template:     "this is a very long template content that exceeds the limit",
		},
		Guardrails: []capability.Guardrail{
			{
				ID:    "gr-1",
				Name:  "length-check",
				Phase: capability.GuardrailPhasePre,
				Config: map[string]any{
					"max_prompt_length": float64(20),
				},
			},
		},
	}

	err := mgr.CheckVersion(ctx, version)
	if err == nil {
		t.Fatal("expected error when template exceeds max length")
	}
}

func TestContains_EmptySubstr(t *testing.T) {
	if !contains("anything", "") {
		t.Fatal("expected contains to return true for empty substr")
	}
}

func TestContains_SubstrNotFound(t *testing.T) {
	if contains("abc", "xyz") {
		t.Fatal("expected contains to return false for missing substr")
	}
}

func TestContains_SubstrFound(t *testing.T) {
	if !contains("hello world", "world") {
		t.Fatal("expected contains to return true for found substr")
	}
}

func TestCheckVersion_NonStringRestrictedTerm(t *testing.T) {
	mgr := testManager()
	ctx := context.Background()

	version := &capability.Version{
		ID:      "ver-1",
		Version: 1,
		Prompt: capability.Prompt{
			Instructions: "safe content with a number 42",
		},
		Guardrails: []capability.Guardrail{
			{
				ID:    "gr-1",
				Name:  "term-check",
				Phase: capability.GuardrailPhasePre,
				Config: map[string]any{
					"restricted_terms": []any{42},
				},
			},
		},
	}

	err := mgr.CheckVersion(ctx, version)
	if err != nil {
		t.Fatalf("expected no error for non-string restricted term, got: %v", err)
	}
}

func TestCheckVersion_PreGuardrailRestrictedTerms(t *testing.T) {
	mgr := testManager()
	ctx := context.Background()

	version := &capability.Version{
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
