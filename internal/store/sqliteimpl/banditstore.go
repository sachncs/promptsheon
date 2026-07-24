// Package sqliteimpl — BanditStore is the SQLite-backed
// banditstore.Backend. Each (arm, replica) pair is a row;
// concurrent merges are conflict-safe via the MAX() update
// policy on the (successes, failures) pair. The grow-only
// CRDT semantics live in internal/bandit/crdt.go; this file
// only translates them into SQLite.
package sqliteimpl

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/sachncs/promptsheon/internal/bandit"
)

// BanditStore implements banditstore.Backend against SQLite.
// The (arm_id, replica_id) primary key is the natural
// per-replica partition; merging is a MAX() upsert so two
// concurrent replicas can never regress each other's
// counters. The effective posterior (alpha = 1+successes,
// beta = 1+failures) is reconstructed on read.
type BanditStore struct{ db *sql.DB }

// NewBanditStore wraps db.
func NewBanditStore(db *sql.DB) *BanditStore { return &BanditStore{db: db} }

// Load returns the per-arm SUM across every replica. Each
// replica's (successes, failures) contributes one tally to
// the bucket for arm_id; the bucket is the sum of every
// contributing replica's observations. The per-(arm,
// replica) Merge still uses MAX() so duplicate snapshots
// from a single replica are no-ops.
func (s *BanditStore) Load(ctx context.Context) (bandit.State, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT arm_id, SUM(successes), SUM(failures) FROM bandit_arm_counters GROUP BY arm_id`,
	)
	if err != nil {
		return nil, fmt.Errorf("banditstore: query: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := bandit.State{}
	for rows.Next() {
		var armID string
		var successes, failures int64
		if err := rows.Scan(&armID, &successes, &failures); err != nil {
			return nil, fmt.Errorf("banditstore: scan: %w", err)
		}
		out[armID] = bandit.Counter{
			Successes: uint64(successes),
			Failures:  uint64(failures),
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// Observe bumps one (replica, arm) counter. The MAX() update
// policy keeps the operation conflict-safe: a concurrent
// observer that already saw a higher count will be retained.
func (s *BanditStore) Observe(ctx context.Context, replicaID, armID string, success bool) error {
	if replicaID == "" || armID == "" {
		return errors.New("banditstore: replicaID and armID are required")
	}
	// Apply the per-(arm, replica) bump. We carry the +1 through
	// the MAX() expression so an existing higher count wins.
	if success {
		if _, err := s.db.ExecContext(ctx,
			`INSERT INTO bandit_arm_counters (arm_id, replica_id, successes, failures, updated_at)
			 VALUES (?, ?, 1, 0, CURRENT_TIMESTAMP)
			 ON CONFLICT(arm_id, replica_id) DO UPDATE SET
			   successes = MAX(excluded.successes, bandit_arm_counters.successes + 1),
			   updated_at = excluded.updated_at`,
			armID, replicaID,
		); err != nil {
			return fmt.Errorf("banditstore: observe success: %w", err)
		}
		return nil
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO bandit_arm_counters (arm_id, replica_id, successes, failures, updated_at)
		 VALUES (?, ?, 0, 1, CURRENT_TIMESTAMP)
		 ON CONFLICT(arm_id, replica_id) DO UPDATE SET
		   failures = MAX(excluded.failures, bandit_arm_counters.failures + 1),
		   updated_at = excluded.updated_at`,
		armID, replicaID,
	); err != nil {
		return fmt.Errorf("banditstore: observe failure: %w", err)
	}
	return nil
}

// Merge folds a remote State into the local store. The merge
// is conflict-safe because each per-arm counter is the MAX()
// of the local row and the incoming value.
func (s *BanditStore) Merge(ctx context.Context, replicaID string, remote bandit.State) error {
	if replicaID == "" {
		return errors.New("banditstore: replicaID required for merge")
	}
	if len(remote) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("banditstore: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO bandit_arm_counters (arm_id, replica_id, successes, failures, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(arm_id, replica_id) DO UPDATE SET
		   successes = MAX(excluded.successes, bandit_arm_counters.successes),
		   failures  = MAX(excluded.failures,  bandit_arm_counters.failures),
		   updated_at = excluded.updated_at`,
	)
	if err != nil {
		return fmt.Errorf("banditstore: prepare: %w", err)
	}
	defer func() { _ = stmt.Close() }()
	for arm, c := range remote {
		if _, err := stmt.ExecContext(ctx, arm, replicaID, c.Successes, c.Failures); err != nil {
			return fmt.Errorf("banditstore: merge %q: %w", arm, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("banditstore: commit: %w", err)
	}
	return nil
}
