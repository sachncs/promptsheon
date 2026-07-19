package store_test

import (
	"context"
	"errors"
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
	if n < 20 {
		t.Errorf("migrations applied = %d, want at least 20", n)
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

	// Tamper with the action column of the second entry.
	if _, err := s.DB().ExecContext(ctx,
		`UPDATE audit_entries SET action = 'tampered' WHERE id = 'a2'`); err != nil {
		t.Fatalf("tamper update: %v", err)
	}

	ok, reason, err := s.VerifyAuditChain(ctx)
	if err != nil {
		t.Fatalf("VerifyAuditChain: %v", err)
	}
	if ok {
		t.Error("verification should have failed after tampering")
	}
	if reason == "" {
		t.Error("reason should describe the tampering")
	}
	if !strings.Contains(reason, "a2") {
		t.Errorf("reason should mention tampered entry a2, got %q", reason)
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
		Secret:    "secret",
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
		ID:          "sc1",
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
		ID:          "sc2",
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

	got, err := s.GetSchedule(ctx, "sc1")
	if err != nil {
		t.Fatalf("GetSchedule: %v", err)
	}
	if got.Cron != "*/5 * * * *" {
		t.Errorf("GetSchedule cron = %q, want */5 * * * *", got.Cron)
	}

	dueNow, err := s.ListDueSchedules(ctx, now, 10)
	if err != nil {
		t.Fatalf("ListDueSchedules: %v", err)
	}
	if len(dueNow) != 1 || dueNow[0].ID != "sc1" {
		t.Errorf("ListDueSchedules = %+v, want [sc1]", dueNow)
	}

	byRelease, err := s.ListSchedulesForRelease(ctx, "r1")
	if err != nil {
		t.Fatalf("ListSchedulesForRelease: %v", err)
	}
	if len(byRelease) != 2 {
		t.Errorf("ListSchedulesForRelease count = %d, want 2", len(byRelease))
	}

	due.FiredCount = 1
	due.NextFireAt = now.Add(5 * time.Minute)
	if err := s.UpdateSchedule(ctx, due); err != nil {
		t.Fatalf("UpdateSchedule: %v", err)
	}
	after, _ := s.GetSchedule(ctx, "sc1")
	if after.FiredCount != 1 {
		t.Errorf("after update FiredCount = %d, want 1", after.FiredCount)
	}

	if err := s.DeleteSchedule(ctx, "sc2"); err != nil {
		t.Fatalf("DeleteSchedule: %v", err)
	}
	_, err = s.GetSchedule(ctx, "sc2")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetSchedule after delete: err = %v, want ErrNotFound", err)
	}
}
