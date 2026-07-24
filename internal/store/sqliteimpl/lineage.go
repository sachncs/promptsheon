package sqliteimpl

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/sachncs/promptsheon/internal/lineage"
)

type LineageRepository struct{ db *sql.DB }

func NewLineageRepository(db *sql.DB) *LineageRepository { return &LineageRepository{db: db} }

var _ lineage.Repository = (*LineageRepository)(nil)

func (r *LineageRepository) GetGraph(ctx context.Context, capabilityID string) (*lineage.Graph, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT parent_capability_id, parent_version, child_capability_id, child_version, source, recommendation_id, created_at, created_by, notes FROM lineage_edges WHERE capability_id = ? ORDER BY created_at ASC`, capabilityID)
	if err != nil {
		return nil, fmt.Errorf("lineage: query: %w", err)
	}
	defer func() { _ = rows.Close() }()
	g := &lineage.Graph{CapabilityID: capabilityID}
	for rows.Next() {
		var e lineage.Edge
		var source string
		var recommendationID sql.NullString
		if err := rows.Scan(&e.Parent.CapabilityID, &e.Parent.Version, &e.Child.CapabilityID, &e.Child.Version, &source, &recommendationID, &e.CreatedAt, &e.CreatedBy, &e.Notes); err != nil {
			return nil, fmt.Errorf("lineage: scan: %w", err)
		}
		e.Source = lineage.Source(source)
		if recommendationID.Valid {
			e.RecommendationID = recommendationID.String
		}
		g.Edges = append(g.Edges, e)
		g.UpdatedAt = e.CreatedAt
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return g, nil
}

func (r *LineageRepository) PutGraph(ctx context.Context, g *lineage.Graph) error {
	if g == nil {
		return errors.New("lineage: nil graph")
	}
	if err := g.Validate(); err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `DELETE FROM lineage_edges WHERE capability_id = ?`, g.CapabilityID); err != nil {
		return err
	}
	for i := range g.Edges {
		e := &g.Edges[i]
		var recommendationID any
		if e.RecommendationID != "" {
			recommendationID = e.RecommendationID
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO lineage_edges (capability_id, parent_capability_id, parent_version, child_capability_id, child_version, source, recommendation_id, created_at, created_by, notes) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, g.CapabilityID, e.Parent.CapabilityID, e.Parent.Version, e.Child.CapabilityID, e.Child.Version, e.Source, recommendationID, e.CreatedAt, e.CreatedBy, e.Notes); err != nil {
			return fmt.Errorf("lineage: insert edge: %w", err)
		}
	}
	return tx.Commit()
}
