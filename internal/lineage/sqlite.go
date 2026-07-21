// Package lineage: SQLite-backed implementation of the lineage
// Repository interface. The lineage graph records the
// parent→child edges between capability versions, sourced from
// either a recommendation (auto-suggested by the optimizer) or
// a manual migration.
package lineage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sachncs/promptsheon/internal/capability"
)

// SQLiteRepository is the production implementation of Repository
// backed by the daemon's SQLite handle. Schema is added by
// migration 056_lineage.sql.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository constructs a Repository against the
// supplied database handle. The handle is borrowed, not owned.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

// AppendRecommendation records that the supplied Recommendation
// (when adopted) produced a new Capability Version. Source is
// "recommendation".
func (r *SQLiteRepository) AppendRecommendation(ctx context.Context, capabilityID, parentVersionID, childVersionID string, recommendation *capability.Recommendation) error {
	return r.append(ctx, capabilityID, parentVersionID, childVersionID, SourceRecommendation, recommendation.ID, recommendation.CreatedAt, "", "")
}

// AppendManual records a manual edge between two capability
// versions.
func (r *SQLiteRepository) AppendManual(ctx context.Context, capabilityID, parentVersionID, childVersionID, createdBy, notes string) error {
	return r.append(ctx, capabilityID, parentVersionID, childVersionID, SourceManual, "", nil, createdBy, notes)
}

func (r *SQLiteRepository) append(ctx context.Context, capabilityID, parent, child string, source Source, recID string, ts any, createdBy, notes string) error {
	if capabilityID == "" || parent == "" || child == "" {
		return errors.New("lineage: capability_id, parent, and child are required")
	}
	if ts == nil {
		ts = nowUTC()
	}
	notesJSON, err := json.Marshal(notes)
	if err != nil {
		return fmt.Errorf("lineage: marshal notes: %w", err)
	}
	recJSON := ""
	if recID != "" {
		recJSON = recID
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO lineage_edges (capability_id, parent, child, source, recommendation_id, created_at, created_by, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		capabilityID, parent, child, string(source), recJSON, ts, createdBy, string(notesJSON),
	)
	if err != nil {
		return fmt.Errorf("lineage: insert edge: %w", err)
	}
	return nil
}

// GetGraph returns the full lineage graph for a capability,
// oldest edges first.
func (r *SQLiteRepository) GetGraph(ctx context.Context, capabilityID string) (Graph, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT parent, child, source, recommendation_id, created_at, created_by, notes
		FROM lineage_edges
		WHERE capability_id = ?
		ORDER BY created_at ASC`, capabilityID)
	if err != nil {
		return Graph{}, fmt.Errorf("lineage: query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	g := Graph{CapabilityID: capabilityID}
	for rows.Next() {
		var e Edge
		var src string
		var notesJSON string
		var recID sql.NullString
		var createdBy sql.NullString
		if err := rows.Scan(&e.Parent, &e.Child, &src, &recID, &e.CreatedAt, &createdBy, &notesJSON); err != nil {
			return Graph{}, fmt.Errorf("lineage: scan: %w", err)
		}
		e.Source = Source(src)
		if recID.Valid {
			e.RecommendationID = recID.String
		}
		if createdBy.Valid {
			e.CreatedBy = createdBy.String
		}
		e.Notes = notesJSON
		g.Edges = append(g.Edges, e)
	}
	return g, rows.Err()
}
