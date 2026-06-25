// Package guardrail provides static and runtime guardrail enforcement for prompts.
package guardrail

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/sachn-cs/promptsheon/internal/metrics"
	"github.com/sachn-cs/promptsheon/internal/models"
	"github.com/sachn-cs/promptsheon/internal/store"
)

// Severity levels for guardrail violations.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

const maxViolations = 10000

// ViolationType categorizes guardrail violations.
type ViolationType string

const (
	ViolationPromptLength      ViolationType = "prompt_length"
	ViolationRestrictedTerm    ViolationType = "restricted_term"
	ViolationModelAccess       ViolationType = "model_access"
	ViolationQuotaExceeded     ViolationType = "quota_exceeded"
	ViolationHallucinationHigh ViolationType = "hallucination_high"
	ViolationFormatInvalid     ViolationType = "format_invalid"
	ViolationCostLimit         ViolationType = "cost_limit"
	ViolationLatencyLimit      ViolationType = "latency_limit"
	ViolationContentPolicy     ViolationType = "content_policy"
)

// Violation represents a guardrail violation record.
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

// ViolationResult is the result of a guardrail check.
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
	db         store.Repository
}

// NewManager creates a new guardrail manager.
func NewManager(logger *slog.Logger, collector *metrics.Collector) *Manager {
	return &Manager{
		rules:      make(map[string]*Rule),
		violations: []*Violation{},
		logger:     logger,
		metrics:    collector,
	}
}

// NewManagerWithDB creates a new guardrail manager with database persistence.
func NewManagerWithDB(logger *slog.Logger, collector *metrics.Collector, db store.Repository) *Manager {
	m := &Manager{
		rules:      make(map[string]*Rule),
		violations: []*Violation{},
		logger:     logger,
		metrics:    collector,
		db:         db,
	}
	// Load rules from database
	m.loadRulesFromDB()
	return m
}

// loadRulesFromDB loads all rules from the database into memory.
func (m *Manager) loadRulesFromDB() {
	if m.db == nil {
		return
	}
	ctx := context.Background()
	dbRules, err := m.db.ListGuardrailRules(ctx)
	if err != nil {
		if m.logger != nil {
			m.logger.Error("failed to load guardrail rules from db", "err", err)
		}
		return
	}
	for _, dr := range dbRules {
		rule := &Rule{
			ID:           dr.ID,
			Name:         dr.Name,
			Type:         ViolationType(dr.Type),
			Severity:     Severity(dr.Severity),
			Enabled:      dr.Enabled,
			Config:       dr.Config,
			Environments: dr.Environments,
			PromptIDs:    dr.PromptIDs,
			AgentIDs:     dr.AgentIDs,
			CreatedAt:    dr.CreatedAt,
			UpdatedAt:    dr.UpdatedAt,
		}
		m.rules[rule.ID] = rule
	}
}

// AddRule adds or updates a guardrail rule.
func (m *Manager) AddRule(rule *Rule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules[rule.ID] = rule

	// Persist to database
	if m.db != nil {
		dbRule := &models.GuardrailRule{
			ID:           rule.ID,
			Name:         rule.Name,
			Type:         string(rule.Type),
			Severity:     string(rule.Severity),
			Enabled:      rule.Enabled,
			Config:       rule.Config,
			Environments: rule.Environments,
			PromptIDs:    rule.PromptIDs,
			AgentIDs:     rule.AgentIDs,
			CreatedAt:    rule.CreatedAt,
			UpdatedAt:    rule.UpdatedAt,
		}
		if err := m.db.SaveGuardrailRule(context.Background(), dbRule); err != nil {
			if m.logger != nil {
				m.logger.Error("failed to save guardrail rule to db", "err", err, "rule_id", rule.ID)
			}
		}
	}
}

// RemoveRule removes a guardrail rule.
func (m *Manager) RemoveRule(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rules, id)
}

// GetRule returns a rule by ID.
func (m *Manager) GetRule(id string) (*Rule, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.rules[id]
	return r, ok
}

// ListRules returns all rules.
func (m *Manager) ListRules() []*Rule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var rules []*Rule
	for _, r := range m.rules {
		rules = append(rules, r)
	}
	return rules
}

// ListViolations returns all violations.
func (m *Manager) ListViolations() []*Violation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.violations
}

// RecordViolation records a guardrail violation.
func (m *Manager) RecordViolation(v *Violation) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Cap violations to prevent unbounded memory growth
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

// RecordBlock records a guardrail block.
func (m *Manager) RecordBlock() {
	if m.metrics != nil {
		m.metrics.GuardrailBlocks.Inc()
	}
}

// RecordPass records a guardrail pass.
func (m *Manager) RecordPass() {
	if m.metrics != nil {
		m.metrics.GuardrailPasses.Inc()
	}
}

// --- Static Guardrails ---

// CheckPromptLength checks if prompt content exceeds max length.
func (m *Manager) CheckPromptLength(content string, maxLength int) *ViolationResult {
	if maxLength <= 0 {
		return &ViolationResult{Passed: true}
	}

	tokenEstimate := len(strings.Fields(content)) // rough word count as proxy
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
					Details:   map[string]any{"term": term},
					Timestamp: time.Now(),
				},
			}
		}
	}
	return &ViolationResult{Passed: true}
}

