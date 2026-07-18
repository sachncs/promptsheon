// Package approval owns the Approval aggregate.
//
// Repository is the consumer-defined persistence interface for the
// Approval aggregate. Storage implementations in internal/store satisfy
// this interface.
package approval

import (
	"context"
	"errors"
)

// ErrNotFound is returned by Repository implementations when a row is
// missing. It is package-local so callers do not need to import a
// storage-specific sentinel.
var ErrNotFound = errors.New("approval: not found")

// Repository persists Approval rows and their votes.
type Repository interface {
	CreateApproval(ctx context.Context, a *Approval) error
	GetApproval(ctx context.Context, releaseID string) (*Approval, error)
	UpdateApproval(ctx context.Context, a *Approval) error
	DeleteApproval(ctx context.Context, releaseID string) error
}
