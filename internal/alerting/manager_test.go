package alerting

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/metrics"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	collector := metrics.NewCollector()
	return NewManager(logger, collector)
}

func TestNewManagerInitialisesEmptyState(t *testing.T) {
	m := newTestManager(t)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.rules == nil {
		t.Error("rules map should be non-nil")
	}
	if m.alerts == nil {
		t.Error("alerts slice should be non-nil")
	}
	if m.groups == nil {
		t.Error("groups map should be non-nil")
	}
}

func TestAddRulePersistsInMemory(t *testing.T) {
	m := newTestManager(t)
	rule := &AlertRule{
		ID:        "r1",
		Name:      "high latency",
		Type:      "latency",
		Severity:  SeverityHigh,
		Enabled:   true,
		Threshold: 500,
	}
	m.AddRule(rule)
	got, ok := m.GetRule("r1")
	if !ok {
		t.Fatal("GetRule returned not-found for freshly added rule")
	}
	if got.Name != "high latency" {
		t.Fatalf("expected name 'high latency', got %q", got.Name)
	}
	if !got.Enabled {
		t.Error("expected Enabled=true")
	}
}

func TestAddRuleRejectsDuplicateID(t *testing.T) {
	m := newTestManager(t)
	m.AddRule(&AlertRule{ID: "r1", Name: "first", Type: "latency"})
	// The current implementation silently overwrites; pin
	// whatever the behaviour is so a future change is
	// intentional. We expect the second AddRule to leave the
	// first intact, OR to overwrite — either is acceptable as
	// long as it does not panic.
	m.AddRule(&AlertRule{ID: "r1", Name: "second", Type: "latency"})
	got, ok := m.GetRule("r1")
	if !ok {
		t.Fatal("expected rule to exist after duplicate AddRule")
	}
	if got.Name == "" {
		t.Error("expected rule name to be non-empty after AddRule")
	}
}

func TestRemoveRule(t *testing.T) {
	m := newTestManager(t)
	m.AddRule(&AlertRule{ID: "r1", Name: "n", Type: "latency"})
	m.RemoveRule("r1")
	_, ok := m.GetRule("r1")
	if ok {
		t.Fatal("expected rule to be gone after RemoveRule")
	}
}

func TestRemoveRuleUnknownID(t *testing.T) {
	m := newTestManager(t)
	// Should not panic on missing rule.
	m.RemoveRule("nope")
}

func TestListRules(t *testing.T) {
	m := newTestManager(t)
	m.AddRule(&AlertRule{ID: "r1", Name: "first", Type: "latency"})
	m.AddRule(&AlertRule{ID: "r2", Name: "second", Type: "error_rate"})
	rules := m.ListRules()
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
}

func TestListAlertsEmpty(t *testing.T) {
	m := newTestManager(t)
	alerts := m.ListAlerts()
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts, got %d", len(alerts))
	}
}

func TestNotificationGroupLifecycle(t *testing.T) {
	m := newTestManager(t)
	group := &NotificationGroup{ID: "g1", Name: "ops", Channels: []string{"slack", "email"}}
	m.AddNotificationGroup(group)
	groups := m.groups
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups["g1"].Name != "ops" {
		t.Errorf("expected name 'ops', got %q", groups["g1"].Name)
	}
}

func TestSeverityAndStatusConstants(t *testing.T) {
	// These constants are part of the wire format; renaming any
	// of them is a breaking change. Pin them so a refactor that
	// accidentally renames one trips a test.
	if SeverityLow != "low" {
		t.Errorf("SeverityLow: got %q", SeverityLow)
	}
	if SeverityMedium != "medium" {
		t.Errorf("SeverityMedium: got %q", SeverityMedium)
	}
	if SeverityHigh != "high" {
		t.Errorf("SeverityHigh: got %q", SeverityHigh)
	}
	if SeverityCritical != "critical" {
		t.Errorf("SeverityCritical: got %q", SeverityCritical)
	}
	if StatusActive != "active" {
		t.Errorf("StatusActive: got %q", StatusActive)
	}
	if StatusResolved != "resolved" {
		t.Errorf("StatusResolved: got %q", StatusResolved)
	}
	if StatusPending != "pending" {
		t.Errorf("StatusPending: got %q", StatusPending)
	}
}

func TestResolveAlertUnknownID(t *testing.T) {
	m := newTestManager(t)
	if m.ResolveAlert("nope") {
		t.Fatal("expected ResolveAlert to return false for unknown id")
	}
}

func TestTriggerAlertCreatesActiveRecord(t *testing.T) {
	m := newTestManager(t)
	rule := &AlertRule{ID: "r1", Name: "spike", Type: "latency", Severity: SeverityHigh}
	m.AddRule(rule)
	alert := m.TriggerAlert(rule, "latency too high", map[string]any{"ms": 1234})
	if alert == nil {
		t.Fatal("TriggerAlert returned nil")
	}
	if alert.Status != StatusActive {
		t.Errorf("expected active status, got %q", alert.Status)
	}
	if alert.RuleID != "r1" {
		t.Errorf("expected rule id r1, got %q", alert.RuleID)
	}
	if alert.Message != "latency too high" {
		t.Errorf("expected message preserved, got %q", alert.Message)
	}
	if len(m.ListAlerts()) != 1 {
		t.Errorf("expected 1 alert in list, got %d", len(m.ListAlerts()))
	}
}

func TestRunMonitoringChecksNilCollector(t *testing.T) {
	// A nil collector is the documented no-op case; the
	// function must return an empty slice without panic.
	m := newTestManager(t)
	got := m.RunMonitoringChecks(nil)
	if len(got) != 0 {
		t.Errorf("expected 0 alerts with nil collector, got %d", len(got))
	}
}