// CheckModelAccess checks if a model is allowed for the given environment.
func (m *Manager) CheckModelAccess(model, environment string, allowedModels map[string][]string) *ViolationResult {
	if allowedModels == nil {
		return &ViolationResult{Passed: true}
	}

	allowed, ok := allowedModels[environment]
	if !ok {
		// No restriction for this environment
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

// --- Runtime Guardrails ---

// CheckResponseFormat validates response matches expected format.
func (m *Manager) CheckResponseFormat(response string, formatSpec string) *ViolationResult {
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
					Details:   map[string]any{"expected_format": "json"},
					Timestamp: time.Now(),
				},
			}
		}
	case "markdown":
		// Basic markdown check - should contain some structure
		if !strings.Contains(response, "#") && !strings.Contains(response, "*") && !strings.Contains(response, "-") {
			return &ViolationResult{
				Passed: false,
				Violation: &Violation{
					ID:        fmt.Sprintf("v-%d", time.Now().UnixNano()),
					Type:      ViolationFormatInvalid,
					Severity:  SeverityLow,
					Message:   "response may not be valid markdown",
					Details:   map[string]any{"expected_format": "markdown"},
					Timestamp: time.Now(),
				},
			}
		}
	default:
		// Regex pattern matching
		matched, err := regexp.MatchString(formatSpec, response)
		if err != nil || !matched {
			return &ViolationResult{
				Passed: false,
				Violation: &Violation{
					ID:        fmt.Sprintf("v-%d", time.Now().UnixNano()),
					Type:      ViolationFormatInvalid,
					Severity:  SeverityMedium,
					Message:   fmt.Sprintf("response does not match expected pattern: %s", formatSpec),
					Details:   map[string]any{"expected_format": formatSpec},
					Timestamp: time.Now(),
				},
			}
		}
	}

	return &ViolationResult{Passed: true}
}

// CheckContentPolicy checks for harmful or policy-violating content.
func (m *Manager) CheckContentPolicy(content string, policies []string) *ViolationResult {
	contentLower := strings.ToLower(content)

	// Default policy checks
	for _, policy := range policies {
		switch policy {
		case "no_pii":
			// Basic PII detection patterns
			piiPatterns := []string{
				`\b\d{3}-\d{2}-\d{4}\b`, // SSN
				`\b\d{16}\b`,            // Credit card (basic)
				`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`, // Email
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
							Details:   map[string]any{"policy": "no_harmful", "term": term},
							Timestamp: time.Now(),
						},
					}
				}
			}
		}
	}

	return &ViolationResult{Passed: true}
}

// CheckCostLimit checks if estimated cost exceeds limit.
func (m *Manager) CheckCostLimit(estimatedCostUSD float64, limitUSD float64) *ViolationResult {
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

// CheckLatencyLimit checks if latency exceeds limit.
func (m *Manager) CheckLatencyLimit(latencyMs int64, limitMs int64) *ViolationResult {
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

// RunAllStaticChecks runs all static guardrails on prompt content.
func (m *Manager) RunAllStaticChecks(ctx context.Context, content, model, environment string) []*Violation {
	var violations []*Violation

	// Prompt length check
	if result := m.CheckPromptLength(content, 10000); !result.Passed {
		violations = append(violations, result.Violation)
	}

	// Restricted terms (default list)
	bannedTerms := []string{"banned_topic_1", "banned_topic_2"}
	if result := m.CheckRestrictedTerms(content, bannedTerms); !result.Passed {
		violations = append(violations, result.Violation)
	}

	// Model access
	allowedModels := map[string][]string{
		"prod": {"gpt-4", "claude-3-opus"},
		"dev":  {"gpt-3.5-turbo", "gpt-4", "claude-3-haiku"},
	}
	if result := m.CheckModelAccess(model, environment, allowedModels); !result.Passed {
		violations = append(violations, result.Violation)
	}

	// Record violations
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

// RunAgentChecks runs guardrails specific to an agent's configuration.
// It checks restricted terms and content policy against the provided content.
func (m *Manager) RunAgentChecks(ctx context.Context, restrictedTerms []string, contentPolicy []string, content string) []*Violation {
	var violations []*Violation

	// Check restricted terms
	for _, term := range restrictedTerms {
		if strings.Contains(strings.ToLower(content), strings.ToLower(term)) {
			v := &Violation{
				ID:        fmt.Sprintf("v-%d", time.Now().UnixNano()),
				Type:      ViolationRestrictedTerm,
				Severity:  SeverityHigh,
				Message:   fmt.Sprintf("restricted term found: %s", term),
				Details:   map[string]any{"term": term},
				Timestamp: time.Now(),
			}
			violations = append(violations, v)
			m.RecordViolation(v)
		}
	}

	// Check content policy
	if len(contentPolicy) > 0 {
		if result := m.CheckContentPolicy(content, contentPolicy); !result.Passed {
			violations = append(violations, result.Violation)
			m.RecordViolation(result.Violation)
		}
	}

	return violations
}
