package guardrail

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

func testManagerWithoutMetrics() *Manager {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	return NewManager(logger, nil)
}

func TestNewManager(t *testing.T) {
	m := testManager()
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestListRules_Empty(t *testing.T) {
	m := testManager()
	rules := m.ListRules()
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules, got %d", len(rules))
	}
}

func TestListRules_Multiple(t *testing.T) {
	m := testManager()
	m.mu.Lock()
	m.rules["r1"] = &Rule{ID: "r1", Name: "rule1", Enabled: true}
	m.rules["r2"] = &Rule{ID: "r2", Name: "rule2", Enabled: false}
	m.mu.Unlock()

	rules := m.ListRules()
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
}

func TestListViolations_Empty(t *testing.T) {
	m := testManager()
	vs := m.ListViolations()
	if len(vs) != 0 {
		t.Fatalf("expected 0 violations, got %d", len(vs))
	}
}

func TestListViolations_AfterRecord(t *testing.T) {
	m := testManager()
	v := &Violation{ID: "v-1", Type: ViolationPromptLength, Severity: SeverityMedium, Timestamp: time.Now()}
	m.RecordViolation(v)

	vs := m.ListViolations()
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(vs))
	}
	if vs[0].ID != "v-1" {
		t.Fatalf("expected v-1, got %s", vs[0].ID)
	}
}

func TestRecordViolation_MaxViolationsOverflow(t *testing.T) {
	m := testManager()
	for i := 0; i < maxViolations+10; i++ {
		m.RecordViolation(&Violation{ID: "v", Timestamp: time.Now()})
	}
	vs := m.ListViolations()
	if len(vs) != maxViolations {
		t.Fatalf("expected %d violations, got %d", maxViolations, len(vs))
	}
}

func TestRecordViolation_NilMetrics(t *testing.T) {
	m := testManagerWithoutMetrics()
	v := &Violation{ID: "v-1", Type: ViolationPromptLength, Severity: SeverityMedium, Timestamp: time.Now()}
	m.RecordViolation(v)
	vs := m.ListViolations()
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(vs))
	}
}

func TestRecordBlock_WithMetrics(t *testing.T) {
	m := testManager()
	before := m.metrics.GuardrailBlocks.Value()
	m.RecordBlock()
	after := m.metrics.GuardrailBlocks.Value()
	if after-before != 1 {
		t.Fatalf("expected GuardrailBlocks to increase by 1, got %f", after-before)
	}
}

func TestRecordBlock_NilMetrics(_ *testing.T) {
	m := testManagerWithoutMetrics()
	m.RecordBlock()
}

func TestRecordPass_WithMetrics(t *testing.T) {
	m := testManager()
	before := m.metrics.GuardrailPasses.Value()
	m.RecordPass()
	after := m.metrics.GuardrailPasses.Value()
	if after-before != 1 {
		t.Fatalf("expected GuardrailPasses to increase by 1, got %f", after-before)
	}
}

func TestRecordPass_NilMetrics(_ *testing.T) {
	m := testManagerWithoutMetrics()
	m.RecordPass()
}

func TestCheckPromptLength_ZeroMaxLength(t *testing.T) {
	m := testManager()
	result := m.CheckPromptLength("some content", 0)
	if !result.Passed {
		t.Fatal("expected passed for zero max length")
	}
}

func TestCheckPromptLength_NegativeMaxLength(t *testing.T) {
	m := testManager()
	result := m.CheckPromptLength("some content", -1)
	if !result.Passed {
		t.Fatal("expected passed for negative max length")
	}
}

func TestCheckPromptLength_UnderLimit(t *testing.T) {
	m := testManager()
	result := m.CheckPromptLength("short", 100)
	if !result.Passed {
		t.Fatal("expected passed for content under limit")
	}
}

func TestCheckPromptLength_OverLimit(t *testing.T) {
	m := testManager()
	content := make([]byte, 200)
	for i := range content {
		content[i] = 'a'
	}
	result := m.CheckPromptLength(string(content), 100)
	if result.Passed {
		t.Fatal("expected violation for content over limit")
	}
	if result.Violation.Type != ViolationPromptLength {
		t.Fatalf("expected ViolationPromptLength, got %s", result.Violation.Type)
	}
	if result.Violation.Severity != SeverityMedium {
		t.Fatalf("expected SeverityMedium, got %s", result.Violation.Severity)
	}
}

func TestCheckRestrictedTerms_NoBannedTerms(t *testing.T) {
	m := testManager()
	result := m.CheckRestrictedTerms("safe content", nil)
	if !result.Passed {
		t.Fatal("expected passed with no banned terms")
	}
}

