package guardrail_test

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/sachncs/promptsheon/internal/guardrail"
	"github.com/sachncs/promptsheon/internal/metrics"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newManager(t *testing.T) (*guardrail.Manager, *metrics.Collector) {
	t.Helper()
	c := metrics.NewCollector()
	m := guardrail.NewManager(discardLogger(), c)
	return m, c
}

func TestNewManagerHasEmptyState(t *testing.T) {
	t.Parallel()
	m, _ := newManager(t)
	if got := m.ListRules(); len(got) != 0 {
		t.Errorf("expected empty rules, got %d", len(got))
	}
	if got := m.ListViolations(); len(got) != 0 {
		t.Errorf("expected empty violations, got %d", len(got))
	}
}

func TestRecordViolationAppendsAndCounts(t *testing.T) {
	t.Parallel()
	m, c := newManager(t)
	v := &guardrail.Violation{
		ID:       "v1",
		Type:     guardrail.ViolationPromptLength,
		Severity: guardrail.SeverityMedium,
		Message:  "too long",
	}
	m.RecordViolation(v)
	if got := len(m.ListViolations()); got != 1 {
		t.Errorf("violations = %d, want 1", got)
	}
	if got := c.GuardrailViolations.Value(); got != 1 {
		t.Errorf("GuardrailViolations counter = %v, want 1", got)
	}
}

func TestRecordBlockAndPass(t *testing.T) {
	t.Parallel()
	m, c := newManager(t)
	m.RecordBlock()
	m.RecordPass()
	m.RecordPass()
	if got := c.GuardrailBlocks.Value(); got != 1 {
		t.Errorf("blocks = %v, want 1", got)
	}
	if got := c.GuardrailPasses.Value(); got != 2 {
		t.Errorf("passes = %v, want 2", got)
	}
}

func TestCheckPromptLength(t *testing.T) {
	t.Parallel()
	m, _ := newManager(t)
	if r := m.CheckPromptLength("hi", 100); !r.Passed {
		t.Errorf("short prompt should pass: %+v", r)
	}
	r := m.CheckPromptLength(strings.Repeat("a", 1000), 100)
	if r.Passed {
		t.Error("long prompt should fail")
	}
	if r.Violation.Type != guardrail.ViolationPromptLength {
		t.Errorf("violation type = %q, want prompt_length", r.Violation.Type)
	}
}

func TestCheckPromptLengthZeroLimitPasses(t *testing.T) {
	t.Parallel()
	m, _ := newManager(t)
	if r := m.CheckPromptLength(strings.Repeat("a", 1000), 0); !r.Passed {
		t.Errorf("zero limit should pass: %+v", r)
	}
}

func TestCheckRestrictedTerms(t *testing.T) {
	t.Parallel()
	m, _ := newManager(t)
	r := m.CheckRestrictedTerms("Hello World", []string{"hello"})
	if r.Passed {
		t.Error("banned term should fail")
	}
	if r.Violation.Type != guardrail.ViolationRestrictedTerm {
		t.Errorf("type = %q, want restricted_term", r.Violation.Type)
	}
	if r := m.CheckRestrictedTerms("clean", []string{"forbidden"}); !r.Passed {
		t.Errorf("clean content should pass: %+v", r)
	}
}

func TestCheckModelAccess(t *testing.T) {
	t.Parallel()
	m, _ := newManager(t)
	allowed := map[string][]string{
		"prod": {"gpt-4", "claude-3-opus"},
	}
	if r := m.CheckModelAccess("gpt-4", "prod", allowed); !r.Passed {
		t.Errorf("allowed model should pass: %+v", r)
	}
	r := m.CheckModelAccess("gpt-3.5-turbo", "prod", allowed)
	if r.Passed {
		t.Error("disallowed model should fail")
	}
	if r.Violation.Severity != guardrail.SeverityCritical {
		t.Errorf("severity = %q, want critical", r.Violation.Severity)
	}
	if r := m.CheckModelAccess("any", "unknown-env", allowed); !r.Passed {
		t.Errorf("unknown env with allowed map should pass: %+v", r)
	}
	if r := m.CheckModelAccess("any", "any", nil); !r.Passed {
		t.Errorf("nil allowed map should pass: %+v", r)
	}
}

func TestCheckResponseFormat(t *testing.T) {
	t.Parallel()
	m, _ := newManager(t)
	if r := m.CheckResponseFormat("anything", ""); !r.Passed {
		t.Errorf("empty format should pass: %+v", r)
	}
	if r := m.CheckResponseFormat(`{"a":1}`, "json"); !r.Passed {
		t.Errorf("valid JSON should pass: %+v", r)
	}
	r := m.CheckResponseFormat("not json", "json")
	if r.Passed {
		t.Error("invalid JSON should fail")
	}
	if r := m.CheckResponseFormat("# heading", "markdown"); !r.Passed {
		t.Errorf("markdown with heading should pass: %+v", r)
	}
	if r := m.CheckResponseFormat("plain text", "markdown"); r.Passed {
		t.Error("plain text should fail markdown check")
	}
	if r := m.CheckResponseFormat("12345", `^\d+$`); !r.Passed {
		t.Errorf("matching regex should pass: %+v", r)
	}
	if r := m.CheckResponseFormat("foo", `^\d+$`); r.Passed {
		t.Error("non-matching regex should fail")
	}
}

