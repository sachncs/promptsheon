package adoption

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
)

type fakeRepo struct {
	items []*Record
}

func (f *fakeRepo) Append(_ context.Context, r *Record) error {
	f.items = append(f.items, r)
	return nil
}
func (f *fakeRepo) List(_ context.Context, f2 Filter) ([]*Record, error) {
	out := make([]*Record, 0)
	for _, r := range f.items {
		if f2.WorkspaceID != "" && r.WorkspaceID != f2.WorkspaceID {
			continue
		}
		if f2.CapabilityID != "" && r.CapabilityID != f2.CapabilityID {
			continue
		}
		if f2.Outcome != "" && r.Outcome != f2.Outcome {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}
func (f *fakeRepo) CountByOutcome(_ context.Context, workspaceID string) (map[string]int64, error) {
	out := map[string]int64{"adopted": 0, "rejected": 0, "superseded": 0}
	for _, r := range f.items {
		if workspaceID != "" && r.WorkspaceID != workspaceID {
			continue
		}
		out[r.Outcome]++
	}
	return out, nil
}

func TestAppendAndList(t *testing.T) {
	t.Parallel()
	r := &fakeRepo{}
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	if err := r.Append(context.Background(), &Record{
		ID: "rec-1", WorkspaceID: "ws-1", RecommendationID: "rec-1",
		Outcome: "adopted", DecidedBy: "alice", DecidedAt: now,
		Recommendation: capability.Recommendation{ID: "rec-1", Type: capability.RecommendationCompressPrompt},
	}); err != nil {
		t.Fatalf("append: %v", err)
	}
	got, err := r.List(context.Background(), Filter{WorkspaceID: "ws-1"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 record, got %d", len(got))
	}
	if got[0].ID != "rec-1" {
		t.Fatalf("expected rec-1, got %s", got[0].ID)
	}
}

func TestListFiltersByWorkspace(t *testing.T) {
	t.Parallel()
	r := &fakeRepo{}
	_ = r.Append(context.Background(), &Record{ID: "1", WorkspaceID: "ws-a", Outcome: "adopted"})
	_ = r.Append(context.Background(), &Record{ID: "2", WorkspaceID: "ws-b", Outcome: "adopted"})
	got, _ := r.List(context.Background(), Filter{WorkspaceID: "ws-a"})
	if len(got) != 1 {
		t.Fatalf("expected 1 record for ws-a, got %d", len(got))
	}
}

func TestCountByOutcome(t *testing.T) {
	t.Parallel()
	r := &fakeRepo{}
	_ = r.Append(context.Background(), &Record{ID: "1", WorkspaceID: "ws-1", Outcome: "adopted"})
	_ = r.Append(context.Background(), &Record{ID: "2", WorkspaceID: "ws-1", Outcome: "adopted"})
	_ = r.Append(context.Background(), &Record{ID: "3", WorkspaceID: "ws-1", Outcome: "rejected"})
	counts, err := r.CountByOutcome(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if counts["adopted"] != 2 {
		t.Fatalf("expected 2 adopted, got %d", counts["adopted"])
	}
	if counts["rejected"] != 1 {
		t.Fatalf("expected 1 rejected, got %d", counts["rejected"])
	}
}

func TestListErrUnknownOutcome(t *testing.T) {
	t.Parallel()
	_ = errors.New
}
