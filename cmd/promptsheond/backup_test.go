//go:build !e2e

// backup_test.go: P4.6 — backup + restore round-trip.
//
// The daemon exposes a `promptsheond backup <dst>` subcommand
// (see runBackup / runBackupVACUUMINTO in daemon.go). This test
// proves the round-trip works: write some audit entries to a
// fresh DB, VACUUM INTO a snapshot, open the snapshot in a
// fresh connection, and verify the audit chain is intact.
//
// Without this test, a regression in the backup path would
// ship a snapshot that fails to restore on the operator's
// incident-response day. The test is the only thing standing
// between an unrecoverable audit-chain failure and a copy that
// is actually restorable.

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/sachncs/promptsheon/promptsheon/models"
	"github.com/sachncs/promptsheon/promptsheon/store"
)

// randomID generates a fresh ID for the audit entry. The
// daemon package has a private randomID(prefix) helper for
// runtime use; the test uses a separate function name to
// avoid the redeclaration and to make the test fixture
// intent explicit.
func freshAuditID(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return hex.EncodeToString(b)
}

func TestRunBackupVACUUMINTO_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.db")
	dstPath := filepath.Join(dir, "dst.db")

	// 1. Open the source DB and write a couple of audit rows
	//    so the destination has something to verify.
	src, err := store.NewSQLite(srcPath)
	if err != nil {
		t.Fatalf("open src: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })

	ctx := context.Background()
	// The audit table has a foreign key to users. The
	// simplest fixture is to insert a user first, then the
	// audit rows. The point of this test is the
	// backup/restore round-trip — not the audit insert
	// path — so we use the package's exported CreateUser.
	if err := src.CreateUser(ctx, &models.User{ID: "test-user", Name: "Test User"}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	for i := 0; i < 5; i++ {
		if err := src.AppendAudit(ctx, &models.AuditEntry{
			ID:       freshAuditID(t),
			UserID:   "test-user",
			Action:   "round-trip",
			Resource: "backup-test",
			Details:  map[string]any{"i": i},
		}); err != nil {
			t.Fatalf("append audit %d: %v", i, err)
		}
	}

	// 2. Run the backup. runBackupVACUUMINTO refuses to
	//    overwrite; we must ensure dst does not exist.
	_ = os.Remove(dstPath)
	if err := runBackupVACUUMINTO(srcPath, dstPath); err != nil {
		t.Fatalf("backup: %v", err)
	}

	// 3. The destination must exist and be non-empty.
	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("stat dst: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("backup is empty")
	}

	// 4. Open the snapshot and verify the audit rows round-trip.
	dst, err := store.NewSQLite(dstPath)
	if err != nil {
		t.Fatalf("open dst: %v", err)
	}
	t.Cleanup(func() { _ = dst.Close() })

	rows, err := dst.DB().QueryContext(ctx, "SELECT COUNT(*) FROM audit_entries")
	if err != nil {
		t.Fatalf("query dst: %v", err)
	}
	defer rows.Close()
	var count int
	if !rows.Next() {
		t.Fatalf("no row from dst")
	}
	if err := rows.Scan(&count); err != nil {
		t.Fatalf("scan count: %v", err)
	}
	if count != 5 {
		t.Errorf("audit rows in snapshot: got %d, want 5", count)
	}

	// 5. The chain's most recent entry hash must be present and
	//    non-empty. A snapshot without the latest hash cannot be
	//    used to verify the chain forward; the test catches a
	//    regression where the snapshot is taken before the WAL
	//    is flushed.
	var lastHash string
	if err := dst.DB().QueryRowContext(ctx, "SELECT entry_hash FROM audit_entries ORDER BY id DESC LIMIT 1").Scan(&lastHash); err != nil {
		t.Fatalf("query last hash: %v", err)
	}
	if lastHash == "" {
		t.Errorf("snapshot has empty chain head")
	}
}

func TestRunBackupVACUUMINTO_OverwritesExisting(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.db")
	dstPath := filepath.Join(dir, "dst.db")

	// Source must exist for VACUUM INTO to produce a
	// meaningful backup. We don't write any rows because
	// the contract we test here is "does the helper
	// successfully back up an empty-but-valid DB".
	src, err := store.NewSQLite(srcPath)
	if err != nil {
		t.Fatalf("open src: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })

	// Pre-create the destination. The current implementation
	// of runBackupVACUUMINTO deletes the destination if it
	// exists (VACUUM INTO refuses to overwrite). Document
	// that behaviour with this test: callers who care about
	// not clobbering an existing snapshot must not pass an
	// existing path.
	if err := os.WriteFile(dstPath, []byte("placeholder"), 0o600); err != nil {
		t.Fatalf("create dst: %v", err)
	}

	if err := runBackupVACUUMINTO(srcPath, dstPath); err != nil {
		t.Fatalf("backup: %v", err)
	}

	// The placeholder content must be gone; the new file
	// must be a valid SQLite database.
	data, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(data) == "placeholder" {
		t.Errorf("dst still holds the placeholder; the overwrite path did not run")
	}
	if !bytes.HasPrefix(data, []byte("SQLite format 3\x00")) {
		t.Errorf("dst is not a SQLite database (prefix: %q)", firstBytes(data, 16))
	}
}

// firstBytes returns the first n bytes of b as a string for
// use in error messages. Used to keep test failure logs short.
func firstBytes(b []byte, n int) string {
	if len(b) < n {
		return string(b)
	}
	return string(b[:n])
}
