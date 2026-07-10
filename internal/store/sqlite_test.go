package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/models"
)

func setupMemoryDB(t *testing.T) *SQLite {
	t.Helper()
	db, err := NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("NewSQLite(:memory:): %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestNewSQLite_Ping_DB_Close(t *testing.T) {
	db, err := NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}

	if err := db.Ping(context.Background()); err != nil {
		t.Errorf("Ping: %v", err)
	}

	if d := db.DB(); d == nil {
		t.Error("DB() returned nil")
	}

	if err := db.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestNewSQLite_ReOpenSameFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "reopen.db")

	db, err := NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	if err = db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Re-opening same file should re-run migrations with "already applied" path
	db2, err := NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite second: %v", err)
	}
	if err := db2.Ping(context.Background()); err != nil {
		t.Errorf("Ping: %v", err)
	}
	if err := db2.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestMarshalOrErr(t *testing.T) {
	b, err := marshalOrErr(map[string]string{"foo": "bar"})
	if err != nil {
		t.Fatalf("marshalOrErr: %v", err)
	}
	if string(b) != `{"foo":"bar"}` {
		t.Errorf("got %s", string(b))
	}

	_, err = marshalOrErr(make(chan struct{}))
	if err == nil {
		t.Error("expected error for unmarshalable type")
	}
}

func TestMustUnmarshal(t *testing.T) {
	var m map[string]string

	mustUnmarshal(nil, &m)
	mustUnmarshal([]byte{}, &m)

	m = make(map[string]string)
	mustUnmarshal([]byte(`{"key":"val"}`), &m)
	if m["key"] != "val" {
		t.Errorf("got %v", m)
	}

	mustUnmarshal([]byte(`invalid`), &m)
}

func TestUserCRUD(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	u := &models.User{
		ID:        "user-1",
		Email:     "alice@example.com",
		Name:      "Alice",
		Role:      "admin",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := db.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := db.GetUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.Email != "alice@example.com" || got.Name != "Alice" {
		t.Errorf("got %+v", got)
	}

	got2, err := db.GetUserByEmail(ctx, "alice@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if got2.ID != "user-1" {
		t.Errorf("got id %s", got2.ID)
	}

	list, err := db.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 user, got %d", len(list))
	}

	u.Name = "Alice Updated"
	u.UpdatedAt = time.Now().UTC()
	if err = db.UpdateUser(ctx, u); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	got, _ = db.GetUser(ctx, "user-1")
	if got.Name != "Alice Updated" {
		t.Errorf("after update: got name %q", got.Name)
	}

	if err = db.DeleteUser(ctx, "user-1"); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	_, err = db.GetUser(ctx, "user-1")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestUserErrors(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	_, err := db.GetUser(ctx, "no-such")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	_, err = db.GetUserByEmail(ctx, "no-such@example.com")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	u := &models.User{ID: "no-such", UpdatedAt: now}
	err = db.UpdateUser(ctx, u)
	if err == nil {
		t.Error("expected error updating non-existent user")
	}

	err = db.DeleteUser(ctx, "no-such")
	if err == nil {
		t.Error("expected error deleting non-existent user")
	}
}

func TestAPIKeyCRUD(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	_ = db.CreateUser(ctx, &models.User{
		ID: "user-1", Email: "a@b.com", Name: "A",
		Role: "admin", CreatedAt: now, UpdatedAt: now,
	})

	expires := now.Add(24 * time.Hour)
	key := &models.APIKey{
		ID:        "key-1",
		UserID:    "user-1",
		Name:      "Test Key",
		KeyHash:   "abc123hash",
		KeyPrefix: "abc123",
		Role:      "admin",
		ExpiresAt: &expires,
		CreatedAt: now,
	}

	if err := db.CreateAPIKey(ctx, key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	got, err := db.GetAPIKeyByHash(ctx, "abc123hash")
	if err != nil {
		t.Fatalf("GetAPIKeyByHash: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil key")
	}
	if got.Name != "Test Key" {
		t.Errorf("got name %q", got.Name)
	}

	got2, err := db.GetAPIKeyByID(ctx, "key-1")
	if err != nil {
		t.Fatalf("GetAPIKeyByID: %v", err)
	}
	if got2.ID != "key-1" {
		t.Errorf("got id %s", got2.ID)
	}

	list, err := db.ListAPIKeysByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("ListAPIKeysByUser: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 key, got %d", len(list))
	}

	if err = db.UpdateAPIKeyLastUsed(ctx, "key-1"); err != nil {
		t.Fatalf("UpdateAPIKeyLastUsed: %v", err)
	}
	got3, _ := db.GetAPIKeyByID(ctx, "key-1")
	if got3.LastUsed == nil {
		t.Error("expected LastUsed to be set")
	}

	got4, err := db.GetAPIKeyByHash(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetAPIKeyByHash non-existent: %v", err)
	}
	if got4 != nil {
		t.Error("expected nil for non-existent hash")
	}

	_, err = db.GetAPIKeyByID(ctx, "nonexistent")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}

	if err = db.DeleteAPIKey(ctx, "key-1"); err != nil {
		t.Fatalf("DeleteAPIKey: %v", err)
	}
	got5, _ := db.GetAPIKeyByID(ctx, "key-1")
	if got5 == nil {
		t.Fatal("expected key to exist but revoked")
	}
	if !got5.Revoked {
		t.Error("expected key to be revoked")
	}

	got6, err := db.GetAPIKeyByHash(ctx, "abc123hash")
	if err != nil {
		t.Fatalf("GetAPIKeyByHash after revoke: %v", err)
	}
	if got6 != nil {
		t.Error("expected nil for revoked key")
	}
}

