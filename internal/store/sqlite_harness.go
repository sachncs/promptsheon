package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sachncs/promptsheon/internal/harness"
)

// ---------------------------------------------------------------------------
// Datasets
// ---------------------------------------------------------------------------

func (s *SQLite) CreateDataset(ctx context.Context, d *harness.Dataset) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO datasets (id, capability_id, name, description, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		d.ID, d.CapabilityID, d.Name, d.Description, d.CreatedAt, d.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert dataset: %w", err)
	}
	return nil
}

func (s *SQLite) GetDataset(ctx context.Context, id string) (*harness.Dataset, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, capability_id, name, description, created_at, updated_at
		 FROM datasets WHERE id = ?`, id,
	)
	return scanDataset(row)
}

func (s *SQLite) ListDatasetsForCapability(ctx context.Context, capabilityID string) ([]*harness.Dataset, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, capability_id, name, description, created_at, updated_at
		 FROM datasets WHERE capability_id = ? ORDER BY created_at DESC`, capabilityID,
	)
	if err != nil {
		return nil, fmt.Errorf("list datasets: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []*harness.Dataset
	for rows.Next() {
		d, err := scanDataset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *SQLite) DeleteDataset(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM datasets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete dataset: %w", err)
	}
	return nil
}

func scanDataset(scan interface {
	Scan(dest ...any) error
}) (*harness.Dataset, error) {
	var d harness.Dataset
	err := scan.Scan(&d.ID, &d.CapabilityID, &d.Name, &d.Description, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan dataset: %w", err)
	}
	return &d, nil
}

// ---------------------------------------------------------------------------
// DatasetCases
// ---------------------------------------------------------------------------

func (s *SQLite) UpsertDatasetCases(ctx context.Context, datasetID string, cases []harness.DatasetCase) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM dataset_cases WHERE dataset_id = ?`, datasetID); err != nil {
		return fmt.Errorf("delete cases: %w", err)
	}
	for _, c := range cases {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO dataset_cases (id, dataset_id, seq, inputs, expected, description)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			c.ID, datasetID, c.Seq, string(c.Inputs), string(c.Expected), c.Description,
		); err != nil {
			return fmt.Errorf("insert case: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE datasets SET updated_at = ? WHERE id = ?`,
		time.Now().UTC(), datasetID,
	); err != nil {
		return fmt.Errorf("touch dataset: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (s *SQLite) ListDatasetCases(ctx context.Context, datasetID string) ([]harness.DatasetCase, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, dataset_id, seq, inputs, expected, description
		 FROM dataset_cases WHERE dataset_id = ? ORDER BY seq`, datasetID,
	)
	if err != nil {
		return nil, fmt.Errorf("list cases: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []harness.DatasetCase
	for rows.Next() {
		var c harness.DatasetCase
		var inputs, expected, desc string
		if err := rows.Scan(&c.ID, &c.DatasetID, &c.Seq, &inputs, &expected, &desc); err != nil {
			return nil, err
		}
		c.Inputs = []byte(inputs)
		c.Expected = []byte(expected)
		c.Description = desc
		out = append(out, c)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Preconditions
// ---------------------------------------------------------------------------

func (s *SQLite) CreatePrecondition(ctx context.Context, p *harness.Precondition) error {
	enabledInt := 0
	if p.Enabled {
		enabledInt = 1
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO preconditions (id, capability_id, name, command, timeout_sec, enabled, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.CapabilityID, p.Name, p.Command, p.TimeoutSec, enabledInt, p.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert precondition: %w", err)
	}
	return nil
}

// GetPrecondition fetches a single precondition by id. Returns
// ErrNotFound when the row does not exist so the API handler can
// translate it into a 404 via translateDBError.
func (s *SQLite) GetPrecondition(ctx context.Context, id string) (*harness.Precondition, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, capability_id, name, command, timeout_sec, enabled, created_at
		 FROM preconditions WHERE id = ?`, id,
	)
	return scanPrecondition(row)
}

