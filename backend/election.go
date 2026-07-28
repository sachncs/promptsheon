package backend

// Flattened from backend/election/election.go.

// Package election provides a SQLite-backed leader election
// primitive. It lets a multi-replica StatefulSet run several
// Promptsheon daemons against a single database file while
// guaranteeing that only one replica is the writer at any given
// moment. Non-leader replicas stay online for reads but refuse
// mutations.
//
// Implementation: an advisory lock row in a dedicated
// `leader` table. The leader renews its lock every TTL/2. On
// restart the candidate tries to acquire the row; if no live
// holder exists it becomes the leader. If the existing holder
// has not renewed within TTL, the candidate wins.
//
// The lock is best-effort and process-local: a misbehaving
// replica that holds the row but stops renewing will be
// detected within TTL. The election is NOT a quorum protocol;
// it is a single-master advisory lock suitable for SQLite.
import (
	"github.com/sachncs/promptsheon/backend/errs"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

)

// Leader carries the identity of the current leader plus the
// expiry of its lease.
type Leader struct {
	Identity  string
	ExpiresAt time.Time
	IsLeader  bool
}

// Elector manages a single SQL advisory lock on the
// `leader` table. Construct via New and call Run in a
// goroutine.
type Elector struct {
	db        *sql.DB
	identity  string
	ttl       time.Duration
	now       func() time.Time
	isLeader  bool
	lastRenew time.Time
	term      int64
	mu        sync.Mutex
}

// New returns an Elector. The supplied identity should be unique
// across replicas (typically the pod name).
func New(db *sql.DB, identity string, ttl time.Duration) *Elector {
	if ttl < time.Second {
		ttl = 30 * time.Second
	}
	return &Elector{
		db:       db,
		identity: identity,
		ttl:      ttl,
		now:      time.Now,
	}
}