func TestAuditCRUD(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()

	entry := &models.AuditEntry{
		ID:       "audit-1",
		UserID:   "user-1",
		Action:   "create",
		Resource: "prompt:abc",
		Details:  map[string]any{"key": "val"},
	}

	if err := db.AppendAudit(ctx, entry); err != nil {
		t.Fatalf("AppendAudit: %v", err)
	}
	if entry.EntryHash == "" {
		t.Error("expected entry_hash to be set")
	}
	if entry.PreviousHash != "" {
		t.Error("expected previous_hash to be empty for first entry")
	}
	if entry.Timestamp.IsZero() {
		t.Error("expected timestamp to be set")
	}

	entry2 := &models.AuditEntry{
		ID:       "audit-2",
		UserID:   "user-1",
		Action:   "update",
		Resource: "prompt:abc",
		Details:  map[string]any{"key": "val2"},
	}
	if err := db.AppendAudit(ctx, entry2); err != nil {
		t.Fatalf("AppendAudit entry2: %v", err)
	}
	if entry2.PreviousHash != entry.EntryHash {
		t.Errorf("expected previous_hash %q, got %q", entry.EntryHash, entry2.PreviousHash)
	}

	all, err := db.ListAudit(ctx, &models.AuditFilter{})
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}

	filtered, err := db.ListAudit(ctx, &models.AuditFilter{UserID: "user-1"})
	if err != nil {
		t.Fatalf("ListAudit with filter: %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2 filtered entries, got %d", len(filtered))
	}

	filtered2, err := db.ListAudit(ctx, &models.AuditFilter{Action: "create"})
	if err != nil {
		t.Fatalf("ListAudit with action filter: %v", err)
	}
	if len(filtered2) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(filtered2))
	}

	filtered3, err := db.ListAudit(ctx, &models.AuditFilter{Resource: "prompt:abc"})
	if err != nil {
		t.Fatalf("ListAudit with resource filter: %v", err)
	}
	if len(filtered3) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(filtered3))
	}

	since := time.Now().UTC().Add(-1 * time.Hour)
	until := time.Now().UTC().Add(1 * time.Hour)
	filtered4, err := db.ListAudit(ctx, &models.AuditFilter{Since: &since, Until: &until})
	if err != nil {
		t.Fatalf("ListAudit with time filter: %v", err)
	}
	if len(filtered4) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(filtered4))
	}

	filtered5, err := db.ListAudit(ctx, &models.AuditFilter{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("ListAudit with pagination: %v", err)
	}
	if len(filtered5) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(filtered5))
	}

	exported, err := db.ExportAudit(ctx, &models.AuditFilter{UserID: "user-1", Limit: 1, Offset: 0})
	if err != nil {
		t.Fatalf("ExportAudit: %v", err)
	}
	if len(exported) != 2 {
		t.Fatalf("ExportAudit should return all (limit=0), got %d", len(exported))
	}

	ok, reason, err := db.VerifyAuditChain(ctx)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if !ok {
		t.Errorf("expected chain valid, got reason: %s", reason)
	}
}

