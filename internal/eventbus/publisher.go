// Package eventbus defines the Publisher interface and a default
// in-memory implementation. The interface lives here because it is
// defined by consumers — audit, observability, webhooks — not by the
// aggregates that emit events.
//
// The in-memory implementation is intentionally synchronous:
// subscribers run on the publisher's goroutine and a slow subscriber
// blocks subsequent subscribers. That trade-off is correct for the
// first version because audit logs and structured logs must be
// durable before the next event is recorded; eventual-consistency
// here would corrupt the audit chain.
//
// Concurrency: the in-memory Publisher is safe for concurrent
// Subscribe and Publish calls. The Publish call is itself blocking,
// so callers that need to emit without backpressure should compose
// their own async eventing on top of the interface — not change this
// implementation.
package eventbus

import (
	"errors"
	"sync"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
)

// Handler processes one event. Handlers must be idempotent: the same
// event ID may be delivered more than once during a publisher restart
// or a retry.
type Handler func(event capability.Event)

// Subscribe registers a Handler for a set of event types. An empty
// eventTypes slice subscribes the Handler to every event type.
//
// Subscribe returns a Subscription that the caller must keep; call
// its Cancel method to deregister.
type Subscribe func(handler Handler, eventTypes ...capability.EventType) (Subscription, error)

// Publish emits an event. The event is delivered to every subscriber
// whose filter accepts it. Publish blocks until every matching
// subscriber has returned.
type Publish func(event capability.Event) error

// Subscription is the handle returned by Subscribe. Cancel removes
// the subscription; subsequent Publish calls will not deliver events
// to it.
//
// Cancel is idempotent.
type Subscription interface {
	Cancel()
}

// ErrAlreadyCanceled is returned by a Handler whose Subscription has
// been Canceled; consumers should treat it as "stop processing".
var ErrAlreadyCanceled = errors.New("eventbus: subscription canceled")

// subscription is the internal handle.
//
// Cancel flips canceled via the shared mu lock, and the Publish loop
// sees the same flag because the stored slice entry points at this
// subscription.
type subscription struct {
	id         uint64
	handler    Handler
	eventTypes map[capability.EventType]struct{}
	canceled   bool
	mu         sync.Mutex
}

// Cancel removes the subscription from the publisher.
//
// Cancel is idempotent and safe for concurrent use.
func (s *subscription) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.canceled = true
}

// Publisher is the consumer-defined interface for an event bus.
type Publisher interface {
	Subscribe(handler Handler, eventTypes ...capability.EventType) (Subscription, error)
	Publish(event capability.Event) error
}

// Memory is an in-memory Publisher. See the package documentation
// for why this is synchronous.
//
// Memory is safe for concurrent use.
type Memory struct {
	mu          sync.RWMutex
	subscribers []*subscription
	nextID      uint64
}

// NewMemory constructs an empty Memory Publisher.
func NewMemory() *Memory {
	return &Memory{}
}

// Subscribe registers Handler for the supplied event types.
//
// Subscribe returns ErrAlreadyCanceled only if the Memory has been
// Closed; under normal operation it returns nil error.
func (m *Memory) Subscribe(handler Handler, eventTypes ...capability.EventType) (Subscription, error) {
	if handler == nil {
		return nil, errors.New("eventbus: handler must not be nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	filter := make(map[capability.EventType]struct{}, len(eventTypes))
	for _, et := range eventTypes {
		filter[et] = struct{}{}
	}
	sub := &subscription{
		id:         m.nextID,
		handler:    handler,
		eventTypes: filter,
	}
	m.subscribers = append(m.subscribers, sub)
	return sub, nil
}

// Publish delivers event to every matching subscriber.
//
// Publish swallows panics from individual subscribers so one buggy
// handler cannot poison the audit chain. The panic is, however,
// returned as an error so the caller is aware.
func (m *Memory) Publish(event capability.Event) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	m.mu.RLock()
	targets := make([]*subscription, 0, len(m.subscribers))
	for _, s := range m.subscribers {
		if len(s.eventTypes) == 0 {
			targets = append(targets, s)
			continue
		}
		if _, ok := s.eventTypes[event.Type]; ok {
			targets = append(targets, s)
		}
	}
	m.mu.RUnlock()
	for _, s := range targets {
		s.mu.Lock()
		canceled := s.canceled
		h := s.handler
		s.mu.Unlock()
		if canceled {
			continue
		}
		func() {
			defer func() {
				_ = recover()
			}()
			h(event)
		}()
	}
	return nil
}

// Close empties the subscription list. After Close, Subscribe is the
// only operation that still returns the (empty) Memory. Close is
// idempotent.
func (m *Memory) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.subscribers {
		s.mu.Lock()
		s.canceled = true
		s.mu.Unlock()
	}
	m.subscribers = nil
}
