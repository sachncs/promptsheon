// Package alerting provides alert rules, threshold monitoring, and delivery.
package alerting

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/sachncs/promptsheon/internal/metrics"
	"github.com/sachncs/promptsheon/internal/models"
	"github.com/sachncs/promptsheon/internal/store"
)

// Severity levels for alerts.
type Severity string

const (
	// SeverityLow is a low severity level.
	SeverityLow Severity = "low"
	// SeverityMedium is a medium severity level.
	SeverityMedium Severity = "medium"
	// SeverityHigh is a high severity level.
	SeverityHigh Severity = "high"
	// SeverityCritical is a critical severity level.
	SeverityCritical Severity = "critical"
)

// AlertStatus represents the current status of an alert.
type AlertStatus string

const (
	// StatusActive is an active alert status.
	StatusActive AlertStatus = "active"
	// StatusResolved is a resolved alert status.
	StatusResolved AlertStatus = "resolved"
	// StatusPending is a pending alert status.
	StatusPending AlertStatus = "pending"
)

// AlertRule defines a threshold-based alert rule.
type AlertRule struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	Severity  Severity       `json:"severity"`
	Enabled   bool           `json:"enabled"`
	Threshold float64        `json:"threshold"`
	Duration  int            `json:"duration_minutes"`
	Window    int            `json:"window_minutes"`
	Config    map[string]any `json:"config,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
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

// MaxConcurrentDeliveries is the maximum number of concurrent alert
// delivery goroutines. Additional alerts are dropped rather than
// unboundedly spawning goroutines.
const MaxConcurrentDeliveries = 100

// Manager manages alert rules and alerts.
type Manager struct {
	mu           sync.RWMutex
	rules        map[string]*AlertRule
	alerts       []*Alert
	groups       map[string]*NotificationGroup
	logger       *slog.Logger
	metrics      *metrics.Collector
	db           store.Repository
	deliveryFunc func(alert *Alert, channels []string) error
	deliverySem  chan struct{}
	monitoringWg sync.WaitGroup
}

// NewManager creates a new alerting manager.
func NewManager(logger *slog.Logger, collector *metrics.Collector) *Manager {
	return &Manager{
		rules:       make(map[string]*AlertRule),
		alerts:      []*Alert{},
		groups:      make(map[string]*NotificationGroup),
		logger:      logger,
		metrics:     collector,
		deliverySem: make(chan struct{}, MaxConcurrentDeliveries),
	}
}

// NewManagerWithDB creates a new alerting manager with database persistence.
func NewManagerWithDB(logger *slog.Logger, collector *metrics.Collector, db store.Repository) *Manager {
	m := &Manager{
		rules:   make(map[string]*AlertRule),
		alerts:  []*Alert{},
		groups:  make(map[string]*NotificationGroup),
		logger:  logger,
		metrics: collector,
		db:      db,
	}
	// Load from database
	m.loadFromDB()
	return m
}

// loadFromDB loads all alert rules, alerts, and notification groups from the database.
func (m *Manager) loadFromDB() {
	if m.db == nil {
		return
	}
	ctx := context.Background()

	// Load alert rules
	dbRules, err := m.db.ListAlertRules(ctx)
	if err != nil {
		if m.logger != nil {
			m.logger.Error("failed to load alert rules from db", "err", err)
		}
	} else {
		for _, dr := range dbRules {
			rule := &AlertRule{
				ID:        dr.ID,
				Name:      dr.Name,
				Type:      dr.Type,
				Severity:  Severity(dr.Severity),
				Enabled:   dr.Enabled,
				Threshold: dr.Threshold,
				Duration:  dr.Duration,
				Window:    dr.Window,
				Config:    dr.Config,
				CreatedAt: dr.CreatedAt,
				UpdatedAt: dr.UpdatedAt,
			}
			m.rules[rule.ID] = rule
		}
	}

	// Load alerts
	dbAlerts, err := m.db.ListAlerts(ctx, "")
	if err != nil {
		if m.logger != nil {
			m.logger.Error("failed to load alerts from db", "err", err)
		}
	} else {
		for _, da := range dbAlerts {
			alert := &Alert{
				ID:          da.ID,
				RuleID:      da.RuleID,
				RuleName:    da.RuleName,
				Severity:    Severity(da.Severity),
				Status:      AlertStatus(da.Status),
				Message:     da.Message,
				Details:     da.Details,
				TriggeredAt: da.TriggeredAt,
				ResolvedAt:  da.ResolvedAt,
			}
			m.alerts = append(m.alerts, alert)
		}
	}

	// Load notification groups
	dbGroups, err := m.db.ListNotificationGroups(ctx)
	if err != nil {
		if m.logger != nil {
			m.logger.Error("failed to load notification groups from db", "err", err)
		}
	} else {
		for _, dg := range dbGroups {
			group := &NotificationGroup{
				ID:       dg.ID,
				Name:     dg.Name,
				Channels: dg.Channels,
			}
			m.groups[group.ID] = group
		}
	}
}

// SetDeliveryFunc sets the function used to deliver alerts. The
// function is responsible for honouring the channel list: a
// "webhook" channel must produce an HTTP POST to the configured
// endpoints (see internal/webhook); a "log" channel writes to
// slog; future "slack"/"pagerduty" channels can be added without
// touching this file.
//
// If SetDeliveryFunc is never called, alerts persist to the
// database (visible via ListAlerts) and the channel list is
// recorded in the alert's Details. No notification is sent. This
// is the documented default; silent dropping was the previous
// behaviour, but persisting means the alert is at least visible
// to operators who look.
func (m *Manager) SetDeliveryFunc(fn func(alert *Alert, channels []string) error) {
	m.deliveryFunc = fn
}

// AddRule adds or updates an alert rule.
func (m *Manager) AddRule(rule *AlertRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules[rule.ID] = rule

	// Persist to database
	if m.db != nil {
		dbRule := &models.AlertRuleRecord{
			ID:        rule.ID,
			Name:      rule.Name,
			Type:      rule.Type,
			Severity:  string(rule.Severity),
			Enabled:   rule.Enabled,
			Threshold: rule.Threshold,
			Duration:  rule.Duration,
			Window:    rule.Window,
			Config:    rule.Config,
			CreatedAt: rule.CreatedAt,
			UpdatedAt: rule.UpdatedAt,
		}
		if err := m.db.SaveAlertRule(context.Background(), dbRule); err != nil {
			if m.logger != nil {
				m.logger.Error("failed to save alert rule to db", "err", err, "rule_id", rule.ID)
			}
		}
	}
}

// RemoveRule removes an alert rule.
func (m *Manager) RemoveRule(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rules, id)

	// Remove from database
	if m.db != nil {
		if err := m.db.DeleteAlertRule(context.Background(), id); err != nil {
			if m.logger != nil {
				m.logger.Error("failed to delete alert rule from db", "err", err, "rule_id", id)
			}
		}
	}
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
	rules := make([]*AlertRule, 0, len(m.rules))
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

	// Persist to database
	if m.db != nil {
		dbAlert := &models.AlertRecord{
			ID:          alert.ID,
			RuleID:      alert.RuleID,
			RuleName:    alert.RuleName,
			Severity:    string(alert.Severity),
			Status:      string(alert.Status),
			Message:     alert.Message,
			Details:     alert.Details,
			TriggeredAt: alert.TriggeredAt,
			ResolvedAt:  alert.ResolvedAt,
		}
		if err := m.db.SaveAlert(context.Background(), dbAlert); err != nil {
			if m.logger != nil {
				m.logger.Error("failed to save alert to db", "err", err, "alert_id", alert.ID)
			}
		}
	}

	m.logger.Warn("alert triggered",
		"alert_id", alert.ID,
		"rule", rule.Name,
		"severity", alert.Severity,
		"message", message,
	)

	// Deliver notification with bounded concurrency.
	// Acquiring the semaphore token before spawning the goroutine
	// means we block TriggerAlert when the delivery pool is saturated,
	// which naturally back-pressures the caller.
	if m.deliveryFunc != nil {
		channels := m.getNotificationChannels(rule)
		m.deliverySem <- struct{}{}
		go func() {
			defer func() { <-m.deliverySem }()
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
		if a.ID != id || a.Status != StatusActive {
			continue
		}
		a.Status = StatusResolved
		now := time.Now()
		a.ResolvedAt = &now

		// Update in database
		if m.db != nil {
			dbAlert := &models.AlertRecord{
				ID:         a.ID,
				Status:     string(a.Status),
				ResolvedAt: a.ResolvedAt,
			}
			if err := m.db.UpdateAlert(context.Background(), dbAlert); err != nil {
				if m.logger != nil {
					m.logger.Error("failed to update alert in db", "err", err, "alert_id", a.ID)
				}
			}
		}

		return true
	}
	return false
}

// getNotificationChannels returns the channels that should receive
// notifications for the given rule. Post-045c the routing is
// driven by the alert_rule_notification_groups M2M table (added
// in migration 045a and backfilled by 045b). The legacy name-match
// heuristic is gone; the M2M table is the single source of
// truth for which groups fire on a given rule.
//
// The function falls back to []string{channelWebhook} when the
// rule has no M2M row at all. The 046 backfill seeds the M2M
// with the legacy default-group fallback, so an alert without
// an M2M row is the rare case where the operator has explicitly
// opted out of routing for that rule.
func (m *Manager) getNotificationChannels(rule *AlertRule) []string {
	if m.db == nil {
		// In-memory mode (no DB). The M2M is empty. Fall back
		// to the webhook channel so alerts are never silently
		// dropped in tests that don't wire a DB.
		return []string{channelWebhook}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	groups, err := m.db.GetChannelsForAlertRule(ctx, rule.ID)
	if err != nil {
		m.logger.Error("alerting: load M2M for rule", "rule_id", rule.ID, "err", err)
		return []string{channelWebhook}
	}
	if len(groups) == 0 {
		return []string{channelWebhook}
	}
	// DB-10b: dedupe and sort on the manager side. The store
	// also dedupes (DISTINCT in the json_each SQL), but we
	// cannot trust every storage backend to do that; the
	// alerting manager is the last mile before dispatch and
	// must guarantee a stable, unique list.
	seen := make(map[string]struct{}, len(groups))
	out := make([]string, 0, len(groups))
	for _, g := range groups {
		if _, ok := seen[g]; ok {
			continue
		}
		seen[g] = struct{}{}
		out = append(out, g)
	}
	sort.Strings(out)
	return out
}

// AddNotificationGroup adds a notification group.
func (m *Manager) AddNotificationGroup(group *NotificationGroup) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.groups[group.ID] = group

	// Persist to database
	if m.db != nil {
		dbGroup := &models.NotificationGroupRecord{
			ID:       group.ID,
			Name:     group.Name,
			Channels: group.Channels,
		}
		if err := m.db.SaveNotificationGroup(context.Background(), dbGroup); err != nil {
			if m.logger != nil {
				m.logger.Error("failed to save notification group to db", "err", err, "group_id", group.ID)
			}
		}
	}
}

const alertTypeLatencySpike = "latency_spike"
const alertIDLatencySpike = "latency-spike"
const alertIDFailureRate = "failure-rate"
const channelWebhook = "webhook"

// --- Threshold Checks ---

// CheckLatencySpike checks if p95 latency exceeds threshold.
func (m *Manager) CheckLatencySpike(latencyMs, thresholdMs float64, consecutiveRuns int) *AlertRule {
	if latencyMs > thresholdMs && consecutiveRuns >= 10 {
		return &AlertRule{
			ID:        alertIDLatencySpike,
			Name:      "Execution Latency Spike",
			Type:      alertTypeLatencySpike,
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
			ID:        alertIDFailureRate,
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
	if p95 > 2000 {                          // > 2s
		rule := &AlertRule{
			ID:        alertIDLatencySpike,
			Name:      "Execution Latency Spike",
			Type:      alertTypeLatencySpike,
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
				ID:        alertIDFailureRate,
				Name:      "Prompt Failure Spike",
				Type:      "failure_rate",
				Severity:  SeverityHigh,
				Threshold: 10,
			}
			alert := m.TriggerAlert(rule, fmt.Sprintf("Error rate is %.1f%% (threshold: 10%%)", errorRate), map[string]any{
				"error_rate":   errorRate,
				"total_errors": totalErrors,
			})
			triggered = append(triggered, alert)
		}
	}

	return triggered
}

// StartMonitoring begins periodic monitoring checks.
func (m *Manager) StartMonitoring(ctx context.Context, collector *metrics.Collector, interval time.Duration) {
	m.monitoringWg.Add(1)
	go func() {
		defer m.monitoringWg.Done()
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

// StopMonitoring waits for the monitoring goroutine to finish.
func (m *Manager) StopMonitoring() {
	m.monitoringWg.Wait()
}
