// Package recommendation owns the Recommendation and Decision
// aggregates.
//
// Repository is the consumer-defined persistence interface for
// Recommendations and Decisions. A Recommendation without a Decision
// is an outstanding proposal; its Decision (whether Adopted,
// Rejected, or Superseded) is what closes the optimisation loop.
package recommendation

import (
	"context"
	"errors"

	"github.com/sachncs/promptsheon/internal/capability"
)

// Repository persists Recommendations and their Decisions.
type Repository interface {
	CreateRecommendation(ctx context.Context, r *capability.Recommendation) error
	GetRecommendation(ctx context.Context, id string) (*capability.Recommendation, error)
	ListRecommendations(ctx context.Context, capabilityVersionID string) ([]*capability.Recommendation, error)
	UpdateRecommendation(ctx context.Context, r *capability.Recommendation) error

	CreateDecision(ctx context.Context, d *Decision) error
	GetDecision(ctx context.Context, recommendationID string) (*Decision, error)
	ListDecisions(ctx context.Context) ([]*Decision, error)
}

var ErrNotFound = errors.New("recommendation: not found")
