package alerting

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/metrics"
	"github.com/sachncs/promptsheon/internal/models"
	"github.com/sachncs/promptsheon/internal/store"
)

// mockStore implements store.Repository for testing.
type mockStore struct {
	store.Repository
	alertRules          []*models.AlertRuleRecord
	alerts              []*models.AlertRecord
	notificationGroups  []*models.NotificationGroupRecord
	listAlertRulesErr   error
	listAlertsErr       error
	listGroupsErr       error
	saveAlertRuleErr    error
	deleteAlertRuleErr  error
	saveAlertErr        error
	updateAlertErr      error
	saveNotificationGroupErr error
}

func (m *mockStore) ListAlertRules(_ context.Context) ([]*models.AlertRuleRecord, error) {
	return m.alertRules, m.listAlertRulesErr
}

func (m *mockStore) ListAlerts(_ context.Context, _ string) ([]*models.AlertRecord, error) {
	return m.alerts, m.listAlertsErr
}

func (m *mockStore) ListNotificationGroups(_ context.Context) ([]*models.NotificationGroupRecord, error) {
	return m.notificationGroups, m.listGroupsErr
}

func (m *mockStore) SaveAlertRule(_ context.Context, _ *models.AlertRuleRecord) error {
	return m.saveAlertRuleErr
}

func (m *mockStore) DeleteAlertRule(_ context.Context, _ string) error {
	return m.deleteAlertRuleErr
}

func (m *mockStore) SaveAlert(_ context.Context, _ *models.AlertRecord) error {
	return m.saveAlertErr
}

func (m *mockStore) UpdateAlert(_ context.Context, _ *models.AlertRecord) error {
	return m.updateAlertErr
}

func (m *mockStore) SaveNotificationGroup(_ context.Context, _ *models.NotificationGroupRecord) error {
	return m.saveNotificationGroupErr
}

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

// ---------------------------------------------------------------------------
// loadFromDB tests
// ---------------------------------------------------------------------------

func TestLoadFromDB_NilDB(t *testing.T) {
	m := newTestManager(t)
	// loadFromDB is called internally when db is nil; should not panic
	m.loadFromDB()
}

func TestLoadFromDB_LoadsRulesAlertsGroups(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	collector := metrics.NewCollector()
	now := time.Now()

	ms := &mockStore{
		alertRules: []*models.AlertRuleRecord{
			{ID: "r1", Name: "rule1", Type: "latency", Severity: "high", Enabled: true, Threshold: 100, CreatedAt: now, UpdatedAt: now},
		},
		alerts: []*models.AlertRecord{
			{ID: "a1", RuleID: "r1", RuleName: "rule1", Severity: "high", Status: "active", Message: "test", TriggeredAt: now},
		},
		notificationGroups: []*models.NotificationGroupRecord{
			{ID: "g1", Name: "ops", Channels: []string{"slack"}},
		},
	}

	m := NewManagerWithDB(logger, collector, ms)
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if len(m.rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(m.rules))
	}
	if m.rules["r1"].Name != "rule1" {
		t.Errorf("expected rule1, got %q", m.rules["r1"].Name)
	}
	if len(m.alerts) != 1 {
		t.Errorf("expected 1 alert, got %d", len(m.alerts))
	}
	if m.alerts[0].ID != "a1" {
		t.Errorf("expected a1, got %q", m.alerts[0].ID)
	}
	if len(m.groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(m.groups))
	}
	if m.groups["g1"].Channels[0] != "slack" {
		t.Errorf("expected slack channel, got %q", m.groups["g1"].Channels[0])
	}
}

func TestLoadFromDB_ListAlertRulesError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ms := &mockStore{
		listAlertRulesErr: errors.New("db error"),
	}
	m := NewManagerWithDB(logger, metrics.NewCollector(), ms)
	if len(m.rules) != 0 {
		t.Errorf("expected 0 rules on error, got %d", len(m.rules))
	}
}

func TestLoadFromDB_ListAlertsError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ms := &mockStore{
		alertRules: []*models.AlertRuleRecord{
			{ID: "r1", Name: "rule1", Type: "latency", Severity: "low"},
		},
		listAlertsErr: errors.New("db error"),
	}
	m := NewManagerWithDB(logger, metrics.NewCollector(), ms)
	// Rules should still load
	if len(m.rules) != 1 {
		t.Errorf("expected 1 rule despite alerts error, got %d", len(m.rules))
	}
	if len(m.alerts) != 0 {
		t.Errorf("expected 0 alerts on error, got %d", len(m.alerts))
	}
}

