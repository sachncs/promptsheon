package store_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/models"
	"github.com/sachncs/promptsheon/internal/release"
	"github.com/sachncs/promptsheon/internal/schedule"
	"github.com/sachncs/promptsheon/internal/store"
)

func init() {
	// Test runs apply every migration including the destructive one.
	// Production refuses by default; the test environment opts in.
	os.Setenv(store.DestructiveMigrationEnv, "true")
}

func newTestSQLite(t *testing.T) *store.SQLite {
	t.Helper()
	s, err := store.NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	// Seed the three default users ("u1", "u2", "u3") that the
	// legacy test fixtures expect. The post-043 FK on
	// audit_entries.user_id and guardrail_violations.user_id
	// rejects writes for unknown users. The seed is idempotent:
	// it leaves a user row alone if the test created it first.
	seedDefaultUsers(t, s)
	return s
}

// seedDefaultUsers creates "u1", "u2", "u3" if they don't already
// exist. Tests that supply their own "u1" record (with a specific
// email or role) are not stomped: the seed is a no-op for that id.
func seedDefaultUsers(t *testing.T, s *store.SQLite) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	for _, id := range []string{"u1", "u2", "u3"} {
		if _, err := s.GetUser(ctx, id); err == nil {
			continue
		}
		if err := s.CreateUser(ctx, &models.User{
			ID:        id,
			Email:     id + "@test.local",
			Name:      "Test User " + id,
			Role:      "admin",
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			t.Fatalf("seed user %s: %v", id, err)
		}
	}
}

