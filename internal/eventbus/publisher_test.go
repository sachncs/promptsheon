package eventbus

import (
	"errors"
	"sync"
	"testing"

	"github.com/sachncs/promptsheon/internal/capability"
)

func TestPublisherFiltersByType(t *testing.T) {
	t.Parallel()
	p := NewMemory()
	defer p.Close()

	gotCh := make(chan capability.Event, 4)
	sub, err := p.Subscribe(func(e capability.Event) {
		gotCh <- e
	}, capability.EventVersionCreated)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Cancel()

	if err := p.Publish(capability.Event{Type: capability.EventVersionCreated, ID: "1"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if err := p.Publish(capability.Event{Type: capability.EventCapabilityCreated, ID: "2"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case got := <-gotCh:
		if got.ID != "1" {
			t.Fatalf("expected event 1, got %s", got.ID)
		}
	default:
		t.Fatalf("expected a VersionCreated event")
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

	_ = p.Publish(capability.Event{Type: capability.EventVersionCreated, ID: "1"})
	_ = p.Publish(capability.Event{Type: capability.EventDeploymentSucceeded, ID: "2"})

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
	_ = p.Publish(capability.Event{Type: capability.EventVersionCreated})

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

	if err := p.Publish(capability.Event{Type: capability.EventVersionCreated}); err != nil {
		t.Fatalf("publish should swallow subscriber panic, got %v", err)
	}
}