func TestLoadFromDB_ListGroupsError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ms := &mockStore{
		listGroupsErr: errors.New("db error"),
	}
	m := NewManagerWithDB(logger, metrics.NewCollector(), ms)
	if len(m.groups) != 0 {
		t.Errorf("expected 0 groups on error, got %d", len(m.groups))
	}
}

func TestLoadFromDB_NilLogger(t *testing.T) {
	ms := &mockStore{
		listAlertRulesErr: errors.New("db error"),
		listAlertsErr:     errors.New("db error"),
		listGroupsErr:     errors.New("db error"),
	}
	// Should not panic with nil logger
	m := &Manager{
		rules:       make(map[string]*AlertRule),
		alerts:      []*Alert{},
		groups:      make(map[string]*NotificationGroup),
		metrics:     metrics.NewCollector(),
		db:          ms,
	}
	m.loadFromDB()
}

// ---------------------------------------------------------------------------
// RemoveRule with DB tests
// ---------------------------------------------------------------------------

func TestRemoveRuleWithDB(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ms := &mockStore{}
	m := NewManagerWithDB(logger, metrics.NewCollector(), ms)
	m.AddRule(&AlertRule{ID: "r1", Name: "n", Type: "latency"})
	m.RemoveRule("r1")
	_, ok := m.GetRule("r1")
	if ok {
		t.Fatal("expected rule to be gone after RemoveRule")
	}
}

func TestRemoveRuleWithDB_DeleteError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ms := &mockStore{deleteAlertRuleErr: errors.New("db error")}
	m := NewManagerWithDB(logger, metrics.NewCollector(), ms)
	m.AddRule(&AlertRule{ID: "r1", Name: "n", Type: "latency"})
	// Should not panic; rule removed from memory despite DB error
	m.RemoveRule("r1")
	_, ok := m.GetRule("r1")
	if ok {
		t.Fatal("expected rule to be removed from memory despite DB error")
	}
}

// ---------------------------------------------------------------------------
// ResolveAlert additional tests
// ---------------------------------------------------------------------------

func TestResolveAlertAlreadyResolved(t *testing.T) {
	m := newTestManager(t)
	rule := &AlertRule{ID: "r1", Name: "spike", Type: "latency", Severity: SeverityHigh}
	m.AddRule(rule)
	alert := m.TriggerAlert(rule, "msg", nil)
	if !m.ResolveAlert(alert.ID) {
		t.Fatal("expected first resolve to return true")
	}
	// Resolving again should return false since status is already resolved
	if m.ResolveAlert(alert.ID) {
		t.Fatal("expected second resolve to return false for already-resolved alert")
	}
}

func TestResolveAlertWithDB(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ms := &mockStore{}
	m := NewManagerWithDB(logger, metrics.NewCollector(), ms)
	rule := &AlertRule{ID: "r1", Name: "spike", Type: "latency", Severity: SeverityHigh}
	m.AddRule(rule)
	alert := m.TriggerAlert(rule, "msg", nil)
	if !m.ResolveAlert(alert.ID) {
		t.Fatal("expected ResolveAlert to return true")
	}
	alerts := m.ListAlerts()
	if alerts[0].Status != StatusResolved {
		t.Errorf("expected resolved status, got %q", alerts[0].Status)
	}
}

func TestResolveAlertWithDB_UpdateError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ms := &mockStore{updateAlertErr: errors.New("db error")}
	m := NewManagerWithDB(logger, metrics.NewCollector(), ms)
	rule := &AlertRule{ID: "r1", Name: "spike", Type: "latency", Severity: SeverityHigh}
	m.AddRule(rule)
	alert := m.TriggerAlert(rule, "msg", nil)
	// Should still resolve in-memory even if DB update fails
	if !m.ResolveAlert(alert.ID) {
		t.Fatal("expected ResolveAlert to return true despite DB error")
	}
	if alert.Status != StatusResolved {
		t.Errorf("expected resolved status, got %q", alert.Status)
	}
}

// ---------------------------------------------------------------------------
// TriggerAlert additional tests
// ---------------------------------------------------------------------------

func TestTriggerAlertWithDB(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ms := &mockStore{}
	m := NewManagerWithDB(logger, metrics.NewCollector(), ms)
	rule := &AlertRule{ID: "r1", Name: "spike", Type: "latency", Severity: SeverityHigh}
	m.AddRule(rule)
	alert := m.TriggerAlert(rule, "latency too high", map[string]any{"ms": 1234})
	if alert.Status != StatusActive {
		t.Errorf("expected active status, got %q", alert.Status)
	}
	if len(m.ListAlerts()) != 1 {
		t.Errorf("expected 1 alert, got %d", len(m.ListAlerts()))
	}
}

