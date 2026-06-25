package guardrail

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/sachn-cs/promptsheon/internal/metrics"
)

func TestManagerAddRemoveRule(t *testing.T) {
	collector := metrics.NewCollector()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	mgr := NewManager(logger, collector)

	rule := &Rule{
		ID:       "rule-1",
		Name:     "Test Rule",
		Type:     ViolationPromptLength,
		Severity: SeverityMedium,
		Enabled:  true,
	}

	mgr.AddRule(rule)

	got, ok := mgr.GetRule("rule-1")
	if !ok {
		t.Fatal("expected rule to exist")
	}
	if got.Name != "Test Rule" {
		t.Errorf("Name = %q, want %q", got.Name, "Test Rule")
	}

	mgr.RemoveRule("rule-1")
	_, ok = mgr.GetRule("rule-1")
	if ok {
		t.Fatal("expected rule to be removed")
	}
}

func TestManagerListRules(t *testing.T) {
	collector := metrics.NewCollector()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	mgr := NewManager(logger, collector)

	mgr.AddRule(&Rule{ID: "rule-1", Name: "Rule 1"})
	mgr.AddRule(&Rule{ID: "rule-2", Name: "Rule 2"})

	rules := mgr.ListRules()
	if len(rules) != 2 {
		t.Errorf("ListRules() returned %d rules, want 2", len(rules))
	}
}

func TestManagerRecordViolation(t *testing.T) {
	collector := metrics.NewCollector()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	mgr := NewManager(logger, collector)

	v := &Violation{
		ID:           "violation-1",
		RuleID:       "rule-1",
		Type:         ViolationPromptLength,
		Severity:     SeverityHigh,
		ResourceType: "prompt",
		ResourceID:   "prompt-1",
		Message:      "Prompt too long",
		Timestamp:    time.Now(),
	}

	mgr.RecordViolation(v)

	violations := mgr.ListViolations()
	if len(violations) != 1 {
		t.Errorf("ListViolations() returned %d violations, want 1", len(violations))
	}

	if collector.GuardrailViolations.Value() != 1 {
		t.Errorf("GuardrailViolations = %v, want 1", collector.GuardrailViolations.Value())
	}
}

func TestManagerRecordBlockAndPass(t *testing.T) {
	collector := metrics.NewCollector()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	mgr := NewManager(logger, collector)

	mgr.RecordBlock()
	mgr.RecordBlock()
	mgr.RecordPass()

	if collector.GuardrailBlocks.Value() != 2 {
		t.Errorf("GuardrailBlocks = %v, want 2", collector.GuardrailBlocks.Value())
	}
	if collector.GuardrailPasses.Value() != 1 {
		t.Errorf("GuardrailPasses = %v, want 1", collector.GuardrailPasses.Value())
	}
}

func TestManagerViolationCap(t *testing.T) {
	collector := metrics.NewCollector()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	mgr := NewManager(logger, collector)

	// Add more than maxViolations
	for i := 0; i < maxViolations+10; i++ {
		mgr.RecordViolation(&Violation{
			ID:        "v-" + string(rune(i)),
			Timestamp: time.Now(),
		})
	}

	violations := mgr.ListViolations()
	if len(violations) > maxViolations {
		t.Errorf("violations = %d, want at most %d", len(violations), maxViolations)
	}
}

func TestManagerNilMetrics(t *testing.T) {
	// Test that nil metrics doesn't panic
	mgr := NewManager(nil, nil)

	mgr.RecordViolation(&Violation{ID: "v-1"})
	mgr.RecordBlock()
	mgr.RecordPass()

	violations := mgr.ListViolations()
	if len(violations) != 1 {
		t.Errorf("ListViolations() returned %d violations, want 1", len(violations))
	}
}

func TestViolationResultPassed(t *testing.T) {
	result := &ViolationResult{Passed: true}
	if !result.Passed {
		t.Error("expected Passed to be true")
	}
	if result.Violation != nil {
		t.Error("expected Violation to be nil when passed")
	}
}

func TestViolationResultFailed(t *testing.T) {
	v := &Violation{ID: "v-1", Message: "test violation"}
	result := &ViolationResult{Passed: false, Violation: v}

	if result.Passed {
		t.Error("expected Passed to be false")
	}
	if result.Violation == nil {
		t.Error("expected Violation to be set when failed")
	}
}
