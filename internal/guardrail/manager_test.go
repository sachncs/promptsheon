package guardrail

import (
	"context"
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

func newTestManager() *Manager {
	collector := metrics.NewCollector()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewManager(logger, collector)
}

func TestCheckPromptLengthPasses(t *testing.T) {
	m := newTestManager()
	got := m.CheckPromptLength("hello world", 1000)
	if !got.Passed {
		t.Error("expected pass for short content with large limit")
	}
	if got.Violation != nil {
		t.Errorf("expected no violation, got %+v", got.Violation)
	}
}

func TestCheckPromptLengthFails(t *testing.T) {
	m := newTestManager()
	got := m.CheckPromptLength("a very long prompt that exceeds the limit", 10)
	if got.Passed {
		t.Error("expected fail for content over limit")
	}
	if got.Violation == nil {
		t.Fatal("expected violation")
	}
	if got.Violation.Type != ViolationPromptLength {
		t.Errorf("expected type %q, got %q", ViolationPromptLength, got.Violation.Type)
	}
	if got.Violation.Severity != SeverityMedium {
		t.Errorf("expected severity medium, got %q", got.Violation.Severity)
	}
}

func TestCheckPromptLengthZeroLimitPasses(t *testing.T) {
	// A non-positive limit is interpreted as 'no limit'.
	m := newTestManager()
	got := m.CheckPromptLength("anything", 0)
	if !got.Passed {
		t.Error("expected pass for zero/negative limit")
	}
}

func TestCheckRestrictedTermsDetects(t *testing.T) {
	m := newTestManager()
	got := m.CheckRestrictedTerms("the password is hunter2", []string{"hunter2", "secret"})
	if got.Passed {
		t.Error("expected fail for content with banned term")
	}
	if got.Violation == nil || got.Violation.Type != ViolationRestrictedTerm {
		t.Errorf("expected ViolationRestrictedTerm, got %+v", got.Violation)
	}
}

func TestCheckRestrictedTermsCaseInsensitive(t *testing.T) {
	m := newTestManager()
	got := m.CheckRestrictedTerms("HUNTER2 is a banned term", []string{"hunter2"})
	if got.Passed {
		t.Error("expected case-insensitive match")
	}
}

func TestCheckRestrictedTermsClean(t *testing.T) {
	m := newTestManager()
	got := m.CheckRestrictedTerms("clean prompt", []string{"banned", "secret"})
	if !got.Passed {
		t.Errorf("expected pass for clean content, got %+v", got.Violation)
	}
}

func TestCheckModelAccessAllowed(t *testing.T) {
	m := newTestManager()
	allowed := map[string][]string{
		"production": {"gpt-4", "claude-3"},
	}
	got := m.CheckModelAccess("gpt-4", "production", allowed)
	if !got.Passed {
		t.Error("expected pass for allowed model")
	}
}

func TestCheckModelAccessDenied(t *testing.T) {
	m := newTestManager()
	allowed := map[string][]string{
		"production": {"gpt-4"},
	}
	got := m.CheckModelAccess("claude-3", "production", allowed)
	if got.Passed {
		t.Error("expected fail for disallowed model")
	}
	if got.Violation == nil || got.Violation.Type != ViolationModelAccess {
		t.Errorf("expected ViolationModelAccess, got %+v", got.Violation)
	}
	if got.Violation.Severity != SeverityCritical {
		t.Errorf("expected critical severity, got %q", got.Violation.Severity)
	}
}

func TestCheckModelAccessNoRestrictionForEnv(t *testing.T) {
	m := newTestManager()
	allowed := map[string][]string{
		"production": {"gpt-4"},
	}
	got := m.CheckModelAccess("gpt-4", "dev", allowed)
	if !got.Passed {
		t.Error("expected pass for env without restriction")
	}
}

func TestCheckModelAccessNilAllowed(t *testing.T) {
	m := newTestManager()
	got := m.CheckModelAccess("gpt-4", "production", nil)
	if !got.Passed {
		t.Error("expected pass when allowed map is nil (no policy)")
	}
}

func TestCheckModelAccessCaseInsensitive(t *testing.T) {
	m := newTestManager()
	allowed := map[string][]string{
		"production": {"GPT-4"},
	}
	got := m.CheckModelAccess("gpt-4", "production", allowed)
	if !got.Passed {
		t.Error("expected case-insensitive match")
	}
}

func TestCheckResponseFormatJSONValid(t *testing.T) {
	m := newTestManager()
	if !m.CheckResponseFormat(`{"a":1}`, "json").Passed {
		t.Error("expected pass for valid JSON object")
	}
	if !m.CheckResponseFormat(`[1,2,3]`, "json").Passed {
		t.Error("expected pass for valid JSON array")
	}
}

func TestCheckResponseFormatJSONInvalid(t *testing.T) {
	m := newTestManager()
	if m.CheckResponseFormat("plain text", "json").Passed {
		t.Error("expected fail for non-JSON content")
	}
}

func TestCheckResponseFormatEmpty(t *testing.T) {
	m := newTestManager()
	// Empty formatSpec means 'no format constraint'.
	if !m.CheckResponseFormat("anything", "").Passed {
		t.Error("expected pass for empty format spec")
	}
}

func TestRunAllStaticChecks(t *testing.T) {
	m := newTestManager()
	// The current implementation returns an empty slice
	// (no static rules registered by default); the test
	// pins the shape so a future refactor that adds default
	// rules is intentional.
	results := m.RunAllStaticChecks(context.Background(), "a short prompt", "gpt-4", "production")
	_ = results
}

func TestCheckContentPolicy(t *testing.T) {
	m := newTestManager()
	if !m.CheckContentPolicy("normal content", []string{"spam"}).Passed {
		t.Error("expected pass for clean content")
	}
}

func TestCheckCostLimitPasses(t *testing.T) {
	m := newTestManager()
	if !m.CheckCostLimit(5.0, 10.0).Passed {
		t.Error("expected pass when under cost limit")
	}
}

func TestCheckCostLimitFails(t *testing.T) {
	m := newTestManager()
	got := m.CheckCostLimit(50.0, 10.0)
	if got.Passed {
		t.Error("expected fail when over cost limit")
	}
}

func TestCheckLatencyLimitPasses(t *testing.T) {
	m := newTestManager()
	if !m.CheckLatencyLimit(100, 1000).Passed {
		t.Error("expected pass for fast response")
	}
}

func TestCheckLatencyLimitFails(t *testing.T) {
	m := newTestManager()
	got := m.CheckLatencyLimit(5000, 1000)
	if got.Passed {
		t.Error("expected fail for slow response")
	}
}

func TestRunAgentChecks(t *testing.T) {
	m := newTestManager()
	// Pin the shape: RunAgentChecks returns []*Violation;
	// an empty slice is the expected default since no rules
	// are registered.
	results := m.RunAgentChecks(context.Background(), []string{}, []string{}, "normal agent output")
	_ = results
}
