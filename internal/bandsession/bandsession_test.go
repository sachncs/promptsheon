package bandsession

import (
	"context"
	"errors"
	"testing"

	"github.com/sachncs/promptsheon/internal/bandit"
	"github.com/sachncs/promptsheon/internal/banditstore"
)

func newTestSession(t *testing.T) (*Session, *banditstore.Store, *banditstore.InMemory) {
	t.Helper()
	im := banditstore.NewInMemory()
	store, err := banditstore.NewStore(im)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	sess, err := New(store, 42)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return sess, store, im
}

func TestNewRejectsNilStore(t *testing.T) {
	t.Parallel()
	if _, err := New(nil, 0); err == nil {
		t.Fatalf("expected error for nil store")
	}
}

func TestLoadEmpty(t *testing.T) {
	t.Parallel()
	s, _, _ := newTestSession(t)
	if err := s.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := s.ArmIDs(); len(got) != 0 {
		t.Fatalf("expected empty ArmIDs, got %v", got)
	}
}

func TestLoadSeedFromStore(t *testing.T) {
	t.Parallel()
	s, _, im := newTestSession(t)
	im.Put("arm-1", bandit.ArmPosterior{})
	if err := s.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := s.ArmIDs(); len(got) != 1 || got[0] != "arm-1" {
		t.Fatalf("expected [arm-1], got %v", got)
	}
}

func TestSelectBeforeLoadFails(t *testing.T) {
	t.Parallel()
	s, _, _ := newTestSession(t)
	if _, err := s.Select(); err == nil {
		t.Fatalf("expected error selecting before Load")
	}
}

func TestObserveRequiresLoaded(t *testing.T) {
	t.Parallel()
	s, _, _ := newTestSession(t)
	if err := s.Observe(context.Background(), "arm-1", true); err == nil {
		t.Fatalf("expected error observing before Load")
	}
}

func TestRegisterArmsSeedsAndPersists(t *testing.T) {
	t.Parallel()
	s, _, _ := newTestSession(t)
	if err := s.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := s.RegisterArms(context.Background(), []string{"new-arm"}); err != nil {
		t.Fatalf("RegisterArms: %v", err)
	}
	if got := s.ArmIDs(); len(got) != 1 || got[0] != "new-arm" {
		t.Fatalf("expected [new-arm], got %v", got)
	}
}

func TestObserveAndSelect(t *testing.T) {
	t.Parallel()
	s, _, _ := newTestSession(t)
	if err := s.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := s.RegisterArms(context.Background(), []string{"arm-1"}); err != nil {
		t.Fatalf("RegisterArms: %v", err)
	}
	if err := s.Observe(context.Background(), "arm-1", true); err != nil {
		t.Fatalf("Observe: %v", err)
	}
	m, err := s.PosteriorMean("arm-1")
	if err != nil {
		t.Fatalf("PosteriorMean: %v", err)
	}
	if m <= 0.5 {
		t.Fatalf("expected Mean > 0.5, got %f", m)
	}
}

func TestCloseFlushed(t *testing.T) {
	t.Parallel()
	s, _, _ := newTestSession(t)
	if err := s.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := s.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestPosteriorMeanRequiresLoaded(t *testing.T) {
	t.Parallel()
	s, _, _ := newTestSession(t)
	if _, err := s.PosteriorMean("x"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestPosteriorMeanUnknownArm(t *testing.T) {
	t.Parallel()
	s, _, _ := newTestSession(t)
	if err := s.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := s.PosteriorMean("unknown"); !errors.Is(err, bandit.ErrUnknownArm) {
		t.Fatalf("expected ErrUnknownArm, got %v", err)
	}
}