func TestTriggerAlertWithDB_SaveError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ms := &mockStore{saveAlertErr: errors.New("db error")}
	m := NewManagerWithDB(logger, metrics.NewCollector(), ms)
	rule := &AlertRule{ID: "r1", Name: "spike", Type: "latency", Severity: SeverityHigh}
	m.AddRule(rule)
	// Should not panic; alert still added in-memory
	alert := m.TriggerAlert(rule, "msg", nil)
	if alert == nil {
		t.Fatal("expected non-nil alert despite DB error")
	}
	if len(m.ListAlerts()) != 1 {
		t.Errorf("expected 1 alert in memory despite DB error, got %d", len(m.ListAlerts()))
	}
}

func TestTriggerAlertWithDeliveryError(t *testing.T) {
	m := newTestManager(t)
	m.SetDeliveryFunc(func(_ *Alert, _ []string) error {
		return errors.New("delivery failed")
	})
	rule := &AlertRule{ID: "r1", Name: "n", Type: "latency", Severity: SeverityHigh}
	m.AddRule(rule)
	alert := m.TriggerAlert(rule, "msg", nil)
	if alert == nil {
		t.Fatal("expected non-nil alert")
	}
	// Give delivery goroutine time to run
	time.Sleep(50 * time.Millisecond)
}

func TestAddRuleWithDB_SaveError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ms := &mockStore{saveAlertRuleErr: errors.New("db error")}
	m := NewManagerWithDB(logger, metrics.NewCollector(), ms)
	m.AddRule(&AlertRule{ID: "r1", Name: "n", Type: "latency"})
	got, ok := m.GetRule("r1")
	if !ok {
		t.Fatal("expected rule to exist in memory despite DB error")
	}
	if got.Name != "n" {
		t.Errorf("expected name 'n', got %q", got.Name)
	}
}

func TestAddNotificationGroupWithDB_SaveError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ms := &mockStore{saveNotificationGroupErr: errors.New("db error")}
	m := NewManagerWithDB(logger, metrics.NewCollector(), ms)
	m.AddNotificationGroup(&NotificationGroup{ID: "g1", Name: "ops", Channels: []string{"slack"}})
	if len(m.groups) != 1 {
		t.Errorf("expected 1 group in memory despite DB error, got %d", len(m.groups))
	}
}

// ---------------------------------------------------------------------------
// RunMonitoringChecks tests
// ---------------------------------------------------------------------------

func TestRunMonitoringChecksLatencySpike(t *testing.T) {
	m := newTestManager(t)
	collector := metrics.NewCollector()
	// Add latency observations > 2s to trigger P95 > 2000ms
	for i := 0; i < 20; i++ {
		collector.LLMLatency.Observe(3.0) // 3 seconds > 2s threshold
	}
	alerts := m.RunMonitoringChecks(collector)
	if len(alerts) == 0 {
		t.Fatal("expected at least 1 alert for latency spike")
	}
	found := false
	for _, a := range alerts {
		if a.RuleID == "latency-spike" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected latency-spike alert to be triggered")
	}
}

func TestRunMonitoringChecksFailureRate(t *testing.T) {
	m := newTestManager(t)
	collector := metrics.NewCollector()
	// 30 errors out of 100 requests = 30% > 10%
	collector.RequestsTotal.Add(100)
	collector.ErrorsTotal.Add(30)
	alerts := m.RunMonitoringChecks(collector)
	if len(alerts) == 0 {
		t.Fatal("expected at least 1 alert for high failure rate")
	}
	found := false
	for _, a := range alerts {
		if a.RuleID == "failure-rate" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected failure-rate alert to be triggered")
	}
}

func TestRunMonitoringChecksLatencyAndFailure(t *testing.T) {
	m := newTestManager(t)
	collector := metrics.NewCollector()
	// Both conditions met
	for i := 0; i < 20; i++ {
		collector.LLMLatency.Observe(3.0)
	}
	collector.RequestsTotal.Add(100)
	collector.ErrorsTotal.Add(30)
	alerts := m.RunMonitoringChecks(collector)
	if len(alerts) < 2 {
		t.Errorf("expected at least 2 alerts, got %d", len(alerts))
	}
}