// EnsureTable creates the leader table on first call. Safe to
// invoke repeatedly; the SQL is idempotent. The leader row also
// carries a monotonic term counter; every successful Acquire
// increments it so downstream writers can use the term as a
// fencing token.
func (e *Elector) EnsureTable(ctx context.Context) error {
	_, err := e.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS leader (
			name        TEXT PRIMARY KEY,
			identity    TEXT NOT NULL,
			expires_at  DATETIME NOT NULL,
			term        INTEGER NOT NULL DEFAULT 0
		)`)
	if err != nil {
		return fmt.Errorf("election: create table: %w", err)
	}
	return nil
}

// errs.ErrorElectionNotLeader is returned by TryAcquire when this replica did
// not win the election. Callers should treat the local
// replica as a read-only follower until Acquire succeeds.

// Acquire tries to claim the leader row. Returns nil when this
// replica becomes (or already was) the leader; errs.ErrorElectionNotLeader
// when another replica currently holds the lease.
//
// The transaction is upgraded to a SQLite BEGIN IMMEDIATE so the
// read and the conditional write land atomically; otherwise the
// default BEGIN DEFERRED would let the SELECT succeed without
// holding the write lock and the subsequent INSERT/UPDATE could
// fail with SQLITE_BUSY_SNAPSHOT under contention.
//
// A replica that already holds the lease and successfully
// renews it returns nil again without going through the
// SQL contention path.
func (e *Elector) Acquire(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, err := e.db.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("election: begin immediate: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = e.db.ExecContext(context.Background(), "ROLLBACK")
		}
	}()

	var (
		curIdentity string
		expiresAt   time.Time
		curTerm     int64
	)
	row := e.db.QueryRowContext(ctx,
		`SELECT identity, expires_at, term FROM leader WHERE name = 'promptsheon'`)
	switch err := row.Scan(&curIdentity, &expiresAt, &curTerm); {
	case errors.Is(err, sql.ErrNoRows):
		// No leader yet — try to insert.
		expiresAt = e.now().Add(e.ttl)
		if _, err := e.db.ExecContext(ctx,
			`INSERT INTO leader (name, identity, expires_at, term) VALUES ('promptsheon', ?, ?, 1)`,
			e.identity, expiresAt); err != nil {
			return fmt.Errorf("election: insert: %w", err)
		}
		e.term = 1
	case err != nil:
		return fmt.Errorf("election: scan: %w", err)
	default:
		if curIdentity == e.identity {
			// Renew.
			expiresAt = e.now().Add(e.ttl)
			if _, err := e.db.ExecContext(ctx,
				`UPDATE leader SET expires_at = ? WHERE name = 'promptsheon' AND identity = ? AND term = ?`,
				expiresAt, e.identity, curTerm); err != nil {
				return fmt.Errorf("election: renew: %w", err)
			}
			e.term = curTerm
		} else if e.now().Before(expiresAt) {
			// Held by someone else and the lease is still valid.
			return errs.ErrorElectionNotLeader
		} else {
			// Stale lease — steal it but only if the lease is
			// actually expired. The expires_at <= now guard
			// defeats the TOCTOU window where the previous leader
			// renewed between our SELECT and our UPDATE.
			expiresAt = e.now().Add(e.ttl)
			res, err := e.db.ExecContext(ctx,
				`UPDATE leader SET identity = ?, expires_at = ?, term = term + 1
				 WHERE name = 'promptsheon' AND expires_at <= ?`,
				e.identity, expiresAt, e.now())
			if err != nil {
				return fmt.Errorf("election: steal: %w", err)
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				// Lost the race: the prior leader renewed
				// between our SELECT and our UPDATE. Stay as
				// follower.
				return errs.ErrorElectionNotLeader
			}
			e.term = curTerm + 1
		}
	}

	if _, err := e.db.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("election: commit: %w", err)
	}
	committed = true
	e.isLeader = true
	e.lastRenew = e.now()
	return nil
}

// Release voluntarily steps down. Use when shutting down so the
// next replica can pick up the lease immediately.
func (e *Elector) Release(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.isLeader {
		return nil
	}
	if _, err := e.db.ExecContext(ctx,
		`DELETE FROM leader WHERE name = 'promptsheon' AND identity = ?`,
		e.identity); err != nil {
		return fmt.Errorf("election: release: %w", err)
	}
	e.isLeader = false
	return nil
}

// IsLeader reports the cached leadership state. Cheap; safe to
// poll on every request.
func (e *Elector) IsLeader() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.isLeader
}

// Current returns the current leader identity and lease expiry.
// Useful for /ready and /metrics.
func (e *Elector) Current(ctx context.Context) (Leader, error) {
	var l Leader
	var expiresAt time.Time
	var identity string
	row := e.db.QueryRowContext(ctx,
		`SELECT identity, expires_at FROM leader WHERE name = 'promptsheon'`)
	switch err := row.Scan(&identity, &expiresAt); {
	case errors.Is(err, sql.ErrNoRows):
		return Leader{}, nil
	case err != nil:
		return Leader{}, fmt.Errorf("election: query: %w", err)
	}
	l.Identity = identity
	l.ExpiresAt = expiresAt
	l.IsLeader = e.IsLeader()
	return l, nil
}

// Run blocks until ctx is cancelled, renewing the lease every
// TTL/2 and stepping down on cancel. Errors are logged via the
// returned channel and do not stop the loop; a renewal failure
// causes the replica to lose leadership after TTL expires.
func (e *Elector) Run(ctx context.Context, errCh chan<- error) {
	ticker := time.NewTicker(e.ttl / 2)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = e.Release(context.Background())
			return
		case <-ticker.C:
			if err := e.Acquire(ctx); err != nil {
				if !errors.Is(err, errs.ErrorElectionNotLeader) {
					select {
					case errCh <- err:
					default:
					}
				}
				e.mu.Lock()
				e.isLeader = false
				e.mu.Unlock()
			}
		}
	}
}
