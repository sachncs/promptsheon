package supervisor

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakePlugin struct {
	mu       sync.Mutex
	startN   int32
	stopN    int32
	healthN  int32
	stopErr  error
	startErr error
	healthOK bool
	hctx     context.Context
}

func (f *fakePlugin) Start(ctx context.Context) error {
	atomic.AddInt32(&f.startN, 1)
	f.mu.Lock()
	f.hctx = ctx
	f.mu.Unlock()
	return f.startErr
}
func (f *fakePlugin) Stop(ctx context.Context) error {
	atomic.AddInt32(&f.stopN, 1)
	return f.stopErr
}
func (f *fakePlugin) Health(ctx context.Context) error {
	atomic.AddInt32(&f.healthN, 1)
	if f.healthOK {
		return nil
	}
	return errors.New("not healthy")
}

// startErrOnce returns an error the first time Start is called, then
// nil. Used to test the "started failing then recovered" path.
type startErrOnce struct {
	once sync.Once
	err  error
}

func (s *startErrOnce) firstErr() error {
	s.once.Do(func() { s.err = errors.New("startup boom") })
	return s.err
}

type capturePublisher struct {
	mu     sync.Mutex
	events []PluginEvent
}

func (c *capturePublisher) Publish(ev PluginEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, ev)
}
func (c *capturePublisher) snapshot() []PluginEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]PluginEvent, len(c.events))
	copy(out, c.events)
	return out
}

func TestSupervisorRegistersAndLists(t *testing.T) {
	t.Parallel()
	s := New(nil, nil)
	p := &fakePlugin{}
	s.Register("test", p, RestartPolicy{Backoff: time.Millisecond, MaxBackoff: time.Millisecond})
	if got := s.List(); len(got) != 1 || got[0] != "test" {
		t.Fatalf("expected [test], got %v", got)
	}
}

func TestSupervisorRestartsOnHealthFailure(t *testing.T) {
	t.Parallel()
	pub := &capturePublisher{}
	s := New(pub, nil)
	p := &fakePlugin{healthOK: false}
	s.Register("test", p, RestartPolicy{MaxRestarts: 2, Backoff: time.Millisecond, MaxBackoff: 10 * time.Millisecond})
	// Drive one tick manually; in production the ticker does this.
	s.tick(context.Background())
	// Wait for the asynchronous Start goroutine to run.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && atomic.LoadInt32(&p.startN) < 1 {
		time.Sleep(10 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&p.startN); got != 1 {
		t.Fatalf("expected Start called once after tick, got %d", got)
	}
	// Second tick: still unhealthy, restarts once more.
	s.tick(context.Background())
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && atomic.LoadInt32(&p.startN) < 2 {
		time.Sleep(10 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&p.startN); got != 2 {
		t.Fatalf("expected Start called twice, got %d", got)
	}
	// Third tick: budget exhausted.
	s.tick(context.Background())
	if got := atomic.LoadInt32(&p.startN); got != 2 {
		t.Fatalf("expected Start at 2 (budget exhausted), got %d", got)
	}
	// Verify the publisher saw crashed and exhausted events.
	evs := pub.snapshot()
	var sawCrashed, sawExhausted bool
	for _, e := range evs {
		if e.Kind == "crashed" {
			sawCrashed = true
		}
		if e.Kind == "exhausted" {
			sawExhausted = true
		}
	}
	if !sawCrashed {
		t.Fatalf("expected crashed event, events=%+v", evs)
	}
	if !sawExhausted {
		t.Fatalf("expected exhausted event after budget, events=%+v", evs)
	}
}

func TestSupervisorHealthyPluginNotRestarted(t *testing.T) {
	t.Parallel()
	pub := &capturePublisher{}
	s := New(pub, nil)
	p := &fakePlugin{healthOK: true}
	s.Register("healthy", p, RestartPolicy{MaxRestarts: 5, Backoff: time.Millisecond, MaxBackoff: 10 * time.Millisecond})
	for i := 0; i < 3; i++ {
		s.tick(context.Background())
	}
	if got := atomic.LoadInt32(&p.startN); got != 0 {
		t.Fatalf("healthy plugin should not be restarted, Start called %d times", got)
	}
	if got := atomic.LoadInt32(&p.healthN); got < 3 {
		t.Fatalf("expected Health called at least 3 times, got %d", got)
	}
}

func TestSupervisorShutdownStopsPlugins(t *testing.T) {
	t.Parallel()
	s := New(nil, nil)
	p := &fakePlugin{}
	s.Register("test", p, RestartPolicy{Backoff: time.Millisecond, MaxBackoff: 10 * time.Millisecond})
	if err := s.shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if got := atomic.LoadInt32(&p.stopN); got != 1 {
		t.Fatalf("expected Stop called once, got %d", got)
	}
}

func TestSupervisorContextCancelStopsGracefully(t *testing.T) {
	t.Parallel()
	s := New(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- s.Run(ctx)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("supervisor did not stop within 2 seconds of context cancel")
	}
}

func TestSupervisorRunStartsAllPlugins(t *testing.T) {
	t.Parallel()
	s := New(nil, nil)
	p1 := &fakePlugin{healthOK: true}
	p2 := &fakePlugin{healthOK: true}
	s.Register("p1", p1, RestartPolicy{Backoff: time.Millisecond, MaxBackoff: 10 * time.Millisecond})
	s.Register("p2", p2, RestartPolicy{Backoff: time.Millisecond, MaxBackoff: 10 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	// Give Start time.
	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&p1.startN) == 0 {
		t.Fatalf("p1 Start not called")
	}
	if atomic.LoadInt32(&p2.startN) == 0 {
		t.Fatalf("p2 Start not called")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("Run did not exit after cancel")
	}
}
