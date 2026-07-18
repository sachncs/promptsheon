package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/sachncs/promptsheon/internal/approval"
	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/release"
)

// ---------------------------------------------------------------------------
// Releases
// ---------------------------------------------------------------------------

func (s *SQLite) CreateRelease(ctx context.Context, r *release.Release) error {
	manifestJSON, err := marshalOrErr(r.Manifest)
	if err != nil {
		return fmt.Errorf("marshal release manifest: %w", err)
	}
	approvedByJSON, err := marshalOrErr(r.ApprovedBy)
	if err != nil {
		return fmt.Errorf("marshal release approved_by: %w", err)
	}
	var activatedAt, supersededAt sql.NullTime
	if r.ActivatedAt != nil {
		activatedAt = sql.NullTime{Time: *r.ActivatedAt, Valid: true}
	}
	if r.SupersededAt != nil {
		supersededAt = sql.NullTime{Time: *r.SupersededAt, Valid: true}
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO releases
		 (id, capability_id, capability_version, manifest, environment, status,
		  approved_by, superseded_by, replaces_release_id,
		  created_at, created_by, activated_at, superseded_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.CapabilityID, r.CapabilityVersion, string(manifestJSON),
		string(r.Environment), string(r.Status), string(approvedByJSON),
		r.SupersededBy, r.ReplacesReleaseID,
		r.CreatedAt, r.CreatedBy, activatedAt, supersededAt,
	)
	if err != nil {
		return fmt.Errorf("insert release: %w", err)
	}
	return nil
}

func (s *SQLite) GetRelease(ctx context.Context, id string) (*release.Release, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, capability_id, capability_version, manifest, environment, status,
		        approved_by, superseded_by, replaces_release_id,
		        created_at, created_by, activated_at, superseded_at
		 FROM releases WHERE id = ?`, id,
	)
	return scanRelease(row)
}

func (s *SQLite) ListReleasesForCapability(ctx context.Context, capabilityID string) ([]*release.Release, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, capability_id, capability_version, manifest, environment, status,
		        approved_by, superseded_by, replaces_release_id,
		        created_at, created_by, activated_at, superseded_at
		 FROM releases WHERE capability_id = ? ORDER BY created_at DESC`, capabilityID,
	)
	if err != nil {
		return nil, fmt.Errorf("list releases: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*release.Release
	for rows.Next() {
		r, err := scanRelease(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SQLite) ListActiveReleasesForEnvironment(ctx context.Context, env release.Environment) ([]*release.Release, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, capability_id, capability_version, manifest, environment, status,
		        approved_by, superseded_by, replaces_release_id,
		        created_at, created_by, activated_at, superseded_at
		 FROM releases WHERE environment = ? AND status = ? ORDER BY created_at DESC`,
		string(env), string(release.StatusActive),
	)
	if err != nil {
		return nil, fmt.Errorf("list active releases: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*release.Release
	for rows.Next() {
		r, err := scanRelease(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SQLite) UpdateRelease(ctx context.Context, r *release.Release) error {
	manifestJSON, err := marshalOrErr(r.Manifest)
	if err != nil {
		return fmt.Errorf("marshal release manifest: %w", err)
	}
	approvedByJSON, err := marshalOrErr(r.ApprovedBy)
	if err != nil {
		return fmt.Errorf("marshal release approved_by: %w", err)
	}
	var activatedAt, supersededAt sql.NullTime
	if r.ActivatedAt != nil {
		activatedAt = sql.NullTime{Time: *r.ActivatedAt, Valid: true}
	}
	if r.SupersededAt != nil {
		supersededAt = sql.NullTime{Time: *r.SupersededAt, Valid: true}
	}

	res, err := s.db.ExecContext(ctx,
		`UPDATE releases SET
			capability_id = ?, capability_version = ?, manifest = ?, environment = ?,
			status = ?, approved_by = ?, superseded_by = ?, replaces_release_id = ?,
			activated_at = ?, superseded_at = ?
		 WHERE id = ?`,
		r.CapabilityID, r.CapabilityVersion, string(manifestJSON), string(r.Environment),
		string(r.Status), string(approvedByJSON), r.SupersededBy, r.ReplacesReleaseID,
		activatedAt, supersededAt, r.ID,
	)
	if err != nil {
		return fmt.Errorf("update release: %w", err)
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

func (s *SQLite) DeleteRelease(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM releases WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete release: %w", err)
	}
	return nil
}

func scanRelease(scanner interface {
	Scan(dest ...any) error
}) (*release.Release, error) {
	var r release.Release
	var manifestJSON, approvedByJSON string
	var envStr, statusStr string
	var supersededBy, replacesReleaseID, createdBy sql.NullString
	var activatedAt, supersededAt sql.NullTime

	err := scanner.Scan(
		&r.ID, &r.CapabilityID, &r.CapabilityVersion, &manifestJSON,
		&envStr, &statusStr, &approvedByJSON,
		&supersededBy, &replacesReleaseID,
		&r.CreatedAt, &createdBy, &activatedAt, &supersededAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan release: %w", err)
	}

	r.Environment = release.Environment(envStr)
	r.Status = release.Status(statusStr)
	if supersededBy.Valid {
		r.SupersededBy = supersededBy.String
	}
	if replacesReleaseID.Valid {
		r.ReplacesReleaseID = replacesReleaseID.String
	}
	if createdBy.Valid {
		r.CreatedBy = createdBy.String
	}
	if activatedAt.Valid {
		t := activatedAt.Time
		r.ActivatedAt = &t
	}
	if supersededAt.Valid {
		t := supersededAt.Time
		r.SupersededAt = &t
	}
	if manifestJSON != "" && manifestJSON != "{}" {
		mustUnmarshal([]byte(manifestJSON), &r.Manifest)
	}
	if approvedByJSON != "" && approvedByJSON != "[]" {
		mustUnmarshal([]byte(approvedByJSON), &r.ApprovedBy)
	}
	return &r, nil
}

// ---------------------------------------------------------------------------
// Approvals
// ---------------------------------------------------------------------------

func (s *SQLite) CreateApproval(ctx context.Context, a *approval.Approval) error {
	votesJSON, err := marshalOrErr(a.Votes)
	if err != nil {
		return fmt.Errorf("marshal votes: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO approvals (release_id, votes, updated_at) VALUES (?, ?, ?)`,
		a.ReleaseID, string(votesJSON), a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert approval: %w", err)
	}
	return nil
}

func (s *SQLite) GetApproval(ctx context.Context, releaseID string) (*approval.Approval, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT release_id, votes, updated_at FROM approvals WHERE release_id = ?`, releaseID,
	)
	return scanApproval(row)
}

func (s *SQLite) UpdateApproval(ctx context.Context, a *approval.Approval) error {
	votesJSON, err := marshalOrErr(a.Votes)
	if err != nil {
		return fmt.Errorf("marshal votes: %w", err)
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE approvals SET votes = ?, updated_at = ? WHERE release_id = ?`,
		string(votesJSON), a.UpdatedAt, a.ReleaseID,
	)
	if err != nil {
		return fmt.Errorf("update approval: %w", err)
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

func (s *SQLite) DeleteApproval(ctx context.Context, releaseID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM approvals WHERE release_id = ?`, releaseID)
	if err != nil {
		return fmt.Errorf("delete approval: %w", err)
	}
	return nil
}

func scanApproval(scanner interface {
	Scan(dest ...any) error
}) (*approval.Approval, error) {
	var a approval.Approval
	var votesJSON string
	err := scanner.Scan(&a.ReleaseID, &votesJSON, &a.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan approval: %w", err)
	}
	if votesJSON != "" && votesJSON != "[]" {
		mustUnmarshal([]byte(votesJSON), &a.Votes)
	}
	return &a, nil
}

// CapabilityIDsForReleases is a small helper used by capability handlers
// when validating release->capability references; it is satisfied by the
// SQLite store and is intentionally not part of the public Repository
// interface to keep the latter narrow.
func (s *SQLite) CapabilityExists(ctx context.Context, id string) (bool, error) {
	var got string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM capabilities WHERE id = ?`, id).Scan(&got)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("capability lookup: %w", err)
	}
	return true, nil
}

// used to silence unused imports if any
var _ = time.Now
var _ = capability.Manifest{}
