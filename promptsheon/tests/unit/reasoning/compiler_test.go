//go:build tests_migration


package reasoning_test

import (
	. "github.com/sachncs/promptsheon/promptsheon"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/promptsheon/errs"
)

func catalog() []CapabilityDescriptor {
	return []CapabilityDescriptor{
		{
			ID:         "summarise-doc",
			Name:       "Summarise Document",
			Tags:       []string{"summarise", "text", "fast"},
			TrustScore: 0.9,
			CostUSD:    0.001,
			LatencyMS:  500,
			Inputs:     []string{"text"},
			Outputs:    []string{"summary"},
		},
		{
			ID:         "translate",
			Name:       "Translate Text",
			Tags:       []string{"translate", "language"},
			TrustScore: 0.85,
			CostUSD:    0.002,
			LatencyMS:  800,
			Inputs:     []string{"text", "target_lang"},
			Outputs:    []string{"translated"},
		},
		{
			ID:         "classify",
			Name:       "Classify Intent",
			Tags:       []string{"classify", "fast"},
			TrustScore: 0.95,
			CostUSD:    0.0005,
			LatencyMS:  200,
			Inputs:     []string{"text"},
			Outputs:    []string{"label"},
		},
	}
}

func TestCompileMatchesByGoalToken(t *testing.T) {
	t.Parallel()
	c := NewCompiler(catalog())
	p, err := c.Compile(context.Background(), Intent{
		ID:     "i1",
		Goal:   "summarise this document",
		Inputs: map[string]any{"text": "the quick brown fox"},
	})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(p.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(p.Steps))
	}
	if p.Steps[0].CapabilityID != "summarise-doc" {
		t.Errorf("expected summarise-doc, got %q", p.Steps[0].CapabilityID)
	}
	if p.Steps[0].Inputs["text"] != "intent" {
		t.Errorf("expected text bound to intent, got %q", p.Steps[0].Inputs["text"])
	}
}

func TestCompileNoMatch(t *testing.T) {
	t.Parallel()
	c := NewCompiler(catalog())
	_, err := c.Compile(context.Background(), Intent{
		ID:   "i1",
		Goal: "play chess",
	})
	if !errors.Is(err, errs.ErrReasoningNoMatch) {
		t.Errorf("expected errs.ErrReasoningNoMatch, got %v", err)
	}
}

func TestCompileEmptyGoalRejected(t *testing.T) {
	t.Parallel()
	c := NewCompiler(catalog())
	_, err := c.Compile(context.Background(), Intent{ID: "i1", Goal: "   "})
	if err == nil || !strings.Contains(err.Error(), "empty goal") {
		t.Errorf("expected empty-goal error, got %v", err)
	}
}

func TestCompileRespectsMinTrustScore(t *testing.T) {
	t.Parallel()
	c := NewCompiler(catalog())
	_, err := c.Compile(context.Background(), Intent{
		ID:   "i1",
		Goal: "summarise",
		Constraints: Constraints{
			MinTrustScore: 0.99,
		},
	})
	if !errors.Is(err, errs.ErrReasoningConstraintViolation) {
		t.Errorf("expected errs.ErrReasoningConstraintViolation, got %v", err)
	}
}

func TestCompileRespectsMaxCost(t *testing.T) {
	t.Parallel()
	c := NewCompiler(catalog())
	_, err := c.Compile(context.Background(), Intent{
		ID:   "i1",
		Goal: "summarise",
		Constraints: Constraints{
			MaxCostUSD: 0.0001,
		},
	})
	if !errors.Is(err, errs.ErrReasoningConstraintViolation) {
		t.Errorf("expected errs.ErrReasoningConstraintViolation, got %v", err)
	}
}

func TestCompileRespectsRequiredTags(t *testing.T) {
	t.Parallel()
	c := NewCompiler(catalog())
	_, err := c.Compile(context.Background(), Intent{
		ID:   "i1",
		Goal: "summarise",
		Constraints: Constraints{
			RequiredTags: []string{"nonexistent"},
		},
	})
	// RequiredTags is a hard constraint; when no candidate
	// satisfies it, the compiler reports
	// errs.ErrReasoningConstraintViolation (a constraint rejected every
	// match).
	if !errors.Is(err, errs.ErrReasoningConstraintViolation) {
		t.Errorf("expected errs.ErrReasoningConstraintViolation, got %v", err)
	}
}

func TestCompilePicksCheaperOnTie(t *testing.T) {
	t.Parallel()
	cat := []CapabilityDescriptor{
		{ID: "expensive", Name: "Translate", Tags: []string{"translate"}, CostUSD: 1.0, LatencyMS: 100, Inputs: []string{"text"}},
		{ID: "cheap", Name: "Translate", Tags: []string{"translate"}, CostUSD: 0.01, LatencyMS: 100, Inputs: []string{"text"}},
	}
	c := NewCompiler(cat)
	p, err := c.Compile(context.Background(), Intent{ID: "i", Goal: "translate", Inputs: map[string]any{"text": "x"}})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if p.Steps[0].CapabilityID != "cheap" {
		t.Errorf("expected cheap, got %q", p.Steps[0].CapabilityID)
	}
}

func TestCompilePlanIDAndBudget(t *testing.T) {
	t.Parallel()
	c := NewCompiler(catalog())
	before := time.Now().UTC()
	p, err := c.Compile(context.Background(), Intent{ID: "x", Goal: "summarise", Inputs: map[string]any{"text": "x"}})
	after := time.Now().UTC()
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if p.ID != "plan-x" {
		t.Errorf("plan id: got %q want plan-x", p.ID)
	}
	if p.CreatedAt.Before(before) || p.CreatedAt.After(after) {
		t.Errorf("CreatedAt out of bounds: %v", p.CreatedAt)
	}
	if p.BudgetCost <= 0 {
		t.Errorf("BudgetCost must be positive: %v", p.BudgetCost)
	}
}

func TestCompileContextCancel(t *testing.T) {
	t.Parallel()
	c := NewCompiler(catalog())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Compile(ctx, Intent{ID: "i", Goal: "summarise"})
	if err == nil {
		t.Fatal("expected ctx error")
	}
}