func TestCheckRestrictedTerms_EmptyBannedTerms(t *testing.T) {
	m := testManager()
	result := m.CheckRestrictedTerms("safe content", []string{})
	if !result.Passed {
		t.Fatal("expected passed with empty banned terms")
	}
}

func TestCheckRestrictedTerms_TermFound(t *testing.T) {
	m := testManager()
	result := m.CheckRestrictedTerms("this contains secret_password here", []string{"secret_password"})
	if result.Passed {
		t.Fatal("expected violation for restricted term")
	}
	if result.Violation.Type != ViolationRestrictedTerm {
		t.Fatalf("expected ViolationRestrictedTerm, got %s", result.Violation.Type)
	}
	if result.Violation.Severity != SeverityHigh {
		t.Fatalf("expected SeverityHigh, got %s", result.Violation.Severity)
	}
}

func TestCheckRestrictedTerms_CaseInsensitive(t *testing.T) {
	m := testManager()
	result := m.CheckRestrictedTerms("BANNED_TOPIC_1 is bad", []string{"banned_topic_1"})
	if result.Passed {
		t.Fatal("expected violation for case-insensitive restricted term")
	}
}

func TestCheckRestrictedTerms_TermNotFound(t *testing.T) {
	m := testManager()
	result := m.CheckRestrictedTerms("safe content", []string{"banned_topic_1"})
	if !result.Passed {
		t.Fatal("expected passed when term not present")
	}
}

func TestCheckModelAccess_NilAllowed(t *testing.T) {
	m := testManager()
	result := m.CheckModelAccess("gpt-4", "prod", nil)
	if !result.Passed {
		t.Fatal("expected passed for nil allowed models")
	}
}

func TestCheckModelAccess_EnvNotConfigured(t *testing.T) {
	m := testManager()
	result := m.CheckModelAccess("gpt-4", "unknown", map[string][]string{"prod": {"gpt-4"}})
	if !result.Passed {
		t.Fatal("expected passed for unconfigured environment")
	}
}

func TestCheckModelAccess_ModelAllowed(t *testing.T) {
	m := testManager()
	result := m.CheckModelAccess("gpt-4", "prod", map[string][]string{"prod": {"gpt-4", "claude-3"}})
	if !result.Passed {
		t.Fatal("expected passed for allowed model")
	}
}

func TestCheckModelAccess_ModelAllowedCaseInsensitive(t *testing.T) {
	m := testManager()
	result := m.CheckModelAccess("GPT-4", "prod", map[string][]string{"prod": {"gpt-4"}})
	if !result.Passed {
		t.Fatal("expected passed for case-insensitive model match")
	}
}

func TestCheckModelAccess_ModelNotAllowed(t *testing.T) {
	m := testManager()
	result := m.CheckModelAccess("llama-3", "prod", map[string][]string{"prod": {"gpt-4", "claude-3"}})
	if result.Passed {
		t.Fatal("expected violation for disallowed model")
	}
	if result.Violation.Type != ViolationModelAccess {
		t.Fatalf("expected ViolationModelAccess, got %s", result.Violation.Type)
	}
	if result.Violation.Severity != SeverityCritical {
		t.Fatalf("expected SeverityCritical, got %s", result.Violation.Severity)
	}
}

func TestCheckResponseFormat_EmptyFormat(t *testing.T) {
	m := testManager()
	result := m.CheckResponseFormat("any response", "")
	if !result.Passed {
		t.Fatal("expected passed for empty format")
	}
}

func TestCheckResponseFormat_ValidJSON(t *testing.T) {
	m := testManager()
	result := m.CheckResponseFormat(`{"key": "value"}`, "json")
	if !result.Passed {
		t.Fatal("expected passed for valid JSON")
	}
}

func TestCheckResponseFormat_ValidJSONArray(t *testing.T) {
	m := testManager()
	result := m.CheckResponseFormat(`[1, 2, 3]`, "json")
	if !result.Passed {
		t.Fatal("expected passed for valid JSON array")
	}
}

func TestCheckResponseFormat_InvalidJSON(t *testing.T) {
	m := testManager()
	result := m.CheckResponseFormat("just text", "json")
	if result.Passed {
		t.Fatal("expected violation for invalid JSON")
	}
	if result.Violation.Type != ViolationFormatInvalid {
		t.Fatalf("expected ViolationFormatInvalid, got %s", result.Violation.Type)
	}
}

