// Package guardrail provides guardrail management for safety and quality checks.
package guardrail

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/sachncs/promptsheon/internal/metrics"
)

const keyTerm = "term"
const keyExpectedFormat = "expected_format"

// Severity represents the severity level of a guardrail violation.
type Severity string

const (
	// SeverityLow is a low severity level.
	SeverityLow      Severity = "low"
	// SeverityMedium is a medium severity level.
	SeverityMedium   Severity = "medium"
	// SeverityHigh is a high severity level.
	SeverityHigh     Severity = "high"
	// SeverityCritical is a critical severity level.
	SeverityCritical Severity = "critical"
)

const maxViolations = 10000

// ViolationType identifies the kind of guardrail violation.
type ViolationType string

const (
	// ViolationPromptLength is a prompt length violation type.
	ViolationPromptLength      ViolationType = "prompt_length"
	// ViolationRestrictedTerm is a restricted term violation type.
	ViolationRestrictedTerm    ViolationType = "restricted_term"
	// ViolationModelAccess is a model access violation type.
	ViolationModelAccess       ViolationType = "model_access"
	// ViolationQuotaExceeded is a quota exceeded violation type.
	ViolationQuotaExceeded     ViolationType = "quota_exceeded"
	// ViolationHallucinationHigh is a high hallucination violation type.
	ViolationHallucinationHigh ViolationType = "hallucination_high"
	// ViolationFormatInvalid is an invalid format violation type.
	ViolationFormatInvalid     ViolationType = "format_invalid"
	// ViolationCostLimit is a cost limit violation type.
	ViolationCostLimit         ViolationType = "cost_limit"
	// ViolationLatencyLimit is a latency limit violation type.
	ViolationLatencyLimit      ViolationType = "latency_limit"
	// ViolationContentPolicy is a content policy violation type.
	ViolationContentPolicy     ViolationType = "content_policy"
)

// Violation records a single guardrail violation.
type Violation struct {
	ID           string         `json:"id"`
	RuleID       string         `json:"rule_id"`
	RuleName     string         `json:"rule_name"`
	Type         ViolationType  `json:"type"`
	Severity     Severity       `json:"severity"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	UserID       string         `json:"user_id"`
	Message      string         `json:"message"`
	Details      map[string]any `json:"details,omitempty"`
	Resolved     bool           `json:"resolved"`
	ResolvedBy   string         `json:"resolved_by,omitempty"`
	ResolvedAt   *time.Time     `json:"resolved_at,omitempty"`
	Timestamp    time.Time      `json:"timestamp"`
}

// Rule defines a guardrail rule.
type Rule struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Type         ViolationType  `json:"type"`
	Severity     Severity       `json:"severity"`
	Enabled      bool           `json:"enabled"`
	Config       map[string]any `json:"config,omitempty"`
	Environments []string       `json:"environments,omitempty"`
	PromptIDs    []string       `json:"prompt_ids,omitempty"`
	AgentIDs     []string       `json:"agent_ids,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// ViolationResult contains the result of a guardrail check.
type ViolationResult struct {
	Passed    bool       `json:"passed"`
	Violation *Violation `json:"violation,omitempty"`
}

// Manager manages guardrail rules and violations.
type Manager struct {
	mu         sync.RWMutex
	rules      map[string]*Rule
	violations []*Violation
	logger     *slog.Logger
	metrics    *metrics.Collector
}

// NewManager creates a new Manager.
func NewManager(logger *slog.Logger, collector *metrics.Collector) *Manager {
	return &Manager{
		rules:      make(map[string]*Rule),
		violations: []*Violation{},
		logger:     logger,
		metrics:    collector,
	}
}

// ListRules returns all registered guardrail rules.
func (m *Manager) ListRules() []*Rule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rules := make([]*Rule, 0, len(m.rules))
	for _, r := range m.rules {
		rules = append(rules, r)
	}
	return rules
}

// ListViolations returns all recorded violations.
func (m *Manager) ListViolations() []*Violation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.violations
}

// RecordViolation records a guardrail violation.
func (m *Manager) RecordViolation(v *Violation) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.violations) >= maxViolations {
		m.violations = m.violations[1:]
	}
	m.violations = append(m.violations, v)

	if m.metrics != nil {
		m.metrics.GuardrailViolations.Inc()
	}

	if m.logger != nil {
		m.logger.Warn("guardrail violation",
			"rule_id", v.RuleID,
			"type", v.Type,
			"severity", v.Severity,
			"resource", v.ResourceType+":"+v.ResourceID,
			"message", v.Message,
		)
	}
}

// RecordBlock increments the block counter.
func (m *Manager) RecordBlock() {
	if m.metrics != nil {
		m.metrics.GuardrailBlocks.Inc()
	}
}

// RecordPass increments the pass counter.
func (m *Manager) RecordPass() {
	if m.metrics != nil {
		m.metrics.GuardrailPasses.Inc()
	}
}

