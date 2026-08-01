package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sachncs/promptsheon/promptsheon/settings"
)

// SQLite persistence for system_config.

func (s *SQLite) GetSystemConfig(ctx context.Context, key string) (settings.CRDTRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT value, updated_at, updated_by, replica_id, version_vector, tombstone, write_ts
		 FROM system_config WHERE key = ?`, key,
	)
	rec, err := scanSystemConfigRow(row)
	if err != nil {
		return settings.CRDTRecord{}, err
	}
	rec.Key = key
	return rec, nil
}

// SetSystemConfig upserts one key's full CRDT record. The
// resolver supplies the bumped vector; the store just
// persists it.
//
//nolint:gocritic // ponytail: CRDT records stay value types across the store boundary.

func (s *SQLite) SetSystemConfig(ctx context.Context, rec settings.CRDTRecord) error {
	vecJSON, err := encodeVersionVector(rec.VersionVector)
	if err != nil {
		return fmt.Errorf("set system_config %q: encode vector: %w", rec.Key, err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO system_config
		   (key, value, updated_at, updated_by, replica_id, version_vector, tombstone, write_ts)
		 VALUES (?, ?, CURRENT_TIMESTAMP, ?, ?, ?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET
		   value          = excluded.value,
		   updated_at     = excluded.updated_at,
		   updated_by     = excluded.updated_by,
		   replica_id     = excluded.replica_id,
		   version_vector = excluded.version_vector,
		   tombstone      = excluded.tombstone,
		   write_ts       = excluded.write_ts`,
		rec.Key, rec.Value, rec.UpdatedBy, rec.ReplicaID, vecJSON, boolToInt(rec.Tombstone), rec.WriteTS,
	)
	if err != nil {
		return fmt.Errorf("set system_config %q: %w", rec.Key, err)
	}
	return nil
}

// ListSystemConfig returns every row (including tombstones).
// The resolver filters tombstones out of the Get/List surface
// — see settings.Resolver.List.

func (s *SQLite) ListSystemConfig(ctx context.Context) ([]settings.CRDTRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT value, updated_at, updated_by, replica_id, version_vector, tombstone, write_ts, key
		 FROM system_config ORDER BY key`,
	)
	if err != nil {
		return nil, fmt.Errorf("list system_config: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []settings.CRDTRecord
	for rows.Next() {
		var rec settings.CRDTRecord
		var vecJSON string
		var tombstone int
		if err := rows.Scan(&rec.Value, &rec.UpdatedAt, &rec.UpdatedBy, &rec.ReplicaID, &vecJSON, &tombstone, &rec.WriteTS, &rec.Key); err != nil {
			return nil, err
		}
		rec.Tombstone = tombstone != 0
		vv, err := decodeVersionVector(vecJSON)
		if err != nil {
			return nil, fmt.Errorf("list system_config %q: decode vector: %w", rec.Key, err)
		}
		rec.VersionVector = vv
		out = append(out, rec)
	}
	return out, rows.Err()
}

// TestMergeSystemConfigPersistsWinner (in store_test.go) pins
// SETTINGS-CRDT-1: a remote write that dominates the local
// vector must be persisted as the new winner. The merge must
// not silently drop the remote's updates.

// MergeSystemConfig folds a batch of remote records into the
// local store. The per-key merge is the LWW semantics in
// settings.Merge; the resulting record replaces the local row.
// Records with the local replica as the writer are still
// applied (the CRDT is symmetric).

func (s *SQLite) MergeSystemConfig(ctx context.Context, _ string, records []settings.CRDTRecord) error {
	if len(records) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("merge system_config: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, remote := range records {
		if remote.Key == "" {
			continue
		}
		var local settings.CRDTRecord
		row := tx.QueryRowContext(ctx,
			`SELECT value, updated_at, updated_by, replica_id, version_vector, tombstone, write_ts
			 FROM system_config WHERE key = ?`, remote.Key,
		)
		var vecJSON string
		var tombstone int
		err := row.Scan(&local.Value, &local.UpdatedAt, &local.UpdatedBy, &local.ReplicaID, &vecJSON, &tombstone, &local.WriteTS)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("merge system_config %q: scan: %w", remote.Key, err)
		}
		local.Tombstone = tombstone != 0
		if err == nil {
			vv, derr := decodeVersionVector(vecJSON)
			if derr != nil {
				return fmt.Errorf("merge system_config %q: decode vector: %w", remote.Key, derr)
			}
			local.VersionVector = vv
		}
		merged := settings.Merge(local, remote)
		if merged.Key == "" {
			merged.Key = remote.Key
		}
		vecJSON, err = encodeVersionVector(merged.VersionVector)
		if err != nil {
			return fmt.Errorf("merge system_config %q: encode vector: %w", merged.Key, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO system_config
			   (key, value, updated_at, updated_by, replica_id, version_vector, tombstone, write_ts)
			 VALUES (?, ?, CURRENT_TIMESTAMP, ?, ?, ?, ?, ?)
			 ON CONFLICT(key) DO UPDATE SET
			   value          = excluded.value,
			   updated_at     = excluded.updated_at,
			   updated_by     = excluded.updated_by,
			   replica_id     = excluded.replica_id,
			   version_vector = excluded.version_vector,
			   tombstone      = excluded.tombstone,
			   write_ts       = excluded.write_ts`,
			merged.Key, merged.Value, merged.UpdatedBy, merged.ReplicaID, vecJSON, boolToInt(merged.Tombstone), merged.WriteTS,
		); err != nil {
			return fmt.Errorf("merge system_config %q: upsert: %w", merged.Key, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("merge system_config: commit: %w", err)
	}
	return nil
}

// scanSystemConfigRow reads one row from system_config into a
// CRDTRecord. Used by GetSystemConfig and MergeSystemConfig;
// the caller fills in rec.Key because the row scanner can't
// know which key was queried (GetSystemConfig fills it after
// the scan, MergeSystemConfig already has it).

func scanSystemConfigRow(row scannable) (settings.CRDTRecord, error) {
	var rec settings.CRDTRecord
	var vecJSON string
	var tombstone int
	err := row.Scan(&rec.Value, &rec.UpdatedAt, &rec.UpdatedBy, &rec.ReplicaID, &vecJSON, &tombstone, &rec.WriteTS)
	if errors.Is(err, sql.ErrNoRows) {
		return settings.CRDTRecord{}, sql.ErrNoRows
	}
	if err != nil {
		return settings.CRDTRecord{}, err
	}
	rec.Tombstone = tombstone != 0
	vv, err := decodeVersionVector(vecJSON)
	if err != nil {
		return settings.CRDTRecord{}, err
	}
	rec.VersionVector = vv
	return rec, nil
}

// encodeVersionVector marshals a vector map for SQLite. An
// empty map is the JSON "{}" string so we never store NULL
// (NULL would force the scanner to handle two distinct
// representations).

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func encodeVersionVector(v map[string]uint64) (string, error) {
	if v == nil {
		return "{}", nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// decodeVersionVector unmarshals a vector from SQLite. An
// empty-string column (legacy rows) becomes the empty map.

func decodeVersionVector(s string) (map[string]uint64, error) {
	if s == "" {
		return map[string]uint64{}, nil
	}
	out := map[string]uint64{}
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, err
	}
	return out, nil
}