// UpdatePrecondition replaces the mutable fields of an existing
// precondition. The (id, capability_id, created_at) tuple is
// immutable: callers cannot move a precondition to a different
// capability or rewrite its creation time. Returns ErrNotFound
// when no row matched the id so the API can return 404.
func (s *SQLite) UpdatePrecondition(ctx context.Context, p *harness.Precondition) error {
	enabledInt := 0
	if p.Enabled {
		enabledInt = 1
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE preconditions
		 SET name = ?, command = ?, timeout_sec = ?, enabled = ?
		 WHERE id = ?`,
		p.Name, p.Command, p.TimeoutSec, enabledInt, p.ID,
	)
	if err != nil {
		return fmt.Errorf("update precondition: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update precondition rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLite) ListPreconditionsForCapability(ctx context.Context, capabilityID string) ([]*harness.Precondition, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, capability_id, name, command, timeout_sec, enabled, created_at
		 FROM preconditions WHERE capability_id = ? ORDER BY created_at`, capabilityID,
	)
	if err != nil {
		return nil, fmt.Errorf("list preconditions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []*harness.Precondition
	for rows.Next() {
		p, err := scanPrecondition(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *SQLite) DeletePrecondition(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM preconditions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete precondition: %w", err)
	}
	return nil
}

func scanPrecondition(scan interface {
	Scan(dest ...any) error
}) (*harness.Precondition, error) {
	var p harness.Precondition
	var enabled int
	err := scan.Scan(&p.ID, &p.CapabilityID, &p.Name, &p.Command, &p.TimeoutSec, &enabled, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan precondition: %w", err)
	}
	p.Enabled = enabled != 0
	return &p, nil
}

// ---------------------------------------------------------------------------
// EvalRuns
// ---------------------------------------------------------------------------

func (s *SQLite) CreateEvalRun(ctx context.Context, r *harness.EvalRun) error {
	var finished sql.NullTime
	if r.FinishedAt != nil {
		finished = sql.NullTime{Time: *r.FinishedAt, Valid: true}
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO eval_runs
		 (id, release_id, dataset_id, scorer, score, passed, failed, total, status, started_at, finished_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.ReleaseID, r.DatasetID, string(r.Scorer),
		r.Score, r.Passed, r.Failed, r.Total, string(r.Status),
		r.StartedAt, finished,
	)
	if err != nil {
		return fmt.Errorf("insert eval run: %w", err)
	}
	return nil
}

func (s *SQLite) UpdateEvalRun(ctx context.Context, r *harness.EvalRun) error {
	var finished sql.NullTime
	if r.FinishedAt != nil {
		finished = sql.NullTime{Time: *r.FinishedAt, Valid: true}
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE eval_runs SET
			score = ?, passed = ?, failed = ?, total = ?, status = ?, finished_at = ?
		 WHERE id = ?`,
		r.Score, r.Passed, r.Failed, r.Total, string(r.Status), finished, r.ID,
	)
	if err != nil {
		return fmt.Errorf("update eval run: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLite) GetEvalRun(ctx context.Context, id string) (*harness.EvalRun, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, release_id, dataset_id, scorer, score, passed, failed, total, status, started_at, finished_at
		 FROM eval_runs WHERE id = ?`, id,
	)
	return scanEvalRun(row)
}

func (s *SQLite) ListEvalRunsForRelease(ctx context.Context, releaseID string) ([]*harness.EvalRun, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, release_id, dataset_id, scorer, score, passed, failed, total, status, started_at, finished_at
		 FROM eval_runs WHERE release_id = ? ORDER BY started_at DESC`, releaseID,
	)
	if err != nil {
		return nil, fmt.Errorf("list eval runs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []*harness.EvalRun
	for rows.Next() {
		r, err := scanEvalRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func scanEvalRun(scan interface {
	Scan(dest ...any) error
}) (*harness.EvalRun, error) {
	var r harness.EvalRun
	var scorer, status string
	var finished sql.NullTime
	err := scan.Scan(
		&r.ID, &r.ReleaseID, &r.DatasetID, &scorer, &r.Score,
		&r.Passed, &r.Failed, &r.Total, &status,
		&r.StartedAt, &finished,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan eval run: %w", err)
	}
	r.Scorer = harness.Scorer(scorer)
	r.Status = harness.RunStatus(status)
	if finished.Valid {
		t := finished.Time
		r.FinishedAt = &t
	}
	return &r, nil
}

// ---------------------------------------------------------------------------
// EvalResults
// ---------------------------------------------------------------------------

func (s *SQLite) CreateEvalResults(ctx context.Context, results []harness.EvalResult) error {
	if len(results) == 0 {
		return nil
	}
	var b strings.Builder
	args := make([]any, 0, len(results)*7)
	b.WriteString(`INSERT INTO eval_results (id, run_id, case_id, seq, passed, actual, error, latency_ms) VALUES `)
	for i, r := range results {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("(?, ?, ?, ?, ?, ?, ?, ?)")
		passedInt := 0
		if r.Passed {
			passedInt = 1
		}
		args = append(args, r.ID, r.RunID, r.CaseID, r.Seq, passedInt, string(r.Actual), r.Error, r.LatencyMs)
	}
	if _, err := s.db.ExecContext(ctx, b.String(), args...); err != nil {
		return fmt.Errorf("insert eval results: %w", err)
	}
	return nil
}

func (s *SQLite) ListEvalResultsForRun(ctx context.Context, runID string) ([]harness.EvalResult, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, run_id, case_id, seq, passed, actual, error, latency_ms
		 FROM eval_results WHERE run_id = ? ORDER BY seq`, runID,
	)
	if err != nil {
		return nil, fmt.Errorf("list eval results: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []harness.EvalResult
	for rows.Next() {
		var r harness.EvalResult
		var passed int
		var actual, errStr string
		if err := rows.Scan(&r.ID, &r.RunID, &r.CaseID, &r.Seq, &passed, &actual, &errStr, &r.LatencyMs); err != nil {
			return nil, err
		}
		r.Passed = passed != 0
		r.Actual = []byte(actual)
		r.Error = errStr
		out = append(out, r)
	}
	return out, rows.Err()
}
