// Package promptsheon — audit log constants and the background
// worker pool that persists audit entries.
//
// The constants here (FieldKeyPref, KeyStatus, etc.) are
// re-exported via pkg/promptsheon for the SDK. The worker pool
// runs in its own goroutines and is owned by Server.
package promptsheon

import (
	"github.com/sachncs/promptsheon/errf"
	"github.com/sachncs/promptsheon/promptsheon/auth"
	"github.com/sachncs/promptsheon/promptsheon/models"
	"context"
	"time"

)

// AnonUser is the user ID recorded on audit rows when no caller
// is in scope (boot, unauthenticated routes, the auth-disabled
// loopback profile).
const AnonUser = "api"

// Field key names. The values are the on-the-wire JSON keys in
// the audit entry's Details map and in CSV exports.
const (
	KeyName       = "name"
	KeyStatus     = "status"
	KeyVersion    = "version"
	FieldAPIKey   = "api_key" // was "key"; renamed to be unambiguous
	FieldKeyPref  = "key_prefix"
	FieldKeyName  = "key_name"
	FieldProvider = "provider"
	// FieldProviderName is the human-friendly name of the provider
	// (e.g. "openai-production"). Distinct from FieldProvider
	// which is the machine identifier ("openai").
	FieldProviderName = "provider_name"
	FieldModel        = "model"
	FieldValue        = "value"
	FieldUserID       = "user_id"
	FieldEmail        = "email"
	FieldRole         = "role"
	FieldError        = "error"
	FieldOK           = "ok"
)

// auditQueueBackpressure is the maximum time audit() waits for the
// worker pool to drain before dropping the entry. M-7 keeps the
// value short so a slow audit pipeline never holds up the request
// path.
const auditQueueBackpressure = 200 * time.Millisecond

// audit writes an audit entry for a mutation. The user ID is taken
// from the request context (falling back to "anonymous" when auth
// is disabled or no caller is set). Entries are written by a small worker
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
	userID := AnonUser
	if u, ok := auth.UserFromContext(ctx); ok && u != nil && u.ID != "" {
		userID = u.ID
	}
	// HIGH-4 / DEF-15: deep-copy the details map so subsequent
	// mutations in this function don't affect the caller's map.
	// The previous code aliased the caller's map header and then
	// wrote "remote_addr" / "user_agent" into it; if the caller
	// reused the map across requests, later audit rows would
	// carry the previous request's remote address.
	entryDetails := map[string]any{}
	for k, v := range details {
		entryDetails[k] = v
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

// StartAuditWorkers launches the bounded worker pool.
//
// The workers hold their own context (auditCtx), independent of any
// caller-supplied context, so a SIGTERM that cancels the server
// root context does not interrupt the drain. The caller signals
// shutdown via StopAuditWorkers, which closes the queue and waits
// for the workers to drain before cancelling the worker context.
//
// StartAuditWorkers is one-shot. A second call returns an error.
func (s *Server) StartAuditWorkers(n int) error {
	if n < 1 {
		n = 2
	}
	s.auditMu.Lock()
	defer s.auditMu.Unlock()
	if s.auditCancel != nil {
		return errf.Errorf("audit workers already started; StartAuditWorkers is one-shot")
	}
	s.auditQueue = make(chan *models.AuditEntry, 1024)
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
	return nil
}

// StopAuditWorkers closes the audit queue and waits for the
// workers to drain the entries that are already enqueued. The
// wait is bounded by ctx: if ctx is cancelled before the workers
// finish, the function returns ctx.Err() and the workers
// continue draining in the background.
//
// Drain order: close the queue first so the workers see EOF and
// exit; only after Wait() returns do we cancel the per-worker
// context. Reversing the order reintroduces the drain-barrier bug
// (workers exit on context cancel before consuming the queue).
//
// StopAuditWorkers is safe to call multiple times. Subsequent
// calls are no-ops.
func (s *Server) StopAuditWorkers(ctx context.Context) error {
	s.auditMu.Lock()
	if s.auditQueue == nil {
		s.auditMu.Unlock()
		return nil
	}
	if s.auditStopTriggered {
		s.auditMu.Unlock()
		select {
		case <-s.auditDone:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.auditStopTriggered = true
	close(s.auditQueue)
	done := s.auditDone
	s.auditMu.Unlock()

	go func() {
		s.auditWg.Wait()
		close(done)
		if s.auditCancel != nil {
			s.auditCancel()
		}
	}()
	select {
	case <-done:
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
			s.handleAuditEntry(ctx, entry)
		}
	}
}

// handleAuditEntry processes a single audit entry, with panic
// recovery so a misbehaving AppendAudit implementation (e.g. a
// type assertion in a future refactor) cannot permanently shrink
// the audit worker pool. A recovered panic is logged and the
// entry is dropped — durability of individual entries is best-
// effort, but the worker keeps running.
func (s *Server) handleAuditEntry(ctx context.Context, entry *models.AuditEntry) {
	defer func() {
		if r := recover(); r != nil {
			if s.logger != nil {
				s.logger.Error("audit worker panic recovered",
					"err", r, "entry_id", entry.ID, "action", entry.Action)
			}
		}
	}()
	if err := s.db.AppendAudit(ctx, entry); err != nil {
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

// resetAuditForTest re-initialises the audit state for tests. It
// must not be used outside of test code.
func (s *Server) resetAuditForTest() {
	s.auditMu.Lock()
	defer s.auditMu.Unlock()
	if s.auditCancel != nil {
		s.auditCancel()
	}
	s.auditCancel = nil
	s.auditQueue = nil
	s.auditDone = nil
	s.auditStopTriggered = false
	s.auditDropped.Store(0)
}
