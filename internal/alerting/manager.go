// Package alerting provides alert rules, threshold monitoring, and delivery.
package alerting

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"promptsheon/internal/metrics"
)

// Severity levels for alerts.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// AlertStatus represents the current status of an alert.
type AlertStatus string

const (
	StatusActive   AlertStatus = "active"
	StatusResolved AlertStatus = "resolved"
	StatusPending  AlertStatus = "pending"
)

// AlertRule defines a threshold-based alert rule.
type AlertRule struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Severity    Severity       `json:"severity"`
	Enabled     bool           `json:"enabled"`
	Threshold   float64        `json:"threshold"`
	Duration    int            `json:"duration_minutes"`
	Window      int            `json:"window_minutes"`
	Config      map[string]any `json:"config,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// Alert represents a triggered alert.
type Alert struct {
	ID          string         `json:"id"`
	RuleID      string         `json:"rule_id"`
	RuleName    string         `json:"rule_name"`
	Severity    Severity       `json:"severity"`
	Status      AlertStatus    `json:"status"`
	Message     string         `json:"message"`
	Details     map[string]any `json:"details,omitempty"`
	TriggeredAt time.Time      `json:"triggered_at"`
	ResolvedAt  *time.Time     `json:"resolved_at,omitempty"`
}

// NotificationGroup defines a group of notification targets.
type NotificationGroup struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Channels []string `json:"channels"`
}

// Manager manages alert rules and alerts.
type Manager struct {
	mu           sync.RWMutex
	rules        map[string]*AlertRule
	alerts       []*Alert
	groups       map[string]*NotificationGroup
	logger       *slog.Logger
	metrics      *metrics.Collector
	deliveryFunc func(alert *Alert, channels []string) error
}

// NewManager creates a new alerting manager.
func NewManager(logger *slog.Logger, collector *metrics.Collector) *Manager {
	return &Manager{
		rules:  make(map[string]*AlertRule),
		alerts: []*Alert{},
		groups: make(map[string]*NotificationGroup),
		logger: logger,
		metrics: collector,
	}
}

// SetDeliveryFunc sets the function used to deliver alerts.
func (m *Manager) SetDeliveryFunc(fn func(alert *Alert, channels []string) error) {
	m.deliveryFunc = fn
}

// AddRule adds or updates an alert rule.
func (m *Manager) AddRule(rule *AlertRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules[rule.ID] = rule
}

// RemoveRule removes an alert rule.
func (m *Manager) RemoveRule(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rules, id)
}

// GetRule returns a rule by ID.
func (m *Manager) GetRule(id string) (*AlertRule, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.rules[id]
	return r, ok
}

// ListRules returns all rules.
func (m *Manager) ListRules() []*AlertRule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var rules []*AlertRule
	for _, r := range m.rules {
		rules = append(rules, r)
	}
	return rules
}

// ListAlerts returns all alerts.
func (m *Manager) ListAlerts() []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alerts
}

// TriggerAlert triggers a new alert.
func (m *Manager) TriggerAlert(rule *AlertRule, message string, details map[string]any) *Alert {
	alert := &Alert{
		ID:          fmt.Sprintf("alert-%d", time.Now().UnixNano()),
		RuleID:      rule.ID,
		RuleName:    rule.Name,
		Severity:    rule.Severity,
		Status:      StatusActive,
		Message:     message,
		Details:     details,
		TriggeredAt: time.Now(),
	}

	m.mu.Lock()
	m.alerts = append(m.alerts, alert)
	m.mu.Unlock()

	m.logger.Warn("alert triggered",
		"alert_id", alert.ID,
		"rule", rule.Name,
		"severity", alert.Severity,
		"message", message,
	)

	// Deliver notification
	if m.deliveryFunc != nil {
		channels := m.getNotificationChannels(rule)
		go func() {
			if err := m.deliveryFunc(alert, channels); err != nil {
				m.logger.Error("failed to deliver alert", "alert_id", alert.ID, "err", err)
			}
		}()
	}

	return alert
}

// ResolveAlert marks an alert as resolved.
func (m *Manager) ResolveAlert(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.alerts {
		if a.ID == id && a.Status == StatusActive {
			a.Status = StatusResolved
			now := time.Now()
			a.ResolvedAt = &now
			return true
		}
	}
	return false
}

func (m *Manager) getNotificationChannels(rule *AlertRule) []string {
	// Find notification group for this rule's severity
	for _, group := range m.groups {
		return group.Channels
	}
	// Default to webhook
	return []string{"webhook"}
}

// AddNotificationGroup adds a notification group.
func (m *Manager) AddNotificationGroup(group *NotificationGroup) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.groups[group.ID] = group
}

// --- Threshold Checks ---

// CheckLatencySpike checks if p95 latency exceeds threshold.
func (m *Manager) CheckLatencySpike(latencyMs, thresholdMs float64, consecutiveRuns int) *AlertRule {
	if latencyMs > thresholdMs && consecutiveRuns >= 10 {
		return &AlertRule{
			ID:        "latency-spike",
			Name:      "Execution Latency Spike",
			Type:      "latency_spike",
			Severity:  SeverityHigh,
			Enabled:   true,
			Threshold: thresholdMs,
			Duration:  consecutiveRuns,
		}
	}
	return nil
}

// CheckFailureRate checks if error rate exceeds threshold.
func (m *Manager) CheckFailureRate(errorRate float64) *AlertRule {
	if errorRate > 10.0 {
		return &AlertRule{
			ID:        "failure-rate",
			Name:      "Prompt Failure Spike",
			Type:      "failure_rate",
			Severity:  SeverityHigh,
			Enabled:   true,
			Threshold: 10.0,
		}
	}
	return nil
}

// CheckApprovalBottleneck checks if reviews are pending too long.
func (m *Manager) CheckApprovalBottleneck(pendingHours int) *AlertRule {
	if pendingHours > 48 {
		return &AlertRule{
			ID:        "approval-bottleneck",
			Name:      "Approval Bottleneck",
			Type:      "approval_bottleneck",
			Severity:  SeverityMedium,
			Enabled:   true,
			Threshold: 48,
			Duration:  pendingHours,
		}
	}
	return nil
}

// CheckCostOverrun checks if estimated cost exceeds budget.
func (m *Manager) CheckCostOverrun(estimatedCost, budget float64) *AlertRule {
	if estimatedCost > budget {
		return &AlertRule{
			ID:        "cost-overrun",
			Name:      "Cost Overrun",
			Type:      "cost_overrun",
			Severity:  SeverityCritical,
			Enabled:   true,
			Threshold: budget,
		}
	}
	return nil
}

// RunMonitoringChecks runs all monitoring checks against current metrics.
func (m *Manager) RunMonitoringChecks(collector *metrics.Collector) []*Alert {
	var triggered []*Alert

	if collector == nil {
		return triggered
	}

	// Check latency spike (using P95 from histogram)
	p95 := collector.LLMLatency.P95() * 1000 // convert to ms
	if p95 > 2000 { // > 2s
		rule := &AlertRule{
			ID:        "latency-spike",
			Name:      "Execution Latency Spike",
			Type:      "latency_spike",
			Severity:  SeverityHigh,
			Threshold: 2000,
		}
		alert := m.TriggerAlert(rule, fmt.Sprintf("P95 latency is %.0fms (threshold: 2000ms)", p95), map[string]any{
			"p95_latency_ms": p95,
		})
		triggered = append(triggered, alert)
	}

	// Check failure rate
	totalRequests := collector.RequestsTotal.Value()
	totalErrors := collector.ErrorsTotal.Value()
	if totalRequests > 0 {
		errorRate := (totalErrors / totalRequests) * 100
		if errorRate > 10 {
			rule := &AlertRule{
				ID:        "failure-rate",
				Name:      "Prompt Failure Spike",
				Type:      "failure_rate",
				Severity:  SeverityHigh,
				Threshold: 10,
			}
			alert := m.TriggerAlert(rule, fmt.Sprintf("Error rate is %.1f%% (threshold: 10%%)", errorRate), map[string]any{
				"error_rate": errorRate,
				"total_errors": totalErrors,
			})
			triggered = append(triggered, alert)
		}
	}

	return triggered
}

// StartMonitoring begins periodic monitoring checks.
func (m *Manager) StartMonitoring(ctx context.Context, collector *metrics.Collector, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.RunMonitoringChecks(collector)
			}
		}
	}()
}
