package api

import (
	"github.com/sachncs/promptsheon/internal/alerting"
	"github.com/sachncs/promptsheon/internal/auth"
	contextpkg "github.com/sachncs/promptsheon/internal/context"
	"github.com/sachncs/promptsheon/internal/election"
	"github.com/sachncs/promptsheon/internal/guardrail"
	"github.com/sachncs/promptsheon/internal/harness"
	"github.com/sachncs/promptsheon/internal/invoke"
	"github.com/sachncs/promptsheon/internal/llm"
	"github.com/sachncs/promptsheon/internal/metrics"
	"github.com/sachncs/promptsheon/internal/ratelimit"
	"github.com/sachncs/promptsheon/internal/release"
	"github.com/sachncs/promptsheon/internal/rollups"
	"github.com/sachncs/promptsheon/internal/settings"
	"github.com/sachncs/promptsheon/internal/store"
	"github.com/sachncs/promptsheon/internal/trace"
	"github.com/sachncs/promptsheon/internal/vault"
	"github.com/sachncs/promptsheon/internal/webhook"
	"github.com/sachncs/promptsheon/internal/workflow"
	"github.com/sachncs/promptsheon/internal/ws"
)

// Option configures the Server.
type Option func(*Server)

// Authenticator returns the authenticator instance so the caller
// (e.g. main.go) can call Stop() at shutdown. Returns nil if
// authentication is disabled.
func (s *Server) Authenticator() *auth.Authenticator { return s.authn }

// StopDependents stops the lifecycle-managed dependencies that the
// server owns. main.go calls this after httpServer.Shutdown has
// drained in-flight HTTP requests, so any in-flight webhook Emit
// has already enqueued. The dispatcher drains those queued
// deliveries and then exits.
//
// The vault is stopped last so the in-memory master key is
// zeroized immediately before the daemon exits; any late
// request that races against the lifecycle is fail-closed via
// ErrStopped rather than decrypting against a torn-down cipher.
func (s *Server) StopDependents() {
	if s.webhooks != nil {
		s.webhooks.Stop()
	}
	if s.vault != nil {
		s.vault.Stop()
	}
}

// WithAuth enables authentication and authorization on the server.
func WithAuth(db store.APIKeys) Option {
	return func(s *Server) {
		adapter := &storeAuthAdapter{db: db}
		logger := &authAuditLogger{server: s}
		s.authn = auth.NewAuthenticatorWithLogger(adapter, logger)
		s.requireAuth = true
	}
}

// WithTracing attaches a trace.Tracer (used by the HTTP
// middleware to start/finish spans) and the metrics collector
// to the server.
func WithTracing(spans trace.Tracer, collector *metrics.Collector) Option {
	return func(s *Server) {
		s.spans = spans
		s.collector = collector
	}
}

// WithCollector attaches only the metrics collector. Tests use
// this when they want to assert on counter increments without
// pulling in a Tracer. Production callers should prefer
// WithTracing so OTel spans also flow.
func WithCollector(c *metrics.Collector) Option {
	return func(s *Server) {
		s.collector = c
	}
}

// WithWebhooks attaches a webhook dispatcher.
func WithWebhooks(d *webhook.Dispatcher) Option {
	return func(s *Server) {
		s.webhooks = d
	}
}

// WithVault attaches an encryption vault for provider key management.
func WithVault(v *vault.Vault) Option {
	return func(s *Server) {
		s.vault = v
	}
}

// WithOAuth attaches an OAuth manager for SSO authentication.
func WithOAuth(o *auth.OAuthManager) Option {
	return func(s *Server) {
		s.oauth = o
	}
}

// WithLogHub attaches a WebSocket hub for real-time log streaming.
func WithLogHub(h *ws.Hub) Option {
	return func(s *Server) {
		s.logHub = h
	}
}

// WithElector attaches a leader-election Elector. When set, the
// readiness handler reports the current leader and the role of
// this replica.
func WithElector(e *election.Elector) Option {
	return func(s *Server) {
		s.elector = e
	}
}