func TestCheckResponseFormat_ValidMarkdown(t *testing.T) {
	m := testManager()
	result := m.CheckResponseFormat("# Heading\n\nSome *bold* text", "markdown")
	if !result.Passed {
		t.Fatal("expected passed for valid markdown")
	}
}

func TestCheckResponseFormat_ValidMarkdownWithDash(t *testing.T) {
	m := testManager()
	result := m.CheckResponseFormat("- list item", "markdown")
	if !result.Passed {
		t.Fatal("expected passed for markdown with dash")
	}
}

func TestCheckResponseFormat_InvalidMarkdown(t *testing.T) {
	m := testManager()
	result := m.CheckResponseFormat("plain text without markers", "markdown")
	if result.Passed {
		t.Fatal("expected violation for invalid markdown")
	}
	if result.Violation.Severity != SeverityLow {
		t.Fatalf("expected SeverityLow, got %s", result.Violation.Severity)
	}
}

func TestCheckResponseFormat_RegexMatch(t *testing.T) {
	m := testManager()
	result := m.CheckResponseFormat("abc123", "^[a-z]+[0-9]+$")
	if !result.Passed {
		t.Fatal("expected passed for matching regex")
	}
}

func TestCheckResponseFormat_RegexNoMatch(t *testing.T) {
	m := testManager()
	result := m.CheckResponseFormat("123abc", "^[a-z]+[0-9]+$")
	if result.Passed {
		t.Fatal("expected violation for non-matching regex")
	}
}

func TestCheckResponseFormat_InvalidRegex(t *testing.T) {
	m := testManager()
	result := m.CheckResponseFormat("content", "[invalid")
	if result.Passed {
		t.Fatal("expected violation for invalid regex")
	}
}

func TestCheckContentPolicy_EmptyPolicies(t *testing.T) {
	m := testManager()
	result := m.CheckContentPolicy("some content", nil)
	if !result.Passed {
		t.Fatal("expected passed for empty policies")
	}
}

func TestCheckContentPolicy_NoPII_SSN(t *testing.T) {
	m := testManager()
	result := m.CheckContentPolicy("my ssn is 123-45-6789", []string{"no_pii"})
	if result.Passed {
		t.Fatal("expected violation for SSN")
	}
	if result.Violation.Type != ViolationContentPolicy {
		t.Fatalf("expected ViolationContentPolicy, got %s", result.Violation.Type)
	}
}

func TestCheckContentPolicy_NoPII_CreditCard(t *testing.T) {
	m := testManager()
	result := m.CheckContentPolicy("card: 1234567890123456", []string{"no_pii"})
	if result.Passed {
		t.Fatal("expected violation for credit card")
	}
}

func TestCheckContentPolicy_NoPII_Email(t *testing.T) {
	m := testManager()
	result := m.CheckContentPolicy("email: user@example.com", []string{"no_pii"})
	if result.Passed {
		t.Fatal("expected violation for email")
	}
}

func TestCheckContentPolicy_NoPII_Clean(t *testing.T) {
	m := testManager()
	result := m.CheckContentPolicy("clean content without PII", []string{"no_pii"})
	if !result.Passed {
		t.Fatal("expected passed for no PII")
	}
}

func TestCheckContentPolicy_NoHarmful_Found(t *testing.T) {
	m := testManager()
	result := m.CheckContentPolicy("this mentions suicide", []string{"no_harmful"})
	if result.Passed {
		t.Fatal("expected violation for harmful content")
	}
	if result.Violation.Type != ViolationContentPolicy {
		t.Fatalf("expected ViolationContentPolicy, got %s", result.Violation.Type)
	}
	if result.Violation.Severity != SeverityCritical {
		t.Fatalf("expected SeverityCritical, got %s", result.Violation.Severity)
	}
}

func TestCheckContentPolicy_NoHarmful_Clean(t *testing.T) {
	m := testManager()
	result := m.CheckContentPolicy("clean and safe content", []string{"no_harmful"})
	if !result.Passed {
		t.Fatal("expected passed for no harmful content")
	}
}

func TestCheckContentPolicy_MultiplePolicies(t *testing.T) {
	m := testManager()
	result := m.CheckContentPolicy("clean content", []string{"no_pii", "no_harmful"})
	if !result.Passed {
		t.Fatal("expected passed for clean content with multiple policies")
	}
}

func TestCheckCostLimit_ZeroLimit(t *testing.T) {
	m := testManager()
	result := m.CheckCostLimit(100, 0)
	if !result.Passed {
		t.Fatal("expected passed for zero limit")
	}
}