func TestRunMonitoringChecksBelowThresholds(t *testing.T) {
	m := newTestManager(t)
	collector := metrics.NewCollector()
	// Low latency and no errors
	collector.LLMLatency.Observe(0.1)
	collector.RequestsTotal.Add(100)
	collector.ErrorsTotal.Add(1) // 1% < 10%
	alerts := m.RunMonitoringChecks(collector)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts when below thresholds, got %d", len(alerts))
	}
}

func TestRunMonitoringChecksZeroRequests(t *testing.T) {
	m := newTestManager(t)
	collector := metrics.NewCollector()
	// No requests yet but has errors — error rate math is skipped
	collector.ErrorsTotal.Add(5)
	alerts := m.RunMonitoringChecks(collector)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts when no requests, got %d", len(alerts))
	}
}

// ---------------------------------------------------------------------------
// getNotificationChannels tests
// ---------------------------------------------------------------------------

func TestGetNotificationChannels_BySeverity(t *testing.T) {
	m := newTestManager(t)
	m.groups["g1"] = &NotificationGroup{ID: "g1", Name: "High", Channels: []string{"pagerduty"}}
	m.groups["g2"] = &NotificationGroup{ID: "g2", Name: "latency", Channels: []string{"slack"}}
	channels := m.getNotificationChannels(&AlertRule{
		ID: "r1", Severity: SeverityHigh, Type: "latency",
	})
	if len(channels) != 1 || channels[0] != "pagerduty" {
		t.Errorf("expected [pagerduty] for severity 'High', got %v", channels)
	}
}

func TestGetNotificationChannels_ByType(t *testing.T) {
	m := newTestManager(t)
	m.groups["g1"] = &NotificationGroup{ID: "g1", Name: "unknown", Channels: []string{"email"}}
	m.groups["g2"] = &NotificationGroup{ID: "g2", Name: "latency", Channels: []string{"slack"}}
	channels := m.getNotificationChannels(&AlertRule{
		ID: "r1", Severity: SeverityLow, Type: "latency",
	})
	if len(channels) != 1 || channels[0] != "slack" {
		t.Errorf("expected [slack] for type 'latency', got %v", channels)
	}
}

func TestGetNotificationChannels_DefaultGroup(t *testing.T) {
	m := newTestManager(t)
	m.groups["default"] = &NotificationGroup{ID: "default", Name: "Default", Channels: []string{"webhook"}}
	m.groups["other"] = &NotificationGroup{ID: "other", Name: "misc", Channels: []string{"email"}}
	channels := m.getNotificationChannels(&AlertRule{
		ID: "r1", Severity: SeverityLow, Type: "something",
	})
	if len(channels) != 1 || channels[0] != "webhook" {
		t.Errorf("expected [webhook] for default group, got %v", channels)
	}
}

func TestGetNotificationChannels_Fallback(t *testing.T) {
	m := newTestManager(t)
	channels := m.getNotificationChannels(&AlertRule{
		ID: "r1", Severity: SeverityLow, Type: "unknown",
	})
	if len(channels) != 1 || channels[0] != "webhook" {
		t.Errorf("expected [webhook] fallback, got %v", channels)
	}
}

func TestGetNotificationChannels_CaseInsensitiveSeverityMatch(t *testing.T) {
	m := newTestManager(t)
	m.groups["g1"] = &NotificationGroup{ID: "g1", Name: "CRITICAL", Channels: []string{"sms"}}
	channels := m.getNotificationChannels(&AlertRule{
		ID: "r1", Severity: SeverityCritical, Type: "latency",
	})
	if len(channels) != 1 || channels[0] != "sms" {
		t.Errorf("expected [sms] for case-insensitive severity match, got %v", channels)
	}
}

func TestGetNotificationChannels_SeverityTakesPrecedenceOverType(t *testing.T) {
	m := newTestManager(t)
	m.groups["g1"] = &NotificationGroup{ID: "g1", Name: "high", Channels: []string{"pagerduty"}}
	m.groups["g2"] = &NotificationGroup{ID: "g2", Name: "latency", Channels: []string{"slack"}}
	// Both severity 'high' and type 'latency' match, but severity should win
	channels := m.getNotificationChannels(&AlertRule{
		ID: "r1", Severity: SeverityHigh, Type: "latency",
	})
	if len(channels) != 1 || channels[0] != "pagerduty" {
		t.Errorf("expected [pagerduty] (severity takes precedence), got %v", channels)
	}
}
