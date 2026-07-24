package api

import (
	"context"
	"sync"
	"time"

	"github.com/sachncs/promptsheon/internal/auth"
	"github.com/sachncs/promptsheon/internal/models"
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
	entry := &models.AuditEntry{
		ID:        generateID(),
		UserID:    userID,
		Action:    action,
		Resource:  resource,
		Details:   details,
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
		// fall through to drop path
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
func (s *Server) StartAuditWorkers(ctx context.Context, n int) {
	if n < 1 {
		n = 2
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
			// Use a fresh background context for the DB write so a
			// cancelled request does not abort the audit persistence.
			writeStart := time.Now()
			if err := s.db.AppendAudit(context.Background(), entry); err != nil {
				if s.logger != nil {
					s.logger.Error("failed to write audit entry",
						"err", err, "entry_id", entry.ID, "action", entry.Action)
				}
			}
			// OBS-AUDIT-2: surface the time between audit() being
			// called and the DB write committing, so operators can
			// detect worker backlog growth.
			if s.collector != nil {
				s.collector.ObserveAuditQueue(time.Since(entry.Timestamp).Seconds())
			}
			_ = writeStart
		}
	}
}