// CheckPromptLength checks if content exceeds the maximum length.
func (m *Manager) CheckPromptLength(content string, maxLength int) *ViolationResult {
	if maxLength <= 0 {
		return &ViolationResult{Passed: true}
	}

	tokenEstimate := len(strings.Fields(content))
	if len(content) > maxLength {
		return &ViolationResult{
			Passed: false,
			Violation: &Violation{
				ID:        fmt.Sprintf("v-%d", time.Now().UnixNano()),
				Type:      ViolationPromptLength,
				Severity:  SeverityMedium,
				Message:   fmt.Sprintf("prompt exceeds max length: %d chars > %d limit", len(content), maxLength),
				Details:   map[string]any{"length": len(content), "max": maxLength, "token_estimate": tokenEstimate},
				Timestamp: time.Now(),
			},
		}
	}
	return &ViolationResult{Passed: true}
}

// CheckRestrictedTerms checks if content contains banned terms.
func (m *Manager) CheckRestrictedTerms(content string, bannedTerms []string) *ViolationResult {
	contentLower := strings.ToLower(content)
	for _, term := range bannedTerms {
		if strings.Contains(contentLower, strings.ToLower(term)) {
			return &ViolationResult{
				Passed: false,
				Violation: &Violation{
					ID:        fmt.Sprintf("v-%d", time.Now().UnixNano()),
					Type:      ViolationRestrictedTerm,
					Severity:  SeverityHigh,
					Message:   fmt.Sprintf("restricted term detected: %q", term),
					Details:   map[string]any{keyTerm: term},
					Timestamp: time.Now(),
				},
			}
		}
	}
	return &ViolationResult{Passed: true}
}

// CheckModelAccess checks if a model is allowed in the given environment.
func (m *Manager) CheckModelAccess(model, environment string, allowedModels map[string][]string) *ViolationResult {
	if allowedModels == nil {
		return &ViolationResult{Passed: true}
	}

	allowed, ok := allowedModels[environment]
	if !ok {
		return &ViolationResult{Passed: true}
	}

	for _, am := range allowed {
		if strings.EqualFold(am, model) {
			return &ViolationResult{Passed: true}
		}
	}

	return &ViolationResult{
		Passed: false,
		Violation: &Violation{
			ID:        fmt.Sprintf("v-%d", time.Now().UnixNano()),
			Type:      ViolationModelAccess,
			Severity:  SeverityCritical,
			Message:   fmt.Sprintf("model %q not allowed in environment %q", model, environment),
			Details:   map[string]any{"model": model, "environment": environment},
			Timestamp: time.Now(),
		},
	}
}

// CheckResponseFormat checks if the response matches the expected format.
func (m *Manager) CheckResponseFormat(response, formatSpec string) *ViolationResult {
	if formatSpec == "" {
		return &ViolationResult{Passed: true}
	}

	switch formatSpec {
	case "json":
		response = strings.TrimSpace(response)
		if !strings.HasPrefix(response, "{") && !strings.HasPrefix(response, "[") {
			return &ViolationResult{
				Passed: false,
				Violation: &Violation{
					ID:        fmt.Sprintf("v-%d", time.Now().UnixNano()),
					Type:      ViolationFormatInvalid,
					Severity:  SeverityMedium,
					Message:   "response is not valid JSON",
					Details:   map[string]any{keyExpectedFormat: "json"},
					Timestamp: time.Now(),
				},
			}
		}
	case "markdown":
		if !strings.Contains(response, "#") && !strings.Contains(response, "*") && !strings.Contains(response, "-") {
			return &ViolationResult{
				Passed: false,
				Violation: &Violation{
					ID:        fmt.Sprintf("v-%d", time.Now().UnixNano()),
					Type:      ViolationFormatInvalid,
					Severity:  SeverityLow,
					Message:   "response may not be valid markdown",
					Details:   map[string]any{keyExpectedFormat: "markdown"},
					Timestamp: time.Now(),
				},
			}
		}
	default:
		matched, err := regexp.MatchString(formatSpec, response)
		if err != nil || !matched {
			return &ViolationResult{
				Passed: false,
				Violation: &Violation{
					ID:        fmt.Sprintf("v-%d", time.Now().UnixNano()),
					Type:      ViolationFormatInvalid,
					Severity:  SeverityMedium,
					Message:   fmt.Sprintf("response does not match expected pattern: %s", formatSpec),
					Details:   map[string]any{keyExpectedFormat: formatSpec},
					Timestamp: time.Now(),
				},
			}
		}
	}

	return &ViolationResult{Passed: true}
}