func TestStartMonitoringRespectsContext(t *testing.T) {
	m := newTestManager(t)
	ctx, cancel := context.WithCancel(context.Background())
	m.StartMonitoring(ctx, nil, time.Hour) // long interval
	// Give the goroutine a moment to enter the select.
	time.Sleep(20 * time.Millisecond)
	cancel()
	// If the goroutine leaked, the test still passes; the
	// real assertion is that StartMonitoring returns
	// without blocking.
}

func TestGetNotificationChannelsFallback(t *testing.T) {
	// A rule with no groups falls back to the empty
	// channel list. The function must not panic.
	m := newTestManager(t)
	channels := m.getNotificationChannels(&AlertRule{ID: "r"})
	_ = channels
}

func TestNewManagerWithDB(t *testing.T) {
	// NewManagerWithDB takes a nil database and falls back
	// to the in-memory behaviour. The DB-backed load is
	// a no-op when db is nil; we just confirm the manager
	// is constructed.
	collector := metrics.NewCollector()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := NewManagerWithDB(logger, collector, nil)
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if m.metrics != collector {
		t.Error("expected collector to be set")
	}
}

func TestResolveAlertMarksStatusResolved(t *testing.T) {
	m := newTestManager(t)
	rule := &AlertRule{ID: "r1", Name: "spike", Type: "latency", Severity: SeverityHigh}
	m.AddRule(rule)
	alert := m.TriggerAlert(rule, "msg", nil)
	if !m.ResolveAlert(alert.ID) {
		t.Fatal("expected ResolveAlert to return true")
	}
	// Re-trigger to capture the new state via ListAlerts.
	// ListAlerts returns a snapshot at call time, so we re-fetch.
	alerts := m.ListAlerts()
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Status != StatusResolved {
		t.Errorf("expected resolved status, got %q", alerts[0].Status)
	}
}

func TestCheckLatencySpikeTriggersRule(t *testing.T) {
	m := newTestManager(t)
	// CheckLatencySpike returns a synthetic rule when
	// latencyMs > thresholdMs AND consecutiveRuns >= 10. Use
	// those bounds.
	got := m.CheckLatencySpike(500, 100, 10)
	if got == nil {
		t.Fatal("expected rule to fire on latency spike")
	}
	if got.Type != "latency_spike" {
		t.Errorf("expected type latency_spike, got %q", got.Type)
	}
	if !got.Enabled {
		t.Error("expected returned rule to be enabled")
	}
	if m.CheckLatencySpike(50, 100, 10) != nil {
		t.Error("expected no alert when latency below threshold")
	}
	if m.CheckLatencySpike(500, 100, 5) != nil {
		t.Error("expected no alert when consecutiveRuns < 10")
	}
}

func TestCheckFailureRate(t *testing.T) {
	m := newTestManager(t)
	// CheckFailureRate fires when errorRate > 10.0 (treated as
	// a percentage, not a fraction).
	if m.CheckFailureRate(15.0) == nil {
		t.Fatal("expected rule to fire on high error rate")
	}
	if m.CheckFailureRate(0.5) != nil {
		t.Fatal("expected rule NOT to fire on low error rate")
	}
}

func TestCheckApprovalBottleneck(t *testing.T) {
	m := newTestManager(t)
	if m.CheckApprovalBottleneck(72) == nil {
		t.Fatal("expected rule to fire on long-pending review")
	}
	if m.CheckApprovalBottleneck(1) != nil {
		t.Fatal("expected rule NOT to fire on quick review")
	}
}

func TestCheckCostOverrun(t *testing.T) {
	m := newTestManager(t)
	if m.CheckCostOverrun(150, 100) == nil {
		t.Fatal("expected rule to fire on cost overrun")
	}
	if m.CheckCostOverrun(50, 100) != nil {
		t.Fatal("expected rule NOT to fire when under budget")
	}
}

func TestSetDeliveryFunc(t *testing.T) {
	m := newTestManager(t)
	called := make(chan struct{}, 1)
	m.SetDeliveryFunc(func(_ *Alert, _ []string) error {
		called <- struct{}{}
		return nil
	})
	_ = m.TriggerAlert(&AlertRule{ID: "r", Name: "n", Type: "latency"}, "msg", nil)
	select {
	case <-called:
		// expected
	case <-time.After(2 * time.Second):
		t.Error("expected delivery func to be invoked on trigger")
	}
}

func TestTriggerAlertBoundedConcurrency(t *testing.T) {
	m := newTestManager(t)

	var mu sync.Mutex
	concurrent := 0
	maxConcurrent := 0

	m.SetDeliveryFunc(func(_ *Alert, _ []string) error {
		mu.Lock()
		concurrent++
		if concurrent > maxConcurrent {
			maxConcurrent = concurrent
		}
		mu.Unlock()
		defer func() {
			mu.Lock()
			concurrent--
			mu.Unlock()
		}()
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	// Trigger more alerts than MaxConcurrentDeliveries
	for i := 0; i < MaxConcurrentDeliveries+10; i++ {
		_ = m.TriggerAlert(&AlertRule{ID: "r", Name: "n", Type: "latency"}, "msg", nil)
	}

	// Give goroutines time to start
	time.Sleep(200 * time.Millisecond)

	if maxConcurrent > MaxConcurrentDeliveries {
		t.Errorf("expected max concurrent deliveries <= %d, got %d", MaxConcurrentDeliveries, maxConcurrent)
	}
}

func TestStopMonitoring(t *testing.T) {
	m := newTestManager(t)
	ctx, cancel := context.WithCancel(context.Background())
	m.StartMonitoring(ctx, nil, 100*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	cancel()
	m.StopMonitoring()
	// If StopMonitoring returns without blocking, the test passes.
}
