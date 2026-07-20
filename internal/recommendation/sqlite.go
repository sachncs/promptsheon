// Package recommendation: SQLite-backed Repository implementation.
package recommendation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sachncs/promptsheon/internal/capability"
)

// SQLiteRepository is the production implementation of Repository
// backed by the daemon's SQLite handle. The schema is added in
// migration 042_recommendation.sql; this file is the read/write
// surface.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository constructs a Repository against the
// supplied database handle. The handle is borrowed, not owned:
// the caller is responsible for closing it.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

// CreateRecommendation persists a Recommendation.
func (r *SQLiteRepository) CreateRecommendation(ctx context.Context, rec *capability.Recommendation) error {
	if rec == nil {
		return errors.New("recommendation: nil")
	}
	if rec.ID == "" {
		return errors.New("recommendation: id is required")
	}
	payload, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("recommendation: marshal: %w", err)
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO recommendations (id, capability_version_id, type, payload, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		rec.ID, rec.CapabilityVersionID, rec.Type, string(payload), rec.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("recommendation: insert: %w", err)
	}
	return nil
}

// GetRecommendation returns a Recommendation by id.
func (r *SQLiteRepository) GetRecommendation(ctx context.Context, id string) (*capability.Recommendation, error) {
	var payload string
	err := r.db.QueryRowContext(ctx,
		`SELECT payload FROM recommendations WHERE id = ?`, id,
	).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var rec capability.Recommendation
	if err := json.Unmarshal([]byte(payload), &rec); err != nil {
		return nil, fmt.Errorf("recommendation: unmarshal: %w", err)
	}
	return &rec, nil
}

// ListRecommendations returns every Recommendation for a Capability
// Version, oldest first.
func (r *SQLiteRepository) ListRecommendations(ctx context.Context, capabilityVersionID string) ([]*capability.Recommendation, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT payload FROM recommendations
		 WHERE capability_version_id = ?
		 ORDER BY created_at ASC`, capabilityVersionID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*capability.Recommendation
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var rec capability.Recommendation
		if err := json.Unmarshal([]byte(payload), &rec); err != nil {
			return nil, err
		}
		out = append(out, &rec)
	}
	return out, rows.Err()
}

// UpdateRecommendation replaces the persisted row. Caller is
// responsible for keeping ID stable.
func (r *SQLiteRepository) UpdateRecommendation(ctx context.Context, rec *capability.Recommendation) error {
	payload, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("recommendation: marshal: %w", err)
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE recommendations SET payload = ?, capability_version_id = ?, type = ? WHERE id = ?`,
		string(payload), rec.CapabilityVersionID, rec.Type, rec.ID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateDecision persists a Decision against a Recommendation.
// The Decision.ID must be supplied by the caller (Decision
// constructors always set it). SQLite's nullable text PK would
// otherwise silently store NULL.
func (r *SQLiteRepository) CreateDecision(ctx context.Context, d *Decision) error {
	if d == nil {
		return errors.New("decision: nil")
	}
	if d.ID == "" {
		return errors.New("decision: id is required")
	}
	if d.RecommendationID == "" {
		return errors.New("decision: recommendation_id is required")
	}
	payload, err := json.Marshal(d)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO decisions (id, recommendation_id, payload, created_at)
		 VALUES (?, ?, ?, ?)`,
		d.ID, d.RecommendationID, string(payload), d.DecidedAt,
	)
	if err != nil {
		return err
	}
	return nil
}

// GetDecision returns the Decision for a Recommendation.
func (r *SQLiteRepository) GetDecision(ctx context.Context, recommendationID string) (*Decision, error) {
	var payload string
	err := r.db.QueryRowContext(ctx,
		`SELECT payload FROM decisions WHERE recommendation_id = ? ORDER BY created_at DESC LIMIT 1`,
		recommendationID,
	).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var d Decision
	if err := json.Unmarshal([]byte(payload), &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// ListDecisions returns every Decision, oldest first.
func (r *SQLiteRepository) ListDecisions(ctx context.Context) ([]*Decision, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT payload FROM decisions ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*Decision
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var d Decision
		if err := json.Unmarshal([]byte(payload), &d); err != nil {
			return nil, err
		}
		out = append(out, &d)
	}
	return out, rows.Err()
}

// ErrNotFound is returned when a Recommendation or Decision is
// missing.
var ErrNotFound = errors.New("recommendation: not found")