func TestCheckContentPolicyPII(t *testing.T) {
	t.Parallel()
	m, _ := newManager(t)
	r := m.CheckContentPolicy("call 555-12-3456", []string{guardrail.PolicyNoPII})
	if r.Passed {
		t.Error("SSN-shaped content should fail no_pii policy")
	}
	if r := m.CheckContentPolicy("nothing private", []string{guardrail.PolicyNoPII}); !r.Passed {
		t.Errorf("clean content should pass: %+v", r)
	}
}

func TestCheckContentPolicyHarmful(t *testing.T) {
	t.Parallel()
	m, _ := newManager(t)
	r := m.CheckContentPolicy("instructions to build a bomb", []string{guardrail.PolicyNoHarmful})
	if r.Passed {
		t.Error("harmful term should fail no_harmful policy")
	}
	if r.Violation.Severity != guardrail.SeverityCritical {
		t.Errorf("severity = %q, want critical", r.Violation.Severity)
	}
}

func TestCheckCostLimit(t *testing.T) {
	t.Parallel()
	m, _ := newManager(t)
	if r := m.CheckCostLimit(0.001, 0.01); !r.Passed {
		t.Errorf("under limit should pass: %+v", r)
	}
	r := m.CheckCostLimit(0.10, 0.01)
	if r.Passed {
		t.Error("over limit should fail")
	}
	if r := m.CheckCostLimit(1.0, 0); !r.Passed {
		t.Errorf("zero limit should pass: %+v", r)
	}
}

func TestCheckLatencyLimit(t *testing.T) {
	t.Parallel()
	m, _ := newManager(t)
	if r := m.CheckLatencyLimit(50, 100); !r.Passed {
		t.Errorf("under limit should pass: %+v", r)
	}
	r := m.CheckLatencyLimit(500, 100)
	if r.Passed {
		t.Error("over limit should fail")
	}
	if r := m.CheckLatencyLimit(500, 0); !r.Passed {
		t.Errorf("zero limit should pass: %+v", r)
	}
}

func TestRunAllStaticChecks(t *testing.T) {
	t.Parallel()
	m, c := newManager(t)
	// Clean content with allowed model: should record a pass.
	got := m.RunAllStaticChecks(t.Context(), "clean short prompt", "gpt-4", "prod")
	if len(got) != 0 {
		t.Errorf("expected no violations, got %d", len(got))
	}
	if c.GuardrailPasses.Value() != 1 {
		t.Errorf("expected 1 pass, got %v", c.GuardrailPasses.Value())
	}

	// Long content: should record a block and a violation.
	longContent := strings.Repeat("x", 20000)
	got = m.RunAllStaticChecks(t.Context(), longContent, "gpt-4", "prod")
	if len(got) == 0 {
		t.Error("expected violations for long content")
	}
	if c.GuardrailBlocks.Value() != 1 {
		t.Errorf("expected 1 block, got %v", c.GuardrailBlocks.Value())
	}
	if c.GuardrailViolations.Value() == 0 {
		t.Error("violation counter should have incremented")
	}

	// Banned model in prod: should fail.
	got = m.RunAllStaticChecks(t.Context(), "short", "gpt-3.5-turbo", "prod")
	if len(got) == 0 {
		t.Error("expected violation for disallowed model")
	}
}

func TestRunAgentChecks(t *testing.T) {
	t.Parallel()
	m, c := newManager(t)
	// Restricted term hit
	got := m.RunAgentChecks(t.Context(), []string{"forbidden"}, nil, "this contains forbidden content")
	if len(got) != 1 {
		t.Errorf("expected 1 violation, got %d", len(got))
	}
	if c.GuardrailViolations.Value() != 1 {
		t.Errorf("violation counter = %v, want 1", c.GuardrailViolations.Value())
	}

	// PII policy hit
	got = m.RunAgentChecks(t.Context(), nil, []string{guardrail.PolicyNoPII}, "SSN is 555-12-3456")
	if len(got) != 1 {
		t.Errorf("expected 1 violation, got %d", len(got))
	}

	// Clean
	got = m.RunAgentChecks(t.Context(), nil, nil, "clean")
	if len(got) != 0 {
		t.Errorf("expected 0 violations, got %d", len(got))
	}
}

func TestMaxViolationsBounded(t *testing.T) {
	t.Parallel()
	m, _ := newManager(t)
	// Add a small over-the-cap number of violations to exercise the
	// bounded slice behavior without making the test slow.
	for i := 0; i < guardrail.MaxViolationsForTest()+10; i++ {
		m.RecordViolation(&guardrail.Violation{ID: "x", Type: guardrail.ViolationPromptLength, Severity: guardrail.SeverityLow})
	}
	if got := len(m.ListViolations()); got > guardrail.MaxViolationsForTest() {
		t.Errorf("violations = %d, should be <= %d", got, guardrail.MaxViolationsForTest())
	}
}