func TestAuditChainTampered(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()

	entry := &models.AuditEntry{
		ID:       "audit-1",
		UserID:   "user-1",
		Action:   "create",
		Resource: "prompt:abc",
		Details:  map[string]any{"key": "val"},
	}
	if err := db.AppendAudit(ctx, entry); err != nil {
		t.Fatalf("AppendAudit: %v", err)
	}

	if _, err := db.db.ExecContext(ctx, `UPDATE audit_entries SET entry_hash = 'tampered' WHERE id = 'audit-1'`); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	ok, reason, err := db.VerifyAuditChain(ctx)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if ok {
		t.Error("expected chain to be invalid after tampering")
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestComputeAuditHash(t *testing.T) {
	h := computeAuditHash(&models.AuditEntry{
		ID:           "test-id",
		UserID:       "test-user",
		Action:       "test-action",
		Resource:     "test-resource",
		PreviousHash: "prev-hash",
	}, `{"key":"val"}`, "2024-01-01T00:00:00Z")
	if h == "" {
		t.Error("expected non-empty hash")
	}
	h2 := computeAuditHash(&models.AuditEntry{
		ID:           "test-id",
		UserID:       "test-user",
		Action:       "test-action",
		Resource:     "test-resource",
		PreviousHash: "prev-hash",
	}, `{"key":"val"}`, "2024-01-01T00:00:00Z")
	if h != h2 {
		t.Error("expected deterministic hash")
	}
}

func TestProviderKeyCRUD(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	pk := &models.ProviderKey{
		ID:           "pk-1",
		ProviderName: "openai",
		KeyName:      "default",
		EncryptedKey: "encrypted-value",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := db.SaveProviderKey(ctx, pk); err != nil {
		t.Fatalf("SaveProviderKey: %v", err)
	}

	got, err := db.GetProviderKey(ctx, "pk-1")
	if err != nil {
		t.Fatalf("GetProviderKey: %v", err)
	}
	if got.ProviderName != "openai" {
		t.Errorf("got provider %q", got.ProviderName)
	}

	got2, err := db.GetProviderKeyByName(ctx, "openai", "default")
	if err != nil {
		t.Fatalf("GetProviderKeyByName: %v", err)
	}
	if got2.ID != "pk-1" {
		t.Errorf("got id %s", got2.ID)
	}

	list, err := db.ListProviderKeys(ctx)
	if err != nil {
		t.Fatalf("ListProviderKeys: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 key, got %d", len(list))
	}

	pk.EncryptedKey = "new-encrypted"
	pk.UpdatedAt = time.Now().UTC()
	if err = db.SaveProviderKey(ctx, pk); err != nil {
		t.Fatalf("SaveProviderKey update: %v", err)
	}
	got3, _ := db.GetProviderKey(ctx, "pk-1")
	if got3.EncryptedKey != "new-encrypted" {
		t.Errorf("got encrypted_key %q", got3.EncryptedKey)
	}

	if err = db.DeleteProviderKey(ctx, "pk-1"); err != nil {
		t.Fatalf("DeleteProviderKey: %v", err)
	}
	_, err = db.GetProviderKey(ctx, "pk-1")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestProviderKeyErrors(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()

	_, err := db.GetProviderKey(ctx, "no-such")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	_, err = db.GetProviderKeyByName(ctx, "no-such", "no-such")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	err = db.DeleteProviderKey(ctx, "no-such")
	if err == nil {
		t.Error("expected error deleting non-existent key")
	}
}

func TestAlertRuleCRUD(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	r := &models.AlertRuleRecord{
		ID:        "rule-1",
		Name:      "High CPU",
		Type:      "metric",
		Severity:  "critical",
		Enabled:   true,
		Threshold: 90.0,
		Duration:  5,
		Window:    10,
		Config:    map[string]any{"metric": "cpu"},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := db.SaveAlertRule(ctx, r); err != nil {
		t.Fatalf("SaveAlertRule: %v", err)
	}

	got, err := db.GetAlertRule(ctx, "rule-1")
	if err != nil {
		t.Fatalf("GetAlertRule: %v", err)
	}
	if got.Name != "High CPU" || got.Threshold != 90.0 {
		t.Errorf("got %+v", got)
	}
	if got.Config["metric"] != "cpu" {
		t.Errorf("config mismatch")
	}

	list, err := db.ListAlertRules(ctx)
	if err != nil {
		t.Fatalf("ListAlertRules: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(list))
	}

	if err = db.DeleteAlertRule(ctx, "rule-1"); err != nil {
		t.Fatalf("DeleteAlertRule: %v", err)
	}
	_, err = db.GetAlertRule(ctx, "rule-1")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestAlertRuleNotFound(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()

	_, err := db.GetAlertRule(ctx, "no-such")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestAlertCRUD(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	a := &models.AlertRecord{
		ID:          "alert-1",
		RuleID:      "rule-1",
		RuleName:    "High CPU",
		Severity:    "critical",
		Status:      "active",
		Message:     "CPU is at 95%",
		Details:     map[string]any{"value": 95.0},
		TriggeredAt: now,
	}

	if err := db.SaveAlert(ctx, a); err != nil {
		t.Fatalf("SaveAlert: %v", err)
	}

	got, err := db.GetAlert(ctx, "alert-1")
	if err != nil {
		t.Fatalf("GetAlert: %v", err)
	}
	if got.Message != "CPU is at 95%" {
		t.Errorf("got message %q", got.Message)
	}
	if got.Details["value"] != 95.0 {
		t.Errorf("details mismatch")
	}

	resolved := time.Now().UTC()
	a.Status = "resolved"
	a.ResolvedAt = &resolved
	if err = db.UpdateAlert(ctx, a); err != nil {
		t.Fatalf("UpdateAlert: %v", err)
	}
	got2, _ := db.GetAlert(ctx, "alert-1")
	if got2.Status != "resolved" {
		t.Errorf("expected resolved status")
	}
	if got2.ResolvedAt == nil {
		t.Error("expected ResolvedAt to be set")
	}

	list, err := db.ListAlerts(ctx, "")
	if err != nil {
		t.Fatalf("ListAlerts: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(list))
	}

	list2, err := db.ListAlerts(ctx, "resolved")
	if err != nil {
		t.Fatalf("ListAlerts filtered: %v", err)
	}
	if len(list2) != 1 {
		t.Fatalf("expected 1 resolved alert, got %d", len(list2))
	}

	list3, err := db.ListAlerts(ctx, "active")
	if err != nil {
		t.Fatalf("ListAlerts active: %v", err)
	}
	if len(list3) != 0 {
		t.Fatalf("expected 0 active alerts, got %d", len(list3))
	}
}

func TestAlertErrors(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()

	_, err := db.GetAlert(ctx, "no-such")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	a := &models.AlertRecord{ID: "no-such"}
	err = db.UpdateAlert(ctx, a)
	if err == nil {
		t.Error("expected error updating non-existent alert")
	}
}

func TestNotificationGroupCRUD(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()

	g := &models.NotificationGroupRecord{
		ID:       "ng-1",
		Name:     "On-call",
		Channels: []string{"email", "slack"},
	}

	if err := db.SaveNotificationGroup(ctx, g); err != nil {
		t.Fatalf("SaveNotificationGroup: %v", err)
	}

	got, err := db.GetNotificationGroup(ctx, "ng-1")
	if err != nil {
		t.Fatalf("GetNotificationGroup: %v", err)
	}
	if got.Name != "On-call" {
		t.Errorf("got name %q", got.Name)
	}
	if len(got.Channels) != 2 {
		t.Errorf("expected 2 channels, got %d", len(got.Channels))
	}

	list, err := db.ListNotificationGroups(ctx)
	if err != nil {
		t.Fatalf("ListNotificationGroups: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 group, got %d", len(list))
	}

	if err = db.DeleteNotificationGroup(ctx, "ng-1"); err != nil {
		t.Fatalf("DeleteNotificationGroup: %v", err)
	}
	_, err = db.GetNotificationGroup(ctx, "ng-1")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestNotificationGroupNotFound(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()

	_, err := db.GetNotificationGroup(ctx, "no-such")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestWebhookEndpointCRUD(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	ep := &models.WebhookEndpointRecord{
		ID:        "wh-1",
		URL:       "https://example.com/hook",
		Secret:    "secret123",
		Events:    []string{"alert.created", "alert.resolved"},
		Active:    true,
		CreatedAt: now,
	}

	if err := db.SaveWebhookEndpoint(ctx, ep); err != nil {
		t.Fatalf("SaveWebhookEndpoint: %v", err)
	}

	got, err := db.GetWebhookEndpoint(ctx, "wh-1")
	if err != nil {
		t.Fatalf("GetWebhookEndpoint: %v", err)
	}
	if got.URL != "https://example.com/hook" {
		t.Errorf("got url %q", got.URL)
	}
	if len(got.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(got.Events))
	}
	if !got.Active {
		t.Error("expected endpoint to be active")
	}

	list, err := db.ListWebhookEndpoints(ctx)
	if err != nil {
		t.Fatalf("ListWebhookEndpoints: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(list))
	}

	ep.Active = false
	if err = db.SaveWebhookEndpoint(ctx, ep); err != nil {
		t.Fatalf("SaveWebhookEndpoint update: %v", err)
	}
	got2, _ := db.GetWebhookEndpoint(ctx, "wh-1")
	if got2.Active {
		t.Error("expected endpoint to be inactive")
	}

	if err = db.DeleteWebhookEndpoint(ctx, "wh-1"); err != nil {
		t.Fatalf("DeleteWebhookEndpoint: %v", err)
	}
	_, err = db.GetWebhookEndpoint(ctx, "wh-1")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestWebhookEndpointNotFound(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()

	_, err := db.GetWebhookEndpoint(ctx, "no-such")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestScanFunctionsWithBadJSON(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// Insert bad JSON manually to exercise JSON error paths in scan functions
	_, err := db.db.ExecContext(ctx,
		`INSERT INTO audit_entries (id, user_id, action, resource, details, timestamp, previous_hash, entry_hash, timestamp_str)
		 VALUES ('bad-audit', 'u', 'act', 'res', 'NOT VALID JSON', ?, '', '', '')`,
		now,
	)
	if err != nil {
		t.Fatalf("insert bad audit: %v", err)
	}
	_, err = db.ListAudit(ctx, &models.AuditFilter{})
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}

	_, err = db.db.ExecContext(ctx,
		`INSERT INTO alert_rules (id, name, type, severity, config, created_at, updated_at)
		 VALUES ('bad-rule', 'name', 'type', 'sev', 'NOT VALID JSON', ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatalf("insert bad rule: %v", err)
	}
	_, err = db.ListAlertRules(ctx)
	if err != nil {
		t.Fatalf("ListAlertRules: %v", err)
	}

	_, err = db.db.ExecContext(ctx,
		`INSERT INTO alerts (id, rule_id, rule_name, severity, message, details, triggered_at)
		 VALUES ('bad-alert', 'rule', 'name', 'sev', 'msg', 'NOT VALID JSON', ?)`,
		now,
	)
	if err != nil {
		t.Fatalf("insert bad alert: %v", err)
	}
	_, err = db.ListAlerts(ctx, "")
	if err != nil {
		t.Fatalf("ListAlerts: %v", err)
	}

	_, err = db.db.ExecContext(ctx,
		`INSERT INTO notification_groups (id, name, channels)
		 VALUES ('bad-ng', 'name', 'NOT VALID JSON')`,
	)
	if err != nil {
		t.Fatalf("insert bad ng: %v", err)
	}
	_, err = db.ListNotificationGroups(ctx)
	if err != nil {
		t.Fatalf("ListNotificationGroups: %v", err)
	}

	_, err = db.db.ExecContext(ctx,
		`INSERT INTO capabilities (id, project_id, name, state, tags, created_at, updated_at)
		 VALUES ('bad-cap', 'proj', 'name', 'draft', 'NOT VALID JSON', ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatalf("insert bad capability: %v", err)
	}
	_, err = db.ListCapabilities(ctx, "proj")
	if err != nil {
		t.Fatalf("ListCapabilities: %v", err)
	}

	_, err = db.db.ExecContext(ctx,
		`INSERT INTO executions (id, capability_version_id, timestamp, inputs, outputs)
		 VALUES ('bad-exec', 'ver', ?, 'NOT VALID JSON', 'NOT VALID JSON')`,
		now,
	)
	if err != nil {
		t.Fatalf("insert bad execution: %v", err)
	}
	_, err = db.ListExecutions(ctx, ExecutionFilter{})
	if err != nil {
		t.Fatalf("ListExecutions: %v", err)
	}
}

func TestEmptyListOperations(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()

	entries, err := db.ListAudit(ctx, &models.AuditFilter{})
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}

	ok, reason, err := db.VerifyAuditChain(ctx)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if !ok {
		t.Errorf("expected valid empty chain, reason: %s", reason)
	}

	users, err := db.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 0 {
		t.Errorf("expected 0 users, got %d", len(users))
	}

	keys, err := db.ListAPIKeysByUser(ctx, "no-such")
	if err != nil {
		t.Fatalf("ListAPIKeysByUser: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}

	pkeys, err := db.ListProviderKeys(ctx)
	if err != nil {
		t.Fatalf("ListProviderKeys: %v", err)
	}
	if len(pkeys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(pkeys))
	}

	rules, err := db.ListAlertRules(ctx)
	if err != nil {
		t.Fatalf("ListAlertRules: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(rules))
	}

	alerts, err := db.ListAlerts(ctx, "")
	if err != nil {
		t.Fatalf("ListAlerts: %v", err)
	}
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(alerts))
	}

	groups, err := db.ListNotificationGroups(ctx)
	if err != nil {
		t.Fatalf("ListNotificationGroups: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}

	eps, err := db.ListWebhookEndpoints(ctx)
	if err != nil {
		t.Fatalf("ListWebhookEndpoints: %v", err)
	}
	if len(eps) != 0 {
		t.Errorf("expected 0 endpoints, got %d", len(eps))
	}
}

func TestMarshalField(t *testing.T) {
	s, err := marshalField(map[string]string{"a": "b"})
	if err != nil {
		t.Fatalf("marshalField: %v", err)
	}
	if s != `{"a":"b"}` {
		t.Errorf("got %s", s)
	}

	_, err = marshalField(make(chan struct{}))
	if err == nil {
		t.Error("expected error for unmarshalable type")
	}
}

func TestMarshalCapabilityVersionJSONFieldsError(t *testing.T) {
	// marshalCapabilityVersionJSONFields marshals each sub-field in sequence.
	// If any fails it returns immediately. We can't easily trigger a marshal
	// error from a real capability.Version (all fields are structs/maps/slices),
	// so we just verify the happy path returns without error.
	v := &capability.Version{
		Prompt:    capability.Prompt{Instructions: "test"},
		CreatedAt: time.Now().UTC(),
	}
	f, err := marshalCapabilityVersionJSONFields(v)
	if err != nil {
		t.Fatalf("marshalCapabilityVersionJSONFields: %v", err)
	}
	if f.prompt == "" {
		t.Error("expected non-empty prompt JSON")
	}
}

func TestCapabilityStore_UpdateProject(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := db.CreateWorkspace(ctx, &capability.Workspace{
		ID: "ws-1", Name: "Test", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}

	p := &capability.Project{
		ID: "proj-1", WorkspaceID: "ws-1", Name: "Original",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	p.Name = "Updated"
	p.Description = "New desc"
	p.UpdatedAt = time.Now().UTC()
	if err := db.UpdateProject(ctx, p); err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}

	got, err := db.GetProject(ctx, "proj-1")
	if err != nil {
		t.Fatalf("GetProject after update: %v", err)
	}
	if got.Name != "Updated" {
		t.Errorf("got name %q, want %q", got.Name, "Updated")
	}
	if got.Description != "New desc" {
		t.Errorf("got desc %q", got.Description)
	}
}

func TestCapabilityStore_UpdateWorkspace(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := db.CreateWorkspace(ctx, &capability.Workspace{
		ID: "ws-1", Name: "Original", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}

	if err := db.UpdateWorkspace(ctx, &capability.Workspace{
		ID: "ws-1", Name: "Updated", Organization: "Org",
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpdateWorkspace: %v", err)
	}

	got, err := db.GetWorkspace(ctx, "ws-1")
	if err != nil {
		t.Fatalf("GetWorkspace: %v", err)
	}
	if got.Name != "Updated" {
		t.Errorf("got name %q", got.Name)
	}
}

func TestCapabilityStore_DeleteWorkspaceAndProject(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := db.CreateWorkspace(ctx, &capability.Workspace{
		ID: "ws-1", Name: "Test", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	if err := db.CreateProject(ctx, &capability.Project{
		ID: "proj-1", WorkspaceID: "ws-1", Name: "Test",
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if err := db.DeleteProject(ctx, "proj-1"); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if err := db.DeleteWorkspace(ctx, "ws-1"); err != nil {
		t.Fatalf("DeleteWorkspace: %v", err)
	}

	_, err := db.GetWorkspace(ctx, "ws-1")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound for deleted workspace, got %v", err)
	}
	_, err = db.GetProject(ctx, "proj-1")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound for deleted project, got %v", err)
	}
}

func TestCapabilityStore_DeleteCapability(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	setupWorkspaceAndProject(t, db, now)

	if err := db.CreateCapability(ctx, &capability.Capability{
		ID: "cap-del", ProjectID: "proj-1", Name: "To Delete",
		State: capability.StateDraft, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateCapability: %v", err)
	}

	if err := db.DeleteCapability(ctx, "cap-del"); err != nil {
		t.Fatalf("DeleteCapability: %v", err)
	}
	_, err := db.GetCapability(ctx, "cap-del")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCapabilityStore_ScanCapabilityVersionBadJSON(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	setupCapability(t, db, now)

	// Insert a version with bad JSON in one of the JSON fields to exercise
	// the slog.Error path in scanCapabilityVersion.
	_, err := db.db.ExecContext(ctx, `
		INSERT INTO capability_versions (id, capability_id, version, prompt, model_policy, context_contract, knowledge, memory, guardrails, tools, mcp_servers, runtime_policy, evaluation_suite, created_at, created_by)
		VALUES ('ver-bad', 'cap-1', 99, 'NOT VALID JSON', '{}', '{}', '[]', '{}', '[]', '[]', '[]', '{}', '{}', ?, 'test')`,
		now,
	)
	if err != nil {
		t.Fatalf("insert bad version: %v", err)
	}
	_, err = db.ListVersions(ctx, "cap-1")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	_, err = db.GetVersion(ctx, "ver-bad")
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
}

func TestCreateDuplicateUser(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	u := &models.User{
		ID: "dup-user", Email: "dup@example.com", Name: "Dup",
		Role: "admin", CreatedAt: now, UpdatedAt: now,
	}
	if err := db.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	// Second insert with same ID should fail
	err := db.CreateUser(ctx, u)
	if err == nil {
		t.Error("expected error for duplicate user")
	}
}

func TestCreateDuplicateAPIKey(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	_ = db.CreateUser(ctx, &models.User{
		ID: "user-dup", Email: "dup@example.com", Name: "Dup",
		Role: "admin", CreatedAt: now, UpdatedAt: now,
	})

	key := &models.APIKey{
		ID: "dup-key", UserID: "user-dup", Name: "Dup",
		KeyHash: "hash123", KeyPrefix: "pref", Role: "admin",
		CreatedAt: now,
	}
	if err := db.CreateAPIKey(ctx, key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	err := db.CreateAPIKey(ctx, key)
	if err == nil {
		t.Error("expected error for duplicate API key")
	}
}

func TestAppendAuditMarshalError(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()

	entry := &models.AuditEntry{
		ID:       "audit-bad",
		UserID:   "u",
		Action:   "act",
		Resource: "res",
		Details:  map[string]any{"ch": make(chan struct{})},
	}
	err := db.AppendAudit(ctx, entry)
	if err == nil {
		t.Error("expected error for unmarshalable details")
	}
}

func TestNewSQLite_InvalidPath(t *testing.T) {
	dir := t.TempDir()
	_, err := NewSQLite(dir)
	if err != nil {
		t.Logf("NewSQLite with directory path returned: %v", err)
	}
}

func TestAuditChainBreakOnSecondEntry(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()

	entry := &models.AuditEntry{
		ID: "audit-1", UserID: "user-1", Action: "create", Resource: "res",
	}
	if err := db.AppendAudit(ctx, entry); err != nil {
		t.Fatalf("AppendAudit: %v", err)
	}

	// Append a second entry, then corrupt its previous_hash
	entry2 := &models.AuditEntry{
		ID: "audit-2", UserID: "user-1", Action: "update", Resource: "res",
	}
	if err := db.AppendAudit(ctx, entry2); err != nil {
		t.Fatalf("AppendAudit entry2: %v", err)
	}

	if _, err := db.db.ExecContext(ctx, `UPDATE audit_entries SET previous_hash = 'wrong' WHERE id = 'audit-2'`); err != nil {
		t.Fatalf("corrupt prev_hash: %v", err)
	}

	ok, reason, err := db.VerifyAuditChain(ctx)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if ok {
		t.Error("expected chain to be invalid after corrupting previous_hash")
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestCreateExecutionWithBadData(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// Inputs/Outputs containing unmarshalable values should trigger marshal errors
	e := &capability.Execution{
		ID:                  "exec-bad",
		CapabilityVersionID: "ver-1",
		Timestamp:           now,
		Inputs:              map[string]any{"ch": make(chan struct{})},
		Outputs:             map[string]any{"ch": make(chan struct{})},
	}

	err := db.CreateExecution(ctx, e)
	if err == nil {
		t.Error("expected error for unmarshalable inputs")
	}
}

func TestCreateCapabilityAndVersionEdgeCases(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	setupWorkspaceAndProject(t, db, now)

	// Create capability with empty tags
	c := &capability.Capability{
		ID: "cap-edge", ProjectID: "proj-1", Name: "Edge",
		State: capability.StateDraft, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.CreateCapability(ctx, c); err != nil {
		t.Fatalf("CreateCapability: %v", err)
	}

	// Update capability
	c.Name = "Edge Updated"
	if err := db.UpdateCapability(ctx, c); err != nil {
		t.Fatalf("UpdateCapability: %v", err)
	}

	got, err := db.GetCapability(ctx, "cap-edge")
	if err != nil {
		t.Fatalf("GetCapability: %v", err)
	}
	if got.Name != "Edge Updated" {
		t.Errorf("got name %q", got.Name)
	}
}

func TestListExecutionsWithOffset(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	setupCapability(t, db, now)
	v := &capability.Version{
		ID: "ver-offset", CapabilityID: "cap-1", Version: 1,
		Prompt:    capability.Prompt{Instructions: "test"},
		CreatedAt: now,
	}
	if err := db.CreateVersion(ctx, v); err != nil {
		t.Fatalf("CreateVersion: %v", err)
	}

	for i := 0; i < 3; i++ {
		e := &capability.Execution{
			ID:                  fmt.Sprintf("exec-%d", i),
			CapabilityVersionID: "ver-offset",
			Timestamp:           now.Add(time.Duration(i) * time.Second),
			Inputs:              map[string]any{"n": i},
			Outputs:             map[string]any{"result": i * 2},
		}
		if err := db.CreateExecution(ctx, e); err != nil {
			t.Fatalf("CreateExecution %d: %v", i, err)
		}
	}

	all, err := db.ListExecutions(ctx, ExecutionFilter{})
	if err != nil {
		t.Fatalf("ListExecutions: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 executions, got %d", len(all))
	}

	offset := ExecutionFilter{Limit: 2, Offset: 1}
	paginated, err := db.ListExecutions(ctx, offset)
	if err != nil {
		t.Fatalf("ListExecutions with offset: %v", err)
	}
	if len(paginated) != 2 {
		t.Fatalf("expected 2 executions with offset, got %d", len(paginated))
	}
}

func TestScanExecutionZeroTimestamp(t *testing.T) {
	db := setupMemoryDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	setupCapability(t, db, now)
	v := &capability.Version{
		ID: "ver-zt", CapabilityID: "cap-1", Version: 1,
		Prompt:    capability.Prompt{Instructions: "test"},
		CreatedAt: now,
	}
	if err := db.CreateVersion(ctx, v); err != nil {
		t.Fatalf("CreateVersion: %v", err)
	}

	_, err := db.db.ExecContext(ctx,
		`INSERT INTO executions (id, capability_version_id, timestamp)
		 VALUES ('exec-zt', 'ver-zt', ?)`,
		time.Time{},
	)
	if err != nil {
		t.Fatalf("insert zero-timestamp execution: %v", err)
	}

	e, err := db.GetExecution(ctx, "exec-zt")
	if err != nil {
		t.Fatalf("GetExecution: %v", err)
	}
	if e.Timestamp.IsZero() {
		t.Error("expected timestamp to be set to non-zero after scan")
	}
}
