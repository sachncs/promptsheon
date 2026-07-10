// Package policy defines Workspace-scoped Policies.
//
// Repository is the consumer-defined persistence interface for
// Policy bundles. A Policy bundle is a per-Workspace value; it
// constrains every Capability Version, Release, and Execution
// inside the Workspace.
package policy

import "context"

// Repository persists Policy bundles per Workspace.
type Repository interface {
	GetBundle(ctx context.Context, workspaceID string) (*Bundle, error)
	PutBundle(ctx context.Context, workspaceID string, b *Bundle) error
	DeleteBundle(ctx context.Context, workspaceID string) error
}