// WithUsageTracker attaches a usage tracker for top-used resources.
func WithUsageTracker(t *UsageTracker) Option {
	return func(s *Server) {
		s.usageTracker = t
	}
}

// WithGuardrailManager attaches a guardrail manager for policy enforcement.
func WithGuardrailManager(m *guardrail.Manager) Option {
	return func(s *Server) {
		s.guardrailManager = m
	}
}

// WithAlertingManager attaches an alerting manager for threshold monitoring.
func WithAlertingManager(m *alerting.Manager) Option {
	return func(s *Server) {
		s.alertingManager = m
	}
}

// WithContextManager sets the context manager for context assembly.
func WithContextManager(m *contextpkg.Manager) Option {
	return func(s *Server) {
		s.contextManager = m
	}
}

// WithProviders attaches the LLM provider Registry to the server. The
// Registry is the single source of truth for provider construction and
// lookup at runtime; it must be supplied by main, never shared as a
// package-level singleton. See ADR-0012.
func WithProviders(p *llm.Registry) Option {
	return func(s *Server) {
		s.providers = p
	}
}

// WithRateLimiter sets the rate limiter for the server.
func WithRateLimiter(l *ratelimit.Limiter) Option {
	return func(s *Server) {
		s.rateLimiter = l
	}
}

// WithWorkspaceRollups attaches the per-Workspace rollup
// aggregator. The GET /v1/workspaces/{id}/observation route
// queries this aggregator; when nil, the route returns an empty
// summary so the route is observable while the production rollup
// job is wired.
func WithWorkspaceRollups(a *rollups.Aggregator) Option {
	return func(s *Server) {
		s.rollupAgg = a
	}
}

// WithInvoker attaches the canonical invoke.Invoker that
// production wiring supplies. The /v1/versions/{id}/executions
// route calls the Invoker per request for Budget / Quota
// enforcement; when nil, the route records the execution as a stub.
func WithInvoker(i *invoke.Invoker) Option {
	return func(s *Server) {
		s.invoker = i
	}
}

// WithReleaseResolver attaches the canonical release Resolver.
// When set, the invoke-release handler builds a
// ResolvedInvocation from the release's manifest and uses its
// Provider/Model rather than honouring request-supplied values.
func WithReleaseResolver(r *release.Resolver) Option {
	return func(s *Server) { s.releaseResolver = r }
}

// WithWorkflowEngine attaches the workflow.Engine used by
// POST /api/v1/workflows/run. When nil the route returns 503
// (engine not configured) rather than 404, so callers can
// distinguish "missing" from "disabled".
func WithWorkflowEngine(e *workflow.Engine) Option {
	return func(s *Server) { s.workflowEngine = e }
}

// WithReleaseService attaches the release.Service used by the
// /releases routes. When nil, those routes are not registered
// and a release-aware request returns 404. This mirrors the
// pattern used by WithInvoker so callers can opt into release
// support incrementally.
func WithReleaseService(svc *release.Service) Option {
	return func(s *Server) {
		s.releaseSvc = svc
	}
}

// WithHarnessRunner attaches the harness eval runner used by the
// /datasets, /preconditions, and /evals routes. When nil, those
// routes are not registered.
func WithHarnessRunner(runner *harness.EvalRunner) Option {
	return func(s *Server) {
		s.harnessSvc = runner
	}
}

// WithSettings wires the settings notifier and the env-only
// write gate. The replicaID is the CRDT id used to attribute
// local writes; main.go generates one with crypto/rand and
// passes it in so a daemon restart creates a fresh replica
// but the same process keeps a stable id.
func WithSettings(notif *settings.Notifier, replicaID, mode string) Option {
	return func(s *Server) {
		s.settingsNotif = notif
		s.settingsReplicaID = replicaID
		s.settingsMode = mode
	}
}
