package bandsession

import (
	"context"
	"errors"
	"testing"

	"github.com/sachncs/promptsheon/internal/bandit"
	"github.com/sachncs/promptsheon/internal/banditstore"
	"github.com/sachncs/promptsheon/internal/metrics"
)

func newTestSession(t *testing.T) (*Session, *banditstore.Store, *banditstore.InMemory) {
	t.Helper()
	im := banditstore.NewInMemory()
	store, err := banditstore.NewStoreWithReplica(im, "rep-test")
	if err != nil {
		t.Fatalf("NewStoreWithReplica: %v", err)
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
	im.Seed("rep-test", "arm-1", bandit.Counter{Successes: 3, Failures: 1})
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

func TestRegisterArmsDoesNotInventObservations(t *testing.T) {
	t.Parallel()
	s, _, im := newTestSession(t)
	if err := s.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := s.RegisterArms(context.Background(), []string{"arm-1"}); err != nil {
		t.Fatalf("RegisterArms: %v", err)
	}
	// The (replica, arm) row exists, but its Counter is zero;
	// the prior Beta(1, 1) is the cold-start posterior, not a
	// synthetic success/failure tally.
	im.Seed("rep-other", "arm-1", bandit.Counter{Successes: 0, Failures: 0})
	st, err := im.Load(context.Background())
	if err != nil {
		t.Fatalf("backend load: %v", err)
	}
	if c := st["arm-1"]; c.Successes != 0 || c.Failures != 0 {
		t.Fatalf("register invented observations: got %+v", c)
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

func TestObservePersistsViaCRDT(t *testing.T) {
	t.Parallel()
	s, _, im := newTestSession(t)
	if err := s.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterArms(context.Background(), []string{"arm-1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Observe(context.Background(), "arm-1", true); err != nil {
		t.Fatal(err)
	}
	if err := s.Observe(context.Background(), "arm-1", true); err != nil {
		t.Fatal(err)
	}
	if err := s.Observe(context.Background(), "arm-1", false); err != nil {
		t.Fatal(err)
	}
	st, _ := im.Load(context.Background())
	if got := st["arm-1"]; got.Successes != 2 || got.Failures != 1 {
		t.Fatalf("expected {2,1}, got %+v", got)
	}
}

func TestObserveCoherentAcrossSelectorAndStore(t *testing.T) {
	t.Parallel()
	s, _, _ := newTestSession(t)
	if err := s.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterArms(context.Background(), []string{"arm-1", "arm-2"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if err := s.Observe(context.Background(), "arm-1", true); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 3; i++ {
		if err := s.Observe(context.Background(), "arm-2", false); err != nil {
			t.Fatal(err)
		}
	}
	m1, _ := s.PosteriorMean("arm-1")
	m2, _ := s.PosteriorMean("arm-2")
	if m1 <= m2 {
		t.Fatalf("expected arm-1 mean > arm-2 mean, got %f vs %f", m1, m2)
	}
}

func TestSelectionMetricsBridge(t *testing.T) {
	s, _, _ := newTestSession(t)
	collector := metrics.NewCollector()
	s.SetSelectionObserver(collector)
	if err := s.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterArms(context.Background(), []string{"arm-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Select(); err != nil {
		t.Fatal(err)
	}
	summary := collector.GetSummary()
	if summary.BanditMetrics.SelectionsTotal != 1 || summary.BanditMetrics.CurrentRunID != s.RunID() {
		t.Fatalf("unexpected bandit metrics: %+v", summary.BanditMetrics)
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

func TestSessionReplicaIDStable(t *testing.T) {
	t.Parallel()
	s, st, _ := newTestSession(t)
	if s.runID == "" {
		t.Fatal("expected non-empty run id")
	}
	if st.ReplicaID() == "" {
		t.Fatal("expected non-empty store replica id")
	}
}

// TestObserveBackendFailureLeavesSelectorUnchanged pins the
// "validate, persist, mutate" ordering: when the backend
// rejects an Observe, the selector's posterior for the arm
// must be exactly what it was before the call. A failing
// backend must never silently advance the in-memory state
// because the next Select() would draw from a posterior the
// store never recorded.
func TestObserveBackendFailureLeavesSelectorUnchanged(t *testing.T) {
	t.Parallel()
	s, st, _ := newTestSession(t)
	if err := s.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterArms(context.Background(), []string{"arm-1"}); err != nil {
		t.Fatal(err)
	}
	// Snapshot the posterior before the failing call.
	before, ok := s.selector.Posterior("arm-1")
	if !ok {
		t.Fatal("arm-1 must be registered before the failing observe")
	}
	// Inject a backend that fails every Observe. The Session
	// must propagate the error and leave the selector
	// untouched.
	st.SetReplicaID("rep-test")
	if err := replaceStoreBackend(s, failingBackend{}); err != nil {
		t.Fatalf("replace backend: %v", err)
	}
	if err := s.Observe(context.Background(), "arm-1", true); err == nil {
		t.Fatal("expected backend failure to surface")
	}
	after, ok := s.selector.Posterior("arm-1")
	if !ok {
		t.Fatal("arm-1 must still be registered after the failing observe")
	}
	if before != after {
		t.Fatalf("selector mutated by failing backend: before=%+v after=%+v", before, after)
	}
}

// failingBackend is a Backend that rejects every Observe so
// the session's "fail-loud, leave selector untouched" path
// is the only thing under test.
type failingBackend struct{}

func (failingBackend) Load(context.Context) (bandit.State, error) {
	return bandit.State{}, nil
}

func (failingBackend) Observe(_ context.Context, _, _ string, _ bool) error {
	return errors.New("backend down")
}

func (failingBackend) Merge(context.Context, string, bandit.State) error {
	return errors.New("backend down")
}

// replaceStoreBackend swaps the backend on a Store. The
// production Store has no setter for the backend (the
// replica-id setter exists but the backend is immutable), so
// we construct a fresh Store, copy the replica id, and
// replace s.store in place. The selector wiring is unchanged.
func replaceStoreBackend(s *Session, b banditstore.Backend) error {
	replicaID := s.store.ReplicaID()
	newStore, err := banditstore.NewStoreWithReplica(b, replicaID)
	if err != nil {
		return err
	}
	s.store = newStore
	return nil
}
