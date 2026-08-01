// Package promptsheon — in-flight OAuth state token store.
//
// This file exists at the promptsheon root (not in handlers/)
// so the *Server type can embed it without creating a cross-
// package dependency. Handlers in the future handlers/ subdir
// will dispatch through s.oauthStates as before.
package promptsheon

import (
	"context"
	"sync"
	"time"
)

// oauthStateStore holds in-flight OAuth state tokens. The previous
// implementation used a package-level `var` shared across all
// Server instances and tests, which made it impossible to run
// multiple servers in the same test binary without state
// leakage. The fix moves the store onto Server; helpers below
// remain package-level and dispatch to the active server, so
// existing call sites do not need to change.
type oauthStateStore struct {
	mu     sync.Mutex
	states map[string]time.Time
	stop   chan struct{}
	done   chan struct{}
}

func newOAuthStateStore() *oauthStateStore {
	return &oauthStateStore{
		states: make(map[string]time.Time),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
}

// start launches a janitor that removes expired entries every
// minute.
func (s *oauthStateStore) start(ctx context.Context) {
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stop:
				return
			case now := <-ticker.C:
				s.mu.Lock()
				for k, exp := range s.states {
					if now.After(exp) {
						delete(s.states, k)
					}
				}
				s.mu.Unlock()
			}
		}
	}()
}

func (s *oauthStateStore) stopJanitor() {
	select {
	case <-s.stop:
		// already stopped
	default:
		close(s.stop)
	}
	<-s.done
}

func (s *oauthStateStore) put(state string, exp time.Time) {
	s.mu.Lock()
	s.states[state] = exp
	s.mu.Unlock()
}

func (s *oauthStateStore) consume(state string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.states[state]
	if !ok {
		return false
	}
	delete(s.states, state)
	return time.Now().Before(exp)
}