func TestCheckCostLimit_NegativeLimit(t *testing.T) {
	m := testManager()
	result := m.CheckCostLimit(100, -1)
	if !result.Passed {
		t.Fatal("expected passed for negative limit")
	}
}

func TestCheckCostLimit_UnderLimit(t *testing.T) {
	m := testManager()
	result := m.CheckCostLimit(5.0, 10.0)
	if !result.Passed {
		t.Fatal("expected passed when cost under limit")
	}
}

func TestCheckCostLimit_EqualLimit(t *testing.T) {
	m := testManager()
	result := m.CheckCostLimit(10.0, 10.0)
	if !result.Passed {
		t.Fatal("expected passed when cost equals limit")
	}
}

func TestCheckCostLimit_OverLimit(t *testing.T) {
	m := testManager()
	result := m.CheckCostLimit(15.0, 10.0)
	if result.Passed {
		t.Fatal("expected violation when cost over limit")
	}
	if result.Violation.Type != ViolationCostLimit {
		t.Fatalf("expected ViolationCostLimit, got %s", result.Violation.Type)
	}
}

func TestCheckLatencyLimit_ZeroLimit(t *testing.T) {
	m := testManager()
	result := m.CheckLatencyLimit(100, 0)
	if !result.Passed {
		t.Fatal("expected passed for zero limit")
	}
}

func TestCheckLatencyLimit_NegativeLimit(t *testing.T) {
	m := testManager()
	result := m.CheckLatencyLimit(100, -1)
	if !result.Passed {
		t.Fatal("expected passed for negative limit")
	}
}

func TestCheckLatencyLimit_UnderLimit(t *testing.T) {
	m := testManager()
	result := m.CheckLatencyLimit(50, 100)
	if !result.Passed {
		t.Fatal("expected passed when latency under limit")
	}
}

func TestCheckLatencyLimit_EqualLimit(t *testing.T) {
	m := testManager()
	result := m.CheckLatencyLimit(100, 100)
	if !result.Passed {
		t.Fatal("expected passed when latency equals limit")
	}
}

func TestCheckLatencyLimit_OverLimit(t *testing.T) {
	m := testManager()
	result := m.CheckLatencyLimit(200, 100)
	if result.Passed {
		t.Fatal("expected violation when latency over limit")
	}
	if result.Violation.Type != ViolationLatencyLimit {
		t.Fatalf("expected ViolationLatencyLimit, got %s", result.Violation.Type)
	}
}

func TestRunAllStaticChecks_Passes(t *testing.T) {
	m := testManager()
	ctx := context.Background()
	violations := m.RunAllStaticChecks(ctx, "short safe content", "gpt-4", "prod")
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d", len(violations))
	}
}

func TestRunAllStaticChecks_PromptLengthViolation(t *testing.T) {
	m := testManager()
	ctx := context.Background()
	content := make([]byte, 20000)
	for i := range content {
		content[i] = 'a'
	}
	violations := m.RunAllStaticChecks(ctx, string(content), "gpt-4", "prod")
	if len(violations) == 0 {
		t.Fatal("expected violations for long content")
	}
}

func TestRunAllStaticChecks_RestrictedTerm(t *testing.T) {
	m := testManager()
	ctx := context.Background()
	violations := m.RunAllStaticChecks(ctx, "this has banned_topic_1 in it", "gpt-4", "prod")
	if len(violations) == 0 {
		t.Fatal("expected violation for restricted term")
	}
}

func TestRunAllStaticChecks_ModelNotAllowed(t *testing.T) {
	m := testManager()
	ctx := context.Background()
	violations := m.RunAllStaticChecks(ctx, "safe content", "llama-3", "prod")
	if len(violations) == 0 {
		t.Fatal("expected violation for disallowed model")
	}
}

func TestRunAllStaticChecks_MultipleViolations(t *testing.T) {
	m := testManager()
	ctx := context.Background()
	content := make([]byte, 20000)
	for i := range content {
		content[i] = 'a'
	}
	violations := m.RunAllStaticChecks(ctx, string(content)+" banned_topic_1", "llama-3", "prod")
	if len(violations) != 3 {
		t.Fatalf("expected 3 violations (length, term, model), got %d", len(violations))
	}
}

func TestRunAllStaticChecks_CallsRecordPassOnSuccess(t *testing.T) {
	m := testManager()
	ctx := context.Background()
	before := m.metrics.GuardrailPasses.Value()
	m.RunAllStaticChecks(ctx, "safe", "gpt-4", "prod")
	after := m.metrics.GuardrailPasses.Value()
	if after-before != 1 {
		t.Fatalf("expected GuardrailPasses to increment by 1, got %f", after-before)
	}
}