// seedScheduleFixture inserts the minimum workspace + capability +
// version + release needed to satisfy the post-043 schedules FKs.
// TestScheduleCRUD is the only caller today; other tests that
// touch schedules should call this helper explicitly.
func seedScheduleFixture(t *testing.T, s *store.SQLite) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	if err := s.CreateWorkspace(ctx, &capability.Workspace{
		ID: "w1", Name: "test", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if err := s.CreateProject(ctx, &capability.Project{
		ID: "p1", WorkspaceID: "w1", Name: "test", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if err := s.CreateCapability(ctx, &capability.Capability{
		ID: "c1", ProjectID: "p1", Name: "test", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed capability: %v", err)
	}
	manifest := capability.Manifest{}
	if err := s.CreateVersion(ctx, &capability.Version{
		ID: "v1", CapabilityID: "c1", Version: 1, Manifest: manifest,
		ManifestHash: "h1", CreatedAt: now, CreatedBy: "u1",
	}); err != nil {
		t.Fatalf("seed version: %v", err)
	}
	if err := s.CreateRelease(ctx, &release.Release{
		ID: "r1", CapabilityID: "c1", CapabilityVersion: 1, Manifest: manifest,
		Environment: release.EnvProd, Status: release.StatusActive,
		ApprovedBy:  []string{"u1"}, CreatedAt: now, CreatedBy: "u1",
	}); err != nil {
		t.Fatalf("seed release: %v", err)
	}
}

func TestNewSQLiteAndClose(t *testing.T) {
	t.Parallel()
	s := newTestSQLite(t)
	if err := s.Ping(context.Background()); err != nil {
		t.Errorf("Ping: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestNewSQLiteRunsAllMigrations(t *testing.T) {
	t.Parallel()
	s := newTestSQLite(t)
	// Apply no further migrations; verify all known ones are recorded.
	rows, err := s.DB().Query("SELECT COUNT(*) FROM schema_migrations")
	if err != nil {
		t.Fatalf("query migrations: %v", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		t.Fatal("no rows from schema_migrations")
	}
	var n int
	if err := rows.Scan(&n); err != nil {
		t.Fatalf("scan: %v", err)
	}
	// After consolidation the migration count is 8.
	if n != 8 {
		t.Errorf("migrations applied = %d, want 8", n)
	}
}

func TestUserRoundTrip(t *testing.T) {
	t.Parallel()
	s := newTestSQLite(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	// Use a unique user id; the post-043 seedDefaultUsers pre-creates
	// "u1", "u2", "u3" so other tests can write audit / schedule rows
	// that FK against users. This test exercises the round-trip
	// lifecycle for a single user and is OK with any non-seeded id.
	u := &models.User{
		ID:        "alice",
		Email:     "alice@example.com",
		Name:      "Alice",
		Role:      "admin",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := s.GetUser(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.Email != u.Email || got.Name != u.Name {
		t.Errorf("GetUser mismatch: got %+v want %+v", got, u)
	}

	byMail, err := s.GetUserByEmail(ctx, "alice@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if byMail.ID != "alice" {
		t.Errorf("GetUserByEmail id = %q, want alice", byMail.ID)
	}

	users, err := s.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	// The post-043 seedDefaultUsers pre-creates three users
	// ("u1", "u2", "u3") for tests that need FK-respecting
	// parents. We expect at least 4: the three seeds + alice.
	if len(users) < 4 {
		t.Errorf("ListUsers count = %d, want >= 4 (3 seeds + alice)", len(users))
	}
	var found *models.User
	for _, x := range users {
		if x.ID == "alice" {
			found = x
			break
		}
	}
	if found == nil {
		t.Errorf("ListUsers did not return alice")
	}

	u.Name = "AliceUpdated"
	u.UpdatedAt = now.Add(time.Minute)
	if err := s.UpdateUser(ctx, u); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	got, _ = s.GetUser(ctx, "alice")
	if got.Name != "AliceUpdated" {
		t.Errorf("after UpdateUser name = %q, want AliceUpdated", got.Name)
	}

	if err := s.DeleteUser(ctx, "alice"); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	_, err = s.GetUser(ctx, "alice")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetUser after delete: err = %v, want ErrNotFound", err)
	}
}

func TestAPIKeyRoundTrip(t *testing.T) {
	t.Parallel()
	s := newTestSQLite(t)
	ctx := context.Background()
	expires := time.Now().UTC().Add(time.Hour)
	k := &models.APIKey{
		ID:        "k1",
		UserID:    "u1",
		Name:      "primary",
		KeyHash:   "h-1",
		KeyPrefix: "psn_aaaa",
		Role:      "admin",
		ExpiresAt: &expires,
	}
	if err := s.CreateAPIKey(ctx, k); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	byHash, err := s.GetAPIKeyByHash(ctx, "h-1")
	if err != nil {
		t.Fatalf("GetAPIKeyByHash: %v", err)
	}
	if byHash.ID != "k1" {
		t.Errorf("GetAPIKeyByHash id = %q, want k1", byHash.ID)
	}

	byID, err := s.GetAPIKeyByID(ctx, "k1")
	if err != nil {
		t.Fatalf("GetAPIKeyByID: %v", err)
	}
	if byID.KeyHash != "h-1" {
		t.Errorf("GetAPIKeyByID hash = %q, want h-1", byID.KeyHash)
	}

	list, err := s.ListAPIKeysByUser(ctx, "u1")
	if err != nil {
		t.Fatalf("ListAPIKeysByUser: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("ListAPIKeysByUser count = %d, want 1", len(list))
	}

	if err := s.UpdateAPIKeyLastUsed(ctx, "k1"); err != nil {
		t.Fatalf("UpdateAPIKeyLastUsed: %v", err)
	}
	after, _ := s.GetAPIKeyByID(ctx, "k1")
	if after.LastUsed == nil {
		t.Errorf("LastUsed should be set after UpdateAPIKeyLastUsed")
	}

	if err := s.DeleteAPIKey(ctx, "k1"); err != nil {
		t.Fatalf("DeleteAPIKey: %v", err)
	}
	// DeleteAPIKey is a soft delete (sets revoked=1); the row
	// stays and the call returns ErrNotFound only if the key was
	// never created.
	gotAfter, err := s.GetAPIKeyByID(ctx, "k1")
	if err != nil {
		t.Fatalf("GetAPIKeyByID after soft delete: %v", err)
	}
	if !gotAfter.Revoked {
		t.Errorf("after DeleteAPIKey, revoked = false, want true")
	}
}

func TestAppendAndVerifyAuditChain(t *testing.T) {
	t.Parallel()
	s := newTestSQLite(t)
	ctx := context.Background()

	entries := []*models.AuditEntry{
		{ID: "a1", UserID: "u1", Action: "create", Resource: "capability/c1", Details: map[string]any{"k": "v1"}},
		{ID: "a2", UserID: "u1", Action: "update", Resource: "capability/c1", Details: map[string]any{"k": "v2"}},
		{ID: "a3", UserID: "u2", Action: "delete", Resource: "capability/c1", Details: nil},
	}
	for _, e := range entries {
		if err := s.AppendAudit(ctx, e); err != nil {
			t.Fatalf("AppendAudit(%s): %v", e.ID, err)
		}
	}

	ok, reason, err := s.VerifyAuditChain(ctx)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if !ok {
		t.Errorf("audit chain verification failed: %s", reason)
	}

	listed, err := s.ListAudit(ctx, &models.AuditFilter{})
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	if len(listed) != 3 {
		t.Errorf("ListAudit count = %d, want 3", len(listed))
	}
	// ListAudit orders by timestamp DESC; entries were just appended
	// so timestamps may be equal. Order is not strictly asserted.
	seen := map[string]bool{}
	for _, e := range listed {
		seen[e.ID] = true
		if e.EntryHash == "" {
			t.Errorf("entry %s missing EntryHash", e.ID)
		}
	}
	for _, want := range []string{"a1", "a2", "a3"} {
		if !seen[want] {
			t.Errorf("missing entry %s in ListAudit", want)
		}
	}
}

func TestAuditChainDetectsTampering(t *testing.T) {
	t.Parallel()
	s := newTestSQLite(t)
	ctx := context.Background()

	for _, id := range []string{"a1", "a2", "a3"} {
		if err := s.AppendAudit(ctx, &models.AuditEntry{
			ID: id, UserID: "u1", Action: "noop", Resource: "x", Details: map[string]any{"i": id},
		}); err != nil {
			t.Fatalf("AppendAudit(%s): %v", id, err)
		}
	}

	// After consolidation the audit chain is append-only at the
	// database level (BEFORE UPDATE trigger in 002_audit_chain).
	// The tamper-detection path (UPDATE a row, re-verify) is no
	// longer reachable in production. We exercise the append-only
	// invariant here instead.
	_, err := s.DB().ExecContext(ctx,
		`UPDATE audit_entries SET action = 'tampered' WHERE id = 'a2'`)
	if err == nil {
		t.Fatal("expected audit_entries_no_update trigger to reject UPDATE")
	}
	if !strings.Contains(err.Error(), "append-only") {
		t.Errorf("expected 'append-only' in error, got %v", err)
	}

	// Verify the chain is still intact (UPDATE was rolled back).
	ok, _, err := s.VerifyAuditChain(ctx)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if !ok {
		t.Error("chain should still verify; the tamper UPDATE was rejected")
	}
}

func TestListAuditFilters(t *testing.T) {
	t.Parallel()
	s := newTestSQLite(t)
	ctx := context.Background()

	now := time.Now().UTC()
	for _, e := range []*models.AuditEntry{
		{ID: "a1", UserID: "u1", Action: "create", Resource: "capability/c1", Timestamp: now.Add(-2 * time.Hour)},
		{ID: "a2", UserID: "u2", Action: "delete", Resource: "capability/c2", Timestamp: now.Add(-time.Hour)},
		{ID: "a3", UserID: "u1", Action: "update", Resource: "capability/c1", Timestamp: now},
	} {
		if err := s.AppendAudit(ctx, e); err != nil {
			t.Fatalf("AppendAudit(%s): %v", e.ID, err)
		}
	}

	byUser, err := s.ListAudit(ctx, &models.AuditFilter{UserID: "u1"})
	if err != nil {
		t.Fatalf("ListAudit by user: %v", err)
	}
	if len(byUser) != 2 {
		t.Errorf("byUser count = %d, want 2", len(byUser))
	}

	byRes, err := s.ListAudit(ctx, &models.AuditFilter{Resource: "capability/c2"})
	if err != nil {
		t.Fatalf("ListAudit by resource: %v", err)
	}
	if len(byRes) != 1 || byRes[0].ID != "a2" {
		t.Errorf("byRes = %+v, want [a2]", byRes)
	}

	byAct, err := s.ListAudit(ctx, &models.AuditFilter{Action: "create"})
	if err != nil {
		t.Fatalf("ListAudit by action: %v", err)
	}
	if len(byAct) != 1 || byAct[0].ID != "a1" {
		t.Errorf("byAct = %+v, want [a1]", byAct)
	}
}

func TestCapabilityVersionLifecycle(t *testing.T) {
	t.Parallel()
	s := newTestSQLite(t)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := s.CreateWorkspace(ctx, &capability.Workspace{ID: "w1", Name: "Acme", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	if err := s.CreateProject(ctx, &capability.Project{ID: "p1", WorkspaceID: "w1", Name: "Greetings", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	c := &capability.Capability{
		ID:        "c1",
		ProjectID: "p1",
		Name:      "greeting",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateCapability(ctx, c); err != nil {
		t.Fatalf("CreateCapability: %v", err)
	}

	list, err := s.ListCapabilities(ctx, "p1")
	if err != nil {
		t.Fatalf("ListCapabilities: %v", err)
	}
	if len(list) != 1 || list[0].ID != "c1" {
		t.Errorf("ListCapabilities = %+v, want [c1]", list)
	}

	v := &capability.Version{
		ID:           "v1",
		CapabilityID: "c1",
		Version:      1,
		CreatedAt:    now,
		CreatedBy:    "u1",
	}
	if err := s.CreateVersion(ctx, v); err != nil {
		t.Fatalf("CreateVersion: %v", err)
	}

	got, err := s.GetVersion(ctx, "v1")
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if got.CapabilityID != "c1" || got.Version != 1 {
		t.Errorf("GetVersion mismatch: %+v", got)
	}

	versions, err := s.ListVersions(ctx, "c1")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 1 {
		t.Errorf("ListVersions count = %d, want 1", len(versions))
	}

	latest, err := s.GetLatestVersion(ctx, "c1")
	if err != nil {
		t.Fatalf("GetLatestVersion: %v", err)
	}
	if latest.ID != "v1" {
		t.Errorf("GetLatestVersion = %q, want v1", latest.ID)
	}
}

func TestProviderKeyRoundTrip(t *testing.T) {
	t.Parallel()
	s := newTestSQLite(t)
	ctx := context.Background()
	now := time.Now().UTC()
	pk := &models.ProviderKey{
		ID:           "pk1",
		ProviderName: "openai",
		KeyName:      "primary",
		EncryptedKey: "ciphertext",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.SaveProviderKey(ctx, pk); err != nil {
		t.Fatalf("SaveProviderKey: %v", err)
	}
	got, err := s.GetProviderKey(ctx, "pk1")
	if err != nil {
		t.Fatalf("GetProviderKey: %v", err)
	}
	if got.EncryptedKey != "ciphertext" {
		t.Errorf("EncryptedKey mismatch: %q", got.EncryptedKey)
	}

	byName, err := s.GetProviderKeyByName(ctx, "openai", "primary")
	if err != nil {
		t.Fatalf("GetProviderKeyByName: %v", err)
	}
	if byName.ID != "pk1" {
		t.Errorf("GetProviderKeyByName id = %q, want pk1", byName.ID)
	}

	if err := s.DeleteProviderKey(ctx, "pk1"); err != nil {
		t.Fatalf("DeleteProviderKey: %v", err)
	}
	if _, err := s.GetProviderKey(ctx, "pk1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetProviderKey after delete: %v, want ErrNotFound", err)
	}
}

func TestWebhookEndpointRoundTrip(t *testing.T) {
	t.Parallel()
	s := newTestSQLite(t)
	ctx := context.Background()
	now := time.Now().UTC()
	ep := &models.WebhookEndpointRecord{
		ID:        "w1",
		URL:       "https://example.com/hook",
		Events:    []string{"capability.created"},
		Active:    true,
		CreatedAt: now,
	}
	if err := s.SaveWebhookEndpoint(ctx, ep); err != nil {
		t.Fatalf("SaveWebhookEndpoint: %v", err)
	}
	got, err := s.GetWebhookEndpoint(ctx, "w1")
	if err != nil {
		t.Fatalf("GetWebhookEndpoint: %v", err)
	}
	if got.URL != ep.URL || !got.Active {
		t.Errorf("WebhookEndpoint mismatch: %+v", got)
	}

	list, err := s.ListWebhookEndpoints(ctx)
	if err != nil {
		t.Fatalf("ListWebhookEndpoints: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("ListWebhookEndpoints count = %d, want 1", len(list))
	}

	if err := s.DeleteWebhookEndpoint(ctx, "w1"); err != nil {
		t.Fatalf("DeleteWebhookEndpoint: %v", err)
	}
}

// TestWebhookSecretCiphertextOnDisk exercises SEC-7a: the
// plaintext `secret` column must not exist on disk after the
// consolidated 001_core_schema. SaveWebhookEndpoint writes
// SecretCiphertext and the column never had a plaintext sibling.
func TestWebhookSecretCiphertextOnDisk(t *testing.T) {
	t.Parallel()
	s := newTestSQLite(t)
	ctx := context.Background()
	now := time.Now().UTC()
	ep := &models.WebhookEndpointRecord{
		ID:               "w-sec",
		URL:              "https://example.com/hook",
		SecretCiphertext: []byte("ciphertext-blob"),
		Events:           []string{"capability.created"},
		Active:           true,
		CreatedAt:        now,
	}
	if err := s.SaveWebhookEndpoint(ctx, ep); err != nil {
		t.Fatalf("SaveWebhookEndpoint: %v", err)
	}
	// The plaintext `secret` column must not exist in the schema.
	var n int
	if err := s.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pragma_table_info('webhook_endpoints') WHERE name='secret'`,
	).Scan(&n); err != nil {
		t.Fatalf("pragma_table_info: %v", err)
	}
	if n != 0 {
		t.Errorf("plaintext secret column still exists; want absent")
	}
	var ct []byte
	if err := s.DB().QueryRowContext(ctx,
		`SELECT secret_ciphertext FROM webhook_endpoints WHERE id = 'w-sec'`,
	).Scan(&ct); err != nil {
		t.Fatalf("scan ciphertext: %v", err)
	}
	if string(ct) != "ciphertext-blob" {
		t.Errorf("ciphertext mismatch: got %q", ct)
	}
}

func TestScheduleCRUD(t *testing.T) {
	t.Parallel()
	s := newTestSQLite(t)
	ctx := context.Background()
	now := time.Now().UTC()
	// Seed the parent rows that schedules FKs reference. The
	// fixture used to rely on the absence of FKs; the post-043
	// schema enforces the references.
	seedScheduleFixture(t, s)
	due := &schedule.Schedule{
		ID:          "sc-due",
		WorkspaceID: "w1",
		ReleaseID:   "r1",
		Kind:        schedule.KindCron,
		Cron:        "*/5 * * * *",
		NextFireAt:  now.Add(-time.Minute),
		Enabled:     true,
		CreatedAt:   now,
		CreatedBy:   "u1",
	}
	future := &schedule.Schedule{
		ID:          "sc-future",
		WorkspaceID: "w1",
		ReleaseID:   "r1",
		Kind:        schedule.KindCron,
		Cron:        "0 0 * * *",
		NextFireAt:  now.Add(time.Hour),
		Enabled:     true,
		CreatedAt:   now,
		CreatedBy:   "u1",
	}
	for _, sc := range []*schedule.Schedule{due, future} {
		if err := s.CreateSchedule(ctx, sc); err != nil {
			t.Fatalf("CreateSchedule(%s): %v", sc.ID, err)
		}
	}

	// Verify the due schedule is reported and the future one isn't.
	got, err := s.ListDueSchedules(ctx, now.Add(time.Second), 100)
	if err != nil {
		t.Fatalf("ListDueSchedules: %v", err)
	}
	if len(got) != 1 || got[0].ID != "sc-due" {
		t.Errorf("expected only the due schedule, got %d rows", len(got))
	}

	dueNow, err := s.ListDueSchedules(ctx, now, 10)
	if err != nil {
		t.Fatalf("ListDueSchedules: %v", err)
	}
	if len(dueNow) != 1 || dueNow[0].ID != "sc-due" {
		t.Errorf("ListDueSchedules = %+v, want [sc-due]", dueNow)
	}

	due.FiredCount = 1
	due.NextFireAt = now.Add(5 * time.Minute)
	if err := s.UpdateSchedule(ctx, due); err != nil {
		t.Fatalf("UpdateSchedule: %v", err)
	}

	if err := s.UpdateSchedule(ctx, future); err != nil {
		t.Fatalf("UpdateSchedule: %v", err)
	}
}

// TestAuditChainDetectsTailDeletion exercises DB-7 / SEC-CHAIN-1:
// VerifyAuditChain must cross-check the walked rowid against
// audit_chain_state and report "chain tail mismatch" when an
// operator deletes the last N rows out from under the chain.
func TestAuditChainDetectsTailDeletion(t *testing.T) {
	t.Parallel()
	s := newTestSQLite(t)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("td-%02d", i)
		if err := s.AppendAudit(ctx, &models.AuditEntry{
			ID: id, UserID: "u1", Action: "noop", Resource: "x", Details: map[string]any{"i": i},
		}); err != nil {
			t.Fatalf("AppendAudit(%s): %v", id, err)
		}
	}

	// Sanity: chain is intact before any tampering.
	ok, reason, err := s.VerifyAuditChain(ctx)
	if err != nil {
		t.Fatalf("VerifyAuditChain (clean): %v", err)
	}
	if !ok {
		t.Fatalf("clean chain failed verification: %s", reason)
	}

	// Delete the last 5 audit rows directly, bypassing AppendAudit
	// so audit_chain_state is NOT updated. The verifier must catch
	// the rowid mismatch.
	if _, err := s.DB().ExecContext(ctx,
		`DELETE FROM audit_entries WHERE id IN ('td-05','td-06','td-07','td-08','td-09')`,
	); err != nil {
		t.Fatalf("delete tail: %v", err)
	}

	ok, reason, err = s.VerifyAuditChain(ctx)
	if err != nil {
		t.Fatalf("VerifyAuditChain (tampered): %v", err)
	}
	if ok {
		t.Fatalf("expected verifier to flag tail deletion; reason=%q", reason)
	}
	if !strings.Contains(reason, "tail mismatch") {
		t.Errorf("expected reason to mention tail mismatch; got %q", reason)
	}
}

// TestListAuditOffsetOnly exercises DB-9b: with Limit=0 and
// Offset>0 the repo must emit LIMIT -1 OFFSET ? and return
// every remaining row. Before DB-9a this produced a SQL
// syntax error.
func TestListAuditOffsetOnly(t *testing.T) {
	t.Parallel()
	s := newTestSQLite(t)
	ctx := context.Background()
	for i := 0; i < 6; i++ {
		id := fmt.Sprintf("off-%02d", i)
		if err := s.AppendAudit(ctx, &models.AuditEntry{
			ID: id, UserID: "u1", Action: "noop", Resource: "x", Details: map[string]any{"i": i},
		}); err != nil {
			t.Fatalf("AppendAudit(%s): %v", id, err)
		}
	}
	out, err := s.ListAudit(ctx, &models.AuditFilter{Offset: 2})
	if err != nil {
		t.Fatalf("ListAudit offset-only: %v", err)
	}
	if len(out) != 4 {
		t.Errorf("expected 4 rows past offset 2, got %d", len(out))
	}
}

// TestListExecutionsOffsetOnly exercises DB-9b for the
// executions path. With Limit=0 and Offset>0 the query must
// skip the first N rows and return the tail.
func TestListExecutionsOffsetOnly(t *testing.T) {
	t.Parallel()
	s := newTestSQLite(t)
	ctx := context.Background()
	now := time.Now()
	if err := s.CreateWorkspace(ctx, &capability.Workspace{ID: "w1", Name: "Acme", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if err := s.CreateProject(ctx, &capability.Project{ID: "p1", WorkspaceID: "w1", Name: "Greetings", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if err := s.CreateCapability(ctx, &capability.Capability{
		ID: "c1", ProjectID: "p1", Name: "c", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed capability: %v", err)
	}
	if err := s.CreateVersion(ctx, &capability.Version{
		ID: "v1", CapabilityID: "c1", Version: 1,
		ManifestHash: "h1", CreatedAt: now, CreatedBy: "u1",
	}); err != nil {
		t.Fatalf("seed version: %v", err)
	}
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("exec-%02d", i)
		if err := s.CreateExecution(ctx, &capability.Execution{
			ID: id, CapabilityVersionID: "v1", Error: "",
		}); err != nil {
			t.Fatalf("CreateExecution(%s): %v", id, err)
		}
	}
	out, err := s.ListExecutions(ctx, capability.ExecutionFilter{Offset: 2})
	if err != nil {
		t.Fatalf("ListExecutions offset-only: %v", err)
	}
	if len(out) != 3 {
		t.Errorf("expected 3 rows past offset 2, got %d", len(out))
	}
}

// TestBootstrapAdminConcurrent exercises SEC-5a: 100 goroutines
// all hit BootstrapAdmin at the same time with the same email.
// Exactly one must succeed; the rest must see ErrConflict.
func TestBootstrapAdminConcurrent(t *testing.T) {
	t.Parallel()
	s := newTestSQLite(t)
	ctx := context.Background()

	const N = 100
	results := make(chan error, N)
	start := make(chan struct{})
	for i := 0; i < N; i++ {
		go func(i int) {
			u := &models.User{
				ID: fmt.Sprintf("u-%03d", i), Email: "admin@local",
				Name: "admin", Role: "admin", CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}
			k := &models.APIKey{
				ID: fmt.Sprintf("k-%03d", i), UserID: u.ID,
				Name: "bootstrap", KeyHash: fmt.Sprintf("hash-%03d", i),
				KeyPrefix: "pk", Role: "admin", CreatedAt: time.Now(),
			}
			<-start
			results <- s.BootstrapAdmin(ctx, u, k)
		}(i)
	}
	close(start)

	wins, conflicts, other := 0, 0, 0
	for i := 0; i < N; i++ {
		err := <-results
		switch {
		case err == nil:
			wins++
		case errors.Is(err, store.ErrConflict):
			conflicts++
		default:
			other++
			t.Errorf("unexpected error: %v", err)
		}
	}
	if wins != 1 {
		t.Errorf("expected exactly 1 winner, got %d (conflicts=%d, other=%d)", wins, conflicts, other)
	}
	if conflicts+other != N-1 {
		t.Errorf("expected %d non-winners, got conflicts=%d other=%d", N-1, conflicts, other)
	}
}
