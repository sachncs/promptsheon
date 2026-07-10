// Package lineage tracks the relationship between Capability
// Versions: the parent Version that a new Version was derived from,
// and the Recommendation that motivated the derivation.
//
// Lineage closes the audit gap between "v18 exists" and "v18 came
// from v17 because the optimizer recommended it on 2026-07-10 with
// confidence 0.92 and human alice accepted". Without this aggregate,
// the optimization loop is a black box.
package lineage

import (
	"errors"
	"fmt"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
)

// Edge is one derivation: parent Version -> child Version, possibly
// motivated by a Recommendation.
//
// Edge is value-immutable. Append methods return new slices.
type Edge struct {
	ID               string     `json:"id"`
	Parent           VersionRef `json:"parent"`
	Child            VersionRef `json:"child"`
	RecommendationID string     `json:"recommendation_id,omitempty"`
	Source           Source     `json:"source"`
	CreatedAt        time.Time  `json:"created_at"`
	CreatedBy        string     `json:"created_by"`
	Notes            string     `json:"notes,omitempty"`
}

// VersionRef identifies the participating Versions.
//
// CapabilityID + Version captures a particular immutable Version;
// the tuple is what makes lineage safe across renames.
type VersionRef struct {
	CapabilityID string `json:"capability_id"`
	Version      int    `json:"version"`
}

// Source classifies how the child Version came to exist.
//
// recommendation: the optimizer proposed the child.
// manual: an engineer committed the child without an optimizer
// prompt.
// migration: a legacy import or replay-into-history produced the
// child.
type Source string

const (
	SourceRecommendation Source = "recommendation"
	SourceManual         Source = "manual"
	SourceMigration      Source = "migration"
)

// Graph is the lineage of one Capability. Edges are kept in append
// order; the Graph is the audit trail for that Capability.
//
// Graph is value-immutable.
type Graph struct {
	CapabilityID string    `json:"capability_id"`
	Edges        []Edge    `json:"edges"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ErrUnknownSource is returned when a Source is not in the closed set.
var ErrUnknownSource = errors.New("lineage: unknown source")

// ErrSelfReference is returned when an Edge would have a Version as
// its own parent.
var ErrSelfReference = errors.New("lineage: child cannot be its own parent")

// ErrDuplicateEdge is returned when an Edge with the same parent and
// child already exists in the Graph.
var ErrDuplicateEdge = errors.New("lineage: edge already exists")

// ErrInconsistentCapability is returned when an Edge references a
// Capability different from the Graph's Capability.
var ErrInconsistentCapability = errors.New("lineage: edge references a different capability")

// AppendRecommendation records that the child Version was derived
// from the parent as a result of the named Recommendation. If
// recommendationID is empty, the Edge is treated as having no
// specific motivation (which is valid for manual changes).
func (g Graph) AppendRecommendation(parent, child VersionRef, source Source, recommendationID, createdBy, notes string, at time.Time) (Graph, error) {
	if source == "" {
		source = SourceManual
	}
	switch source {
	case SourceRecommendation, SourceManual, SourceMigration:
	default:
		return g, fmt.Errorf("%w: %q", ErrUnknownSource, source)
	}
	if g.CapabilityID != "" && parent.CapabilityID != g.CapabilityID {
		return g, fmt.Errorf("%w: parent %s vs graph %s", ErrInconsistentCapability, parent.CapabilityID, g.CapabilityID)
	}
	if g.CapabilityID != "" && child.CapabilityID != g.CapabilityID {
		return g, fmt.Errorf("%w: child %s vs graph %s", ErrInconsistentCapability, child.CapabilityID, g.CapabilityID)
	}
	if source == SourceRecommendation && recommendationID == "" {
		return g, errors.New("lineage: recommendation source requires recommendation_id")
	}
	if parent == child {
		return g, ErrSelfReference
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	for _, e := range g.Edges {
		if e.Parent == parent && e.Child == child {
			return g, ErrDuplicateEdge
		}
	}
	edge := Edge{
		ID:               fmt.Sprintf("lineage:%s:%d->%d", parent.CapabilityID, parent.Version, child.Version),
		Parent:           parent,
		Child:            child,
		RecommendationID: recommendationID,
		Source:           source,
		CreatedAt:        at,
		CreatedBy:        createdBy,
		Notes:            notes,
	}
	if g.CapabilityID == "" {
		g.CapabilityID = parent.CapabilityID
	}
	g.Edges = append(append([]Edge{}, g.Edges...), edge)
	g.UpdatedAt = at
	return g, nil
}

// Latest returns the most recent child the Graph knows about, or the
// zero VersionRef if the Graph is empty.
func (g Graph) Latest() (VersionRef, bool) {
	if len(g.Edges) == 0 {
		return VersionRef{}, false
	}
	return g.Edges[len(g.Edges)-1].Child, true
}

// DerivedFrom returns the parent edge of child, if any. This answers
// the question "what Version produced this one?" for any Version in
// the Graph.
func (g Graph) DerivedFrom(child VersionRef) (Edge, bool) {
	for _, e := range g.Edges {
		if e.Child == child {
			return e, true
		}
	}
	return Edge{}, false
}

// HasCapability returns true if the Graph covers the named
// Capability. A new Graph is empty and returns false until the first
// Append* call lands.
func (g Graph) HasCapability(id string) bool {
	return g.CapabilityID == id && id != ""
}

// Validate checks structural correctness without consulting storage.
func (g Graph) Validate() error {
	if g.CapabilityID == "" {
		return errors.New("lineage: graph capability_id is required")
	}
	seen := make(map[VersionRef]struct{}, len(g.Edges))
	for i, e := range g.Edges {
		if e.Parent.CapabilityID != g.CapabilityID || e.Child.CapabilityID != g.CapabilityID {
			return fmt.Errorf("lineage: edge %d references a different capability", i)
		}
		if e.Parent == e.Child {
			return fmt.Errorf("lineage: edge %d is a self reference", i)
		}
		key := e.Child
		if _, dup := seen[key]; dup {
			return fmt.Errorf("lineage: child %v has multiple parents", key)
		}
		seen[key] = struct{}{}
	}
	return nil
}

var _ = capability.ArtifactPrompt
