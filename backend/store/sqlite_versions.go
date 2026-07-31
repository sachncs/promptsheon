package store
import (
	"context"
	"database/sql"
	"fmt"

	"github.com/sachncs/promptsheon/backend/errs"
	"github.com/sachncs/promptsheon/backend/capability"
)

// SQLite persistence for versions.

func (s *SQLite) CreateVersion(ctx context.Context, v *capability.Version) error {
	manifestJSON, err := marshalOrErr(v.Manifest)
	if err != nil {
		return fmt.Errorf("marshal version manifest: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO capability_versions
		 (id, capability_id, version, manifest, manifest_hash, created_at, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.CapabilityID, v.Version, string(manifestJSON), v.ManifestHash,
		v.CreatedAt, v.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("insert version: %w", err)
	}
	return nil
}


func (s *SQLite) GetVersion(ctx context.Context, id string) (*capability.Version, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, capability_id, version, manifest, manifest_hash, created_at, created_by
		 FROM capability_versions WHERE id = ?`, id,
	)
	return scanCapabilityVersion(row)
}


func (s *SQLite) ListVersions(ctx context.Context, capabilityID string) ([]*capability.Version, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, capability_id, version, manifest, manifest_hash, created_at, created_by
		 FROM capability_versions WHERE capability_id = ? ORDER BY version DESC`, capabilityID,
	)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []*capability.Version
	for rows.Next() {
		v, err := scanCapabilityVersion(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}


func (s *SQLite) GetLatestVersion(ctx context.Context, capabilityID string) (*capability.Version, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, capability_id, version, manifest, manifest_hash, created_at, created_by
		 FROM capability_versions WHERE capability_id = ? ORDER BY version DESC LIMIT 1`, capabilityID,
	)
	return scanCapabilityVersion(row)
}

// GetVersionByNumber returns the Version whose integer
// `version` column matches the supplied counter for the
// Capability. Used by the diff endpoint.

func (s *SQLite) GetVersionByNumber(ctx context.Context, capabilityID string, version int) (*capability.Version, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, capability_id, version, manifest, manifest_hash, created_at, created_by
		 FROM capability_versions WHERE capability_id = ? AND version = ?`, capabilityID, version,
	)
	return scanCapabilityVersion(row)
}


func scanCapabilityVersion(scanner interface {
	Scan(dest ...any) error
}) (*capability.Version, error) {
	var v capability.Version
	var manifestJSON string

	err := scanner.Scan(&v.ID, &v.CapabilityID, &v.Version,
		&manifestJSON, &v.ManifestHash, &v.CreatedAt, &v.CreatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, errs.ErrorStoreNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan version: %w", err)
	}

	if manifestJSON != "" && manifestJSON != "{}" {
		if err := mustUnmarshal([]byte(manifestJSON), &v.Manifest); err != nil {
			return nil, fmt.Errorf("version %s manifest: %w", v.ID, err)
		}
	}

	return &v, nil
}

// ---------------------------------------------------------------------------
// Executions
// ---------------------------------------------------------------------------


