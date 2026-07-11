package banditstore

import (
	"context"
	"testing"

	"github.com/sachncs/promptsheon/internal/bandit"
)

func TestNewStoreRejectsNil(t *testing.T) {
	t.Parallel()
	if _, err := NewStore(nil); err == nil {
		t.Fatalf("expected error for nil backend")
	}
}

func TestInMemoryLoadAllEmpty(t *testing.T) {
	t.Parallel()
	im := NewInMemory()
	got, err := im.LoadAll(context.Background())
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %d", len(got))
	}
}

func TestInMemorySaveAllRoundTrip(t *testing.T) {
	t.Parallel()
	im := NewInMemory()
	im.m["arm-1"] = bandit.ArmPosterior{}
	im.m["arm-2"] = bandit.ArmPosterior{}
	_ = im.SaveAll(context.Background(), im.m)
	if _, ok := im.m["arm-1"]; !ok {
		t.Fatal("arm-1 should be present after save")
	}
}

func TestInMemorySaveAllReplaces(t *testing.T) {
	t.Parallel()
	im := NewInMemory()
	im.m["arm-1"] = bandit.ArmPosterior{}
	_ = im.SaveAll(context.Background(), im.m)
	if _, ok := im.m["arm-1"]; !ok {
		t.Fatal("arm-1 should still be present after save")
	}
	_ = im.SaveAll(context.Background(), map[string]bandit.ArmPosterior{})
	if _, ok := im.m["arm-1"]; ok {
		t.Fatal("arm-1 should be gone after empty save")
	}
}

func TestStoreLoadEmpty(t *testing.T) {
	t.Parallel()
	im := NewInMemory()
	s, err := NewStore(im)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := s.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(s.ArmIDs()) != 0 {
		t.Fatalf("expected empty ArmIDs, got %v", s.ArmIDs())
	}
}

func TestStorePutGetRoundTrip(t *testing.T) {
	t.Parallel()
	s, _ := NewStore(NewInMemory())
	s.Put("arm-1", bandit.ArmPosterior{})
	if _, ok := s.Get("arm-1"); !ok {
		t.Fatal("expected arm-1 to be present")
	}
	if got := s.ArmIDs(); len(got) != 1 || got[0] != "arm-1" {
		t.Fatalf("expected [arm-1], got %v", got)
	}
}

func TestStoreReconcileSeed(t *testing.T) {
	t.Parallel()
	s, _ := NewStore(NewInMemory())
	s.ReconcileSeed([]string{"arm-1", "arm-2"})
	if got := s.ArmIDs(); len(got) != 2 {
		t.Fatalf("expected 2 arm ids, got %v", got)
	}
	s.Put("arm-1", bandit.ArmPosterior{})
	s.ReconcileSeed([]string{"arm-1"})
	if _, ok := s.Get("arm-1"); !ok {
		t.Fatal("arm-1 should still be present after re-seed")
	}
}

func TestStoreFlush(t *testing.T) {
	t.Parallel()
	im := NewInMemory()
	s, _ := NewStore(im)
	s.Put("arm-1", bandit.ArmPosterior{})
	if err := s.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	got, err := im.LoadAll(context.Background())
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if _, ok := got["arm-1"]; !ok {
		t.Fatal("expected arm-1 in InMemory after Flush")
	}
}

func TestStoreFlushReplace(t *testing.T) {
	t.Parallel()
	im := NewInMemory()
	im.m["stale"] = bandit.ArmPosterior{}
	s, _ := NewStore(im)
	if err := s.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if _, ok := im.m["stale"]; ok {
		t.Fatal("Flush should replace the backend map")
	}
}