// CheckContentPolicy checks content against defined content policies.
func (m *Manager) CheckContentPolicy(content string, policies []string) *ViolationResult {
	contentLower := strings.ToLower(content)

	for _, policy := range policies {
		switch policy {
		case "no_pii":
			piiPatterns := []string{
				`\b\d{3}-\d{2}-\d{4}\b`,
				`\b\d{16}\b`,
				`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`,
			}
			for _, pattern := range piiPatterns {
				if matched, _ := regexp.MatchString(pattern, content); matched {
					return &ViolationResult{
						Passed: false,
						Violation: &Violation{
							ID:        fmt.Sprintf("v-%d", time.Now().UnixNano()),
							Type:      ViolationContentPolicy,
							Severity:  SeverityHigh,
							Message:   "PII detected in response",
							Details:   map[string]any{"policy": "no_pii"},
							Timestamp: time.Now(),
						},
					}
				}
			}
		case "no_harmful":
			harmfulTerms := []string{"suicide", "self-harm", "bomb", "weapon", "kill"}
			for _, term := range harmfulTerms {
				if strings.Contains(contentLower, term) {
					return &ViolationResult{
						Passed: false,
						Violation: &Violation{
							ID:        fmt.Sprintf("v-%d", time.Now().UnixNano()),
							Type:      ViolationContentPolicy,
							Severity:  SeverityCritical,
							Message:   fmt.Sprintf("harmful content detected: %s", term),
							Details:   map[string]any{"policy": "no_harmful", keyTerm: term},
							Timestamp: time.Now(),
						},
					}
				}
			}
		}
	}

	return &ViolationResult{Passed: true}
}

// CheckCostLimit checks if the estimated cost exceeds the limit.
func (m *Manager) CheckCostLimit(estimatedCostUSD, limitUSD float64) *ViolationResult {
	if limitUSD <= 0 {
		return &ViolationResult{Passed: true}
	}

	if estimatedCostUSD > limitUSD {
		return &ViolationResult{
			Passed: false,
			Violation: &Violation{
				ID:        fmt.Sprintf("v-%d", time.Now().UnixNano()),
				Type:      ViolationCostLimit,
				Severity:  SeverityHigh,
				Message:   fmt.Sprintf("estimated cost $%.4f exceeds limit $%.4f", estimatedCostUSD, limitUSD),
				Details:   map[string]any{"estimated": estimatedCostUSD, "limit": limitUSD},
				Timestamp: time.Now(),
			},
		}
	}
	return &ViolationResult{Passed: true}
}

// CheckLatencyLimit checks if the latency exceeds the limit.
func (m *Manager) CheckLatencyLimit(latencyMs, limitMs int64) *ViolationResult {
	if limitMs <= 0 {
		return &ViolationResult{Passed: true}
	}

	if latencyMs > limitMs {
		return &ViolationResult{
			Passed: false,
			Violation: &Violation{
				ID:        fmt.Sprintf("v-%d", time.Now().UnixNano()),
				Type:      ViolationLatencyLimit,
				Severity:  SeverityMedium,
				Message:   fmt.Sprintf("latency %dms exceeds limit %dms", latencyMs, limitMs),
				Details:   map[string]any{"latency_ms": latencyMs, "limit_ms": limitMs},
				Timestamp: time.Now(),
			},
		}
	}
	return &ViolationResult{Passed: true}
}

// RunAllStaticChecks runs all static guardrail checks against content.
func (m *Manager) RunAllStaticChecks(_ context.Context, content, model, environment string) []*Violation {
	var violations []*Violation

	if result := m.CheckPromptLength(content, 10000); !result.Passed {
		violations = append(violations, result.Violation)
	}

	bannedTerms := []string{"banned_topic_1", "banned_topic_2"}
	if result := m.CheckRestrictedTerms(content, bannedTerms); !result.Passed {
		violations = append(violations, result.Violation)
	}

	allowedModels := map[string][]string{
		"prod": {"gpt-4", "claude-3-opus"},
		"dev":  {"gpt-3.5-turbo", "gpt-4", "claude-3-haiku"},
	}
	if result := m.CheckModelAccess(model, environment, allowedModels); !result.Passed {
		violations = append(violations, result.Violation)
	}

	for _, v := range violations {
		m.RecordViolation(v)
	}

	if len(violations) == 0 {
		m.RecordPass()
	} else {
		m.RecordBlock()
	}

	return violations
}

// RunAgentChecks runs guardrail checks for agent-driven content.
func (m *Manager) RunAgentChecks(_ context.Context, restrictedTerms, contentPolicy []string, content string) []*Violation {
	var violations []*Violation

	for _, term := range restrictedTerms {
		if strings.Contains(strings.ToLower(content), strings.ToLower(term)) {
			v := &Violation{
				ID:        fmt.Sprintf("v-%d", time.Now().UnixNano()),
				Type:      ViolationRestrictedTerm,
				Severity:  SeverityHigh,
				Message:   fmt.Sprintf("restricted term found: %s", term),
				Details:   map[string]any{keyTerm: term},
				Timestamp: time.Now(),
			}
			violations = append(violations, v)
			m.RecordViolation(v)
		}
	}

	if len(contentPolicy) > 0 {
		if result := m.CheckContentPolicy(content, contentPolicy); !result.Passed {
			violations = append(violations, result.Violation)
			m.RecordViolation(result.Violation)
		}
	}

	return violations
}