func TestRunAllStaticChecks_CallsRecordBlockOnViolation(t *testing.T) {
	m := testManager()
	ctx := context.Background()
	before := m.metrics.GuardrailBlocks.Value()
	m.RunAllStaticChecks(ctx, "banned_topic_1", "gpt-4", "prod")
	after := m.metrics.GuardrailBlocks.Value()
	if after-before != 1 {
		t.Fatalf("expected GuardrailBlocks to increment by 1, got %f", after-before)
	}
}

func TestRunAllStaticChecks_CallsRecordViolation(t *testing.T) {
	m := testManager()
	ctx := context.Background()
	before := m.metrics.GuardrailViolations.Value()
	m.RunAllStaticChecks(ctx, "banned_topic_1", "gpt-4", "prod")
	after := m.metrics.GuardrailViolations.Value()
	if after-before != 1 {
		t.Fatalf("expected GuardrailViolations to increment by 1, got %f", after-before)
	}
}

func TestRunAgentChecks_NoRestrictions(t *testing.T) {
	m := testManager()
	ctx := context.Background()
	violations := m.RunAgentChecks(ctx, nil, nil, "safe content")
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d", len(violations))
	}
}

func TestRunAgentChecks_EmptyRestrictions(t *testing.T) {
	m := testManager()
	ctx := context.Background()
	violations := m.RunAgentChecks(ctx, []string{}, nil, "safe content")
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d", len(violations))
	}
}

func TestRunAgentChecks_RestrictedTermFound(t *testing.T) {
	m := testManager()
	ctx := context.Background()
	violations := m.RunAgentChecks(ctx, []string{"secret_word"}, nil, "this contains secret_word")
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Type != ViolationRestrictedTerm {
		t.Fatalf("expected ViolationRestrictedTerm, got %s", violations[0].Type)
	}
}

func TestRunAgentChecks_RestrictedTermCaseInsensitive(t *testing.T) {
	m := testManager()
	ctx := context.Background()
	violations := m.RunAgentChecks(ctx, []string{"SECRET_WORD"}, nil, "this contains secret_word")
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
}

func TestRunAgentChecks_MultipleRestrictedTerms(t *testing.T) {
	m := testManager()
	ctx := context.Background()
	violations := m.RunAgentChecks(ctx, []string{"term1", "term2", "term3"}, nil, "has term1 and term3")
	if len(violations) != 2 {
		t.Fatalf("expected 2 violations, got %d", len(violations))
	}
}

func TestRunAgentChecks_ContentPolicyViolation(t *testing.T) {
	m := testManager()
	ctx := context.Background()
	violations := m.RunAgentChecks(ctx, nil, []string{"no_harmful"}, "this mentions suicide")
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Type != ViolationContentPolicy {
		t.Fatalf("expected ViolationContentPolicy, got %s", violations[0].Type)
	}
}

func TestRunAgentChecks_ContentPolicyPasses(t *testing.T) {
	m := testManager()
	ctx := context.Background()
	violations := m.RunAgentChecks(ctx, nil, []string{"no_harmful"}, "safe and clean content")
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d", len(violations))
	}
}

func TestRunAgentChecks_BothRestrictedTermAndContentPolicy(t *testing.T) {
	m := testManager()
	ctx := context.Background()
	violations := m.RunAgentChecks(ctx, []string{"badword"}, []string{"no_pii"}, "has badword and email@example.com")
	if len(violations) != 2 {
		t.Fatalf("expected 2 violations, got %d", len(violations))
	}
}

func TestRunAgentChecks_RecordsViolations(t *testing.T) {
	m := testManager()
	ctx := context.Background()
	before := m.metrics.GuardrailViolations.Value()
	m.RunAgentChecks(ctx, []string{"bad"}, nil, "contains bad")
	after := m.metrics.GuardrailViolations.Value()
	if after-before != 1 {
		t.Fatalf("expected GuardrailViolations to increment by 1, got %f", after-before)
	}
}

func TestRunAgentChecks_NilMetrics(t *testing.T) {
	m := testManagerWithoutMetrics()
	ctx := context.Background()
	violations := m.RunAgentChecks(ctx, []string{"bad"}, nil, "contains bad")
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
}

func TestRecordViolation_NilLogger(t *testing.T) {
	m := NewManager(nil, nil)
	v := &Violation{ID: "v-1", Type: ViolationPromptLength, Timestamp: time.Now()}
	m.RecordViolation(v)
	vs := m.ListViolations()
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(vs))
	}
}
