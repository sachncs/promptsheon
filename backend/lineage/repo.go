// Package lineage owns the Lineage aggregate.
//
// Repository is the consumer-defined persistence interface for
// Version lineage. Lineage edges close the audit gap between
// "v18 exists" and "v18 came from v17 because the optimiser
// recommended it on 2026-07-10 with confidence 0.92 and human
// alice accepted".
package lineage

import "context"

// Repository persists a Lineage Graph for each Capability.
type Repository interface {
	GetGraph(ctx context.Context, capabilityID string) (*Graph, error)
	PutGraph(ctx context.Context, g *Graph) error
}
