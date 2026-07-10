package lineage

import (
	"errors"
	"testing"
	"time"
)

func ref(capabilityID string, version int) VersionRef {
	return VersionRef{CapabilityID: capabilityID, Version: version}
}

func TestAppendManual(t *testing.T) {
	t.Parallel()
	var g Graph
	got, err := g.AppendRecommendation(ref("cap", 1), ref("cap", 2), SourceManual, "", "alice", "tweak", time.Now())
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if got.CapabilityID != "cap" {
		t.Fatalf("expected capability inferred, got %s", got.CapabilityID)
	}
	if len(got.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(got.Edges))
	}
}

func TestAppendRecommendationRequiresID(t *testing.T) {
	t.Parallel()
	var g Graph
	if _, err := g.AppendRecommendation(ref("cap", 1), ref("cap", 2), SourceRecommendation, "", "alice", "", time.Now()); err == nil {
		t.Fatalf("expected error for missing recommendation_id")
	}
}

func TestRejectsSelfReference(t *testing.T) {
	t.Parallel()
	var g Graph
	if _, err := g.AppendRecommendation(ref("cap", 1), ref("cap", 1), SourceManual, "", "alice", "", time.Now()); !errors.Is(err, ErrSelfReference) {
		t.Fatalf("expected ErrSelfReference, got %v", err)
	}
}

func TestRejectsDuplicateEdge(t *testing.T) {
	t.Parallel()
	var g Graph
	g, _ = g.AppendRecommendation(ref("cap", 1), ref("cap", 2), SourceManual, "", "alice", "", time.Now())
	if _, err := g.AppendRecommendation(ref("cap", 1), ref("cap", 2), SourceManual, "", "alice", "", time.Now()); !errors.Is(err, ErrDuplicateEdge) {
		t.Fatalf("expected ErrDuplicateEdge, got %v", err)
	}
}

func TestRejectsInconsistentCapability(t *testing.T) {
	t.Parallel()
	g := Graph{CapabilityID: "cap-a"}
	if _, err := g.AppendRecommendation(ref("cap-b", 1), ref("cap-a", 2), SourceManual, "", "alice", "", time.Now()); !errors.Is(err, ErrInconsistentCapability) {
		t.Fatalf("expected ErrInconsistentCapability, got %v", err)
	}
}

func TestLatestAndDerivedFrom(t *testing.T) {
	t.Parallel()
	var g Graph
	g, _ = g.AppendRecommendation(ref("cap", 1), ref("cap", 2), SourceManual, "", "alice", "", time.Now())
	g, _ = g.AppendRecommendation(ref("cap", 2), ref("cap", 3), SourceRecommendation, "rec-1", "system:auto-optimizer", "", time.Now())

	latest, ok := g.Latest()
	if !ok || latest.Version != 3 {
		t.Fatalf("expected latest=3, got %+v ok=%v", latest, ok)
	}
	parent, ok := g.DerivedFrom(ref("cap", 3))
	if !ok || parent.Parent.Version != 2 || parent.Source != SourceRecommendation {
		t.Fatalf("expected parent v2 recommendation, got %+v", parent)
	}
	_, ok = g.DerivedFrom(ref("cap", 1))
	if ok {
		t.Fatalf("v1 should have no parent")
	}
}

func TestValidateRejectsMultipleParents(t *testing.T) {
	t.Parallel()
	var g Graph
	g, _ = g.AppendRecommendation(ref("cap", 1), ref("cap", 2), SourceManual, "", "alice", "", time.Now())
	g, _ = g.AppendRecommendation(ref("cap", 3), ref("cap", 2), SourceManual, "", "alice", "", time.Now())
	if err := g.Validate(); err == nil {
		t.Fatalf("expected validation error for shared child")
	}
}

func TestValidateEmptyGraph(t *testing.T) {
	t.Parallel()
	if err := (Graph{}).Validate(); err == nil {
		t.Fatalf("expected error for empty graph")
	}
}

func TestHasCapability(t *testing.T) {
	t.Parallel()
	if (Graph{}).HasCapability("cap") {
		t.Fatalf("empty graph should not match")
	}
}
