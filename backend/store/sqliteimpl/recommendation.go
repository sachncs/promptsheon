package sqliteimpl

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/recommendation"
)

type RecommendationRepository struct{ db *sql.DB }

func NewRecommendationRepository(db *sql.DB) *RecommendationRepository {
	return &RecommendationRepository{db: db}
}

var _ recommendation.Repository = (*RecommendationRepository)(nil)

func (r *RecommendationRepository) CreateRecommendation(ctx context.Context, rec *capability.Recommendation) error {
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
	_, err = r.db.ExecContext(ctx, `INSERT INTO recommendations (id, capability_version_id, type, payload, created_at) VALUES (?, ?, ?, ?, ?)`, rec.ID, rec.CapabilityVersionID, rec.Type, string(payload), rec.CreatedAt)
	if err != nil {
		return fmt.Errorf("recommendation: insert: %w", err)
	}
	return nil
}

func (r *RecommendationRepository) GetRecommendation(ctx context.Context, id string) (*capability.Recommendation, error) {
	var payload string
	err := r.db.QueryRowContext(ctx, `SELECT payload FROM recommendations WHERE id = ?`, id).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, recommendation.ErrNotFound
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

func (r *RecommendationRepository) ListRecommendations(ctx context.Context, capabilityVersionID string) ([]*capability.Recommendation, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT payload FROM recommendations WHERE capability_version_id = ? ORDER BY created_at ASC`, capabilityVersionID)
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

func (r *RecommendationRepository) UpdateRecommendation(ctx context.Context, rec *capability.Recommendation) error {
	if rec == nil {
		return errors.New("recommendation: nil")
	}
	payload, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("recommendation: marshal: %w", err)
	}
	res, err := r.db.ExecContext(ctx, `UPDATE recommendations SET payload = ?, capability_version_id = ?, type = ? WHERE id = ?`, string(payload), rec.CapabilityVersionID, rec.Type, rec.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return recommendation.ErrNotFound
	}
	return nil
}

func (r *RecommendationRepository) CreateDecision(ctx context.Context, d *recommendation.Decision) error {
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
	_, err = r.db.ExecContext(ctx, `INSERT INTO decisions (id, recommendation_id, payload, created_at) VALUES (?, ?, ?, ?)`, d.ID, d.RecommendationID, string(payload), d.DecidedAt)
	return err
}

func (r *RecommendationRepository) GetDecision(ctx context.Context, recommendationID string) (*recommendation.Decision, error) {
	var payload string
	err := r.db.QueryRowContext(ctx, `SELECT payload FROM decisions WHERE recommendation_id = ? ORDER BY created_at DESC LIMIT 1`, recommendationID).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, recommendation.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var d recommendation.Decision
	if err := json.Unmarshal([]byte(payload), &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *RecommendationRepository) ListDecisions(ctx context.Context) ([]*recommendation.Decision, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT payload FROM decisions ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*recommendation.Decision
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var d recommendation.Decision
		if err := json.Unmarshal([]byte(payload), &d); err != nil {
			return nil, err
		}
		out = append(out, &d)
	}
	return out, rows.Err()
}
