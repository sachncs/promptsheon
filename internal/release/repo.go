// Package release owns the Release aggregate.
//
// Repository is the consumer-defined persistence interface for the
// Release aggregate. Storage implementations in internal/store satisfy
// this interface. A Postgres implementation added in M1 will be a
// drop-in replacement.
package release

import (
	"context"
	"errors"
)

// ErrNotFound is returned by Repository implementations when a row is
// missing. It is package-local so callers do not need to import a
// storage-specific sentinel.
var ErrNotFound = errors.New("release: not found")

// Repository persists Release rows.
type Repository interface {
	CreateRelease(ctx context.Context, r *Release) error
	GetRelease(ctx context.Context, id string) (*Release, error)
	ListReleasesForCapability(ctx context.Context, capabilityID string) ([]*Release, error)
	ListActiveReleasesForEnvironment(ctx context.Context, env Environment) ([]*Release, error)
	UpdateRelease(ctx context.Context, r *Release) error

	// ActivateAtomic supersedes the prior Release (if non-nil) and
	// persists the next Release in a single transaction. The
	// invariant "exactly one Active Release per (Capability, Environment)"
	// is upheld atomically: either both writes commit or neither does.
	//
	// Storage backends without transactional support may implement
	// this as two separate writes (and document the gap); the call
	// site does not need to know which.
	ActivateAtomic(ctx context.Context, prior, next *Release) error
}
