// Package adoption captures the per-Workspace Recommendation adoption
// record. When a Recommendation is auto-applied (via recommendation.
// CanAutoAdopt) the adoption record is appended to the Workspace's
// history; operators can audit the loop end-to-end by reading this
// list.
//
// Production wiring supplies a backend-backed Repository; the
// Recommendation engine reads the prior adoption record and avoids
// re-recommending a Decision the Workspace already rejected.
package adoption

import (
	"context"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
)

// Record is one adoption event: a Recommendation that was
// Adopted, Rejected, or Superseded, plus the audit metadata.
type Record struct {
	ID               string                    `json:"id"`
	WorkspaceID      string                    `json:"workspace_id"`
	RecommendationID string                    `json:"recommendation_id"`
	CapabilityID     string                    `json:"capability_id"`
	Outcome          string                    `json:"outcome"` // adopted | rejected | superseded
	DecidedBy        string                    `json:"decided_by"`
	DecidedAt        time.Time                 `json:"decided_at"`
	ResultingVersion int                       `json:"resulting_version,omitempty"`
	Recommendation   capability.Recommendation `json:"recommendation"`
	Reason           string                    `json:"reason,omitempty"`
	Auto             bool                      `json:"auto"`
}

// Filter is the consumer-defined query shape.
type Filter struct {
	WorkspaceID  string
	CapabilityID string
	Outcome      string
	Limit        int
	Offset       int
}

// Repository is the consumer-defined persistence interface.
type Repository interface {
	Append(ctx context.Context, r *Record) error
	List(ctx context.Context, f Filter) ([]*Record, error)
	CountByOutcome(ctx context.Context, workspaceID string) (map[string]int64, error)
}
