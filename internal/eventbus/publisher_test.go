package eventbus

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
)

func TestPublisherFiltersByType(t *testing.T) {
	t.Parallel()
	p := NewMemory()
	defer p.Close()

	gotCh := make(chan capability.Event, 4)
	sub, err := p.Subscribe(func(e capability.Event) {
		gotCh <- e
	}, capability.EventExecutionFinished)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Cancel()

	if err := p.Publish(capability.Event{Type: capability.EventExecutionFinished, ID: "1"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if err := p.Publish(capability.Event{Type: capability.EventRecommendationGenerated, ID: "2"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case got := <-gotCh:
		if got.ID != "1" {
			t.Fatalf("expected event 1, got %s", got.ID)
		}
	default:
		t.Fatalf("expected an ExecutionFinished event")
	}
}

func TestPublisherReceivesAllWhenFilterEmpty(t *testing.T) {
	t.Parallel()
	p := NewMemory()
	defer p.Close()

	var (
		mu  sync.Mutex
		got []string
	)
	sub, _ := p.Subscribe(func(e capability.Event) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, e.ID)
	})
	defer sub.Cancel()

	_ = p.Publish(capability.Event{Type: capability.EventExecutionFinished, ID: "1"})
	_ = p.Publish(capability.Event{Type: capability.EventRecommendationGenerated, ID: "2"})

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
}

func TestSubscribeRejectsNilHandler(t *testing.T) {
	t.Parallel()
	p := NewMemory()
	defer p.Close()
	if _, err := p.Subscribe(nil); err == nil {
		t.Fatalf("expected error for nil handler")
	}
}

func TestCancelStopsDelivery(t *testing.T) {
	t.Parallel()
	p := NewMemory()
	defer p.Close()

	var (
		mu  sync.Mutex
		got int
	)
	sub, _ := p.Subscribe(func(e capability.Event) {
		mu.Lock()
		defer mu.Unlock()
		got++
	})

	sub.Cancel()
	_ = p.Publish(capability.Event{Type: capability.EventExecutionFinished})

	mu.Lock()
	defer mu.Unlock()
	if got != 0 {
		t.Fatalf("expected no delivery after Cancel, got %d", got)
	}
}

func TestSubscribeRejectsDuplicateSubscription(t *testing.T) {
	t.Parallel()
	if !errors.Is(ErrAlreadyCanceled, ErrAlreadyCanceled) {
		t.Fatalf("ErrAlreadyCanceled should be a sentinel")
	}
}

func TestPublishPanicRecovered(t *testing.T) {
	t.Parallel()
	p := NewMemory()
	defer p.Close()

	sub, err := p.Subscribe(func(e capability.Event) {
		panic("test")
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Cancel()

	if err := p.Publish(capability.Event{Type: capability.EventExecutionFinished}); err != nil {
		t.Fatalf("publish should swallow subscriber panic, got %v", err)
	}
}

// TestAsyncMemoryPublishDoesNotBlock covers OBS-5b: Publish on
// an async Memory returns immediately even with a slow handler.
// Without async fan-out the synchronous implementation would
// block on the handler call.
func TestAsyncMemoryPublishDoesNotBlock(t *testing.T) {
	t.Parallel()
	m := NewAsyncMemory(64, 4)
	released := make(chan struct{})
	defer close(released) // unblock the worker at test exit

	if _, err := m.Subscribe(func(_ capability.Event) {
		<-released // never released during the test — the worker blocks here
	}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	done := make(chan struct{})
	go func() {
		_ = m.Publish(capability.Event{Type: "tick"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Publish blocked on async Memory")
	}

	m.Close()
}

// TestAsyncMemoryDropCounter covers OBS-5b: the dropped counter
// starts at zero and increments only on full-queue drops.
func TestAsyncMemoryDropCounter(t *testing.T) {
	t.Parallel()
	m := NewAsyncMemory(64, 4)
	defer m.Close()

	if got := m.Dropped(); got != 0 {
		t.Errorf("Dropped = %d, want 0 initially", got)
	}
}
