package backend

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sachncs/promptsheon/backend/auth"
	"github.com/sachncs/promptsheon/backend/models"
)

// audit writes an audit entry for a mutation. The user ID is taken
// from the request context (falling back to "anonymous" when auth is
// disabled or no caller is set). Entries are written by a small worker
// pool so the request goroutine is never blocked by audit I/O and a
// burst of mutations cannot spawn one goroutine per write.
//
// The pool has a bounded queue. M-7 fix: when the queue is full we
// briefly wait (up to auditQueueBackpressure) for the workers to
// catch up, then drop and increment the counter. The previous
// behaviour dropped immediately under any backpressure, which made
// the audit log lose entries under transient spikes that the worker
// pool could otherwise have absorbed.
func (s *Server) audit(ctx context.Context, action, resource string, details map[string]any) {
	userID := auditDefaultUser
	if u, ok := auth.UserFromContext(ctx); ok && u != nil && u.ID != "" {
		userID = u.ID
	}
	// HIGH-4 / DEF-15: shallow-copy the details map so subsequent
	// mutations in this function don't affect the caller's map.
	// The previous code aliased the caller's map header and then
	// wrote "remote_addr" / "user_agent" into it; if the caller
	// reused the map across requests, later audit rows would
	// carry the previous request's remote address.
	entryDetails := details
	if details != nil {
		entryDetails = make(map[string]any, len(details)+2)
		for k, v := range details {
			entryDetails[k] = v
		}
	}
	entry := &models.AuditEntry{
		ID:        generateID(),
		UserID:    userID,
		Action:    action,
		Resource:  resource,
		Details:   entryDetails,
		Timestamp: time.Now(),
	}
	// Add the request's remote address and user-agent so forensic
	// analysis is possible from the audit log alone.
	if r := httpRequestFromContext(ctx); r != nil {
		if entry.Details == nil {
			entry.Details = map[string]any{}
		}
		entry.Details["remote_addr"] = r.RemoteAddr
		entry.Details["user_agent"] = r.UserAgent()
	}
	// Try the fast path first. If the queue is full, briefly wait
	// for a worker to drain. The wait is bounded so a request
	// cannot be blocked indefinitely by an overwhelmed audit pool.
	timer := time.NewTimer(auditQueueBackpressure)
	defer timer.Stop()
	select {
	case s.auditQueue <- entry:
		return
	case <-timer.C:
		// Queue still full: drop the entry and increment
		// the dropped counter below.
	}
	s.auditDropped.Add(1)
	// OBS-7: surface the drop count to the metrics collector so
	// /metrics/summary and the Prometheus scrape expose it.
	if s.collector != nil {
		s.collector.SetAuditDropped(s.auditDropped.Load())
	}
	if s.logger != nil {
		s.logger.Warn("audit queue full, entry dropped",
			"action", action, "resource", resource, "user_id", userID)
	}
}

// StartAuditWorkers launches the bounded worker pool. Call once at
// server startup. Cancel the context to stop the workers gracefully.
//
// 1.8: the previous code re-zeroed s.auditStopOnce on every call.
// StopAuditWorkers was guarded by sync.Once.Do, so a second
// StartAuditWorkers allocated a fresh queue that nothing could
// ever close via the Once; the workers exited only on
// auditCancel(). Result: anything that called audit() in between
// the two Starts lost entries forever.
//
// The function now returns an error on second-call; production
// wiring (daemon.go) panics on the error so a misconfiguration is
// caught at startup rather than silently dropping entries.
//
// The workers hold their own context (auditCtx), independent of the
// server root context. This is the fix for the drain-barrier bug:
// the previous design passed the rootCtx directly, so a SIGTERM
// that cancelled rootCtx immediately stopped the workers, leaving
// queued entries (key_mint, auth_failure, etc.) to be silently
// dropped. With the dedicated auditCtx, main.go can:
//
//  1. httpServer.Shutdown(ctx) — drains in-flight HTTP requests,
//     which produce audit entries via audit()
//  2. srv.StopAuditWorkers(drainCtx) — closes the queue; workers
//     drain whatever is left
//  3. auditCancel() — finally stop the worker goroutines
func (s *Server) StartAuditWorkers(ctx context.Context, n int) error {
	if n < 1 {
		n = 2
	}
	if s.auditCancel != nil {
		return fmt.Errorf("audit workers already started; StartAuditWorkers is one-shot")
	}
	s.auditQueue = make(chan *models.AuditEntry, 1024)
	s.auditStopOnce = sync.Once{}
	s.auditDone = make(chan struct{})
	auditCtx, auditCancel := context.WithCancel(context.Background())
	s.auditCancel = auditCancel
	for i := 0; i < n; i++ {
		s.auditWg.Add(1)
		// #nosec G118 -- auditCtx is owned by this Server and
		// cancelled by StopAuditWorkers (or its caller), not by
		// the request path.
		go s.auditWorker(auditCtx)
	}
	// Reference ctx to keep the signature stable for callers.
	_ = ctx
	return nil
}

// StopAuditWorkers closes the audit queue and waits for the
// workers to drain the entries that are already enqueued. The
// wait is bounded by ctx: if ctx is cancelled before the workers
// finish, the function returns ctx.Err() and the workers
// continue draining in the background.
//
// StopAuditWorkers is safe to call multiple times. Subsequent
// calls are no-ops.
//
// Drain order: close the queue first so the workers see EOF and
// exit; only after Wait() returns do we cancel the per-worker
// context. Reversing the order reintroduces the drain-barrier bug
// (workers exit on context cancel before consuming the queue).
func (s *Server) StopAuditWorkers(ctx context.Context) error {
	if s.auditQueue == nil {
		return nil
	}
	s.auditStopOnce.Do(func() {
		close(s.auditQueue)
		go func() {
			s.auditWg.Wait()
			close(s.auditDone)
		}()
	})
	select {
	case <-s.auditDone:
		if s.auditCancel != nil {
			s.auditCancel()
		}
		return nil
	case <-ctx.Done():
		// Caller's drain budget expired. Cancel the worker context
		// so the goroutines exit instead of leaking past process
		// shutdown.
		if s.auditCancel != nil {
			s.auditCancel()
		}
		return ctx.Err()
	}
}

func (s *Server) auditWorker(ctx context.Context) {
	defer s.auditWg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case entry, ok := <-s.auditQueue:
			if !ok {
				return
			}
			s.handleAuditEntry(entry)
		}
	}
}

// handleAuditEntry processes a single audit entry, with panic
// recovery so a misbehaving AppendAudit implementation (e.g. a
// type assertion in a future refactor) cannot permanently shrink
// the audit worker pool. A recovered panic is logged and the
// entry is dropped — durability of individual entries is best-
// effort, but the worker keeps running.
// AuditEntry handles the request.
func (s *Server) handleAuditEntry(entry *models.AuditEntry) {
	defer func() {
		if r := recover(); r != nil {
			if s.logger != nil {
				s.logger.Error("audit worker panic recovered",
					"err", r, "entry_id", entry.ID, "action", entry.Action)
			}
		}
	}()
	if err := s.db.AppendAudit(context.Background(), entry); err != nil {
		if s.logger != nil {
			s.logger.Error("failed to write audit entry",
				"err", err, "entry_id", entry.ID, "action", entry.Action)
		}
		return
	}
	if s.collector != nil {
		s.collector.ObserveAuditQueue(time.Since(entry.Timestamp).Seconds())
	}
}
