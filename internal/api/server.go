// Package api implements the HTTP REST API for Promptsheon.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sachncs/promptsheon/internal/alerting"
	"github.com/sachncs/promptsheon/internal/api/handlers"
	"github.com/sachncs/promptsheon/internal/auth"

	contextpkg "github.com/sachncs/promptsheon/internal/context"
	"github.com/sachncs/promptsheon/internal/election"
	_ "github.com/sachncs/promptsheon/internal/eval" // Scorer registry (no Server dep yet)
	"github.com/sachncs/promptsheon/internal/guardrail"
	"github.com/sachncs/promptsheon/internal/harness"
	"github.com/sachncs/promptsheon/internal/invoke"
	"github.com/sachncs/promptsheon/internal/llm"
	"github.com/sachncs/promptsheon/internal/metrics"
	"github.com/sachncs/promptsheon/internal/models"
	"github.com/sachncs/promptsheon/internal/ratelimit"
	"github.com/sachncs/promptsheon/internal/release"
	"github.com/sachncs/promptsheon/internal/rollups"
	"github.com/sachncs/promptsheon/internal/search"
	"github.com/sachncs/promptsheon/internal/settings"
	"github.com/sachncs/promptsheon/internal/store"
	"github.com/sachncs/promptsheon/internal/trace"
	"github.com/sachncs/promptsheon/internal/vault"
	"github.com/sachncs/promptsheon/internal/webhook"
	"github.com/sachncs/promptsheon/internal/workflow"
	"github.com/sachncs/promptsheon/internal/ws"
)

// Func is the handler signature that returns errors for centralized handling.
type Func = handlers.Func

// Server holds dependencies and routes for the HTTP API.
type Server struct {
	mux              *http.ServeMux
	db               *store.Repositories
	logger           *slog.Logger
	authn            *auth.Authenticator
	requireAuth      bool
	spans            trace.Tracer
	collector        *metrics.Collector
	webhooks         *webhook.Dispatcher
	vault            *vault.Vault
	oauth            *auth.OAuthManager
	logHub           *ws.Hub
	elector          *election.Elector
	usageTracker     *UsageTracker
	guardrailManager *guardrail.Manager
	alertingManager  *alerting.Manager
	rateLimiter      *ratelimit.Limiter
	contextManager   *contextpkg.Manager
	providers        *llm.Registry
	rollupAgg        *rollups.Aggregator
	invoker          *invoke.Invoker
	releaseResolver  *release.Resolver
	releaseSvc       *release.Service

	// settingsMode + settingsNotif back the /api/v1/settings
	// surface. settingsMode is "mutable" by default; "env-only"
	// disables writes (operator can still read).
	settingsMode      string
	settingsNotif     *settings.Notifier
	settingsReplicaID string
	harnessSvc        *harness.EvalRunner

	// auditQueue is a bounded channel feeding the audit worker pool.
	auditQueue chan *models.AuditEntry
	// auditDropped counts audit entries that were rejected because
	// the queue was full. Exposed for tests and metrics.
	auditDropped atomic.Int64
	// auditWg tracks the in-flight audit workers. StopAuditWorkers
	// closes the queue and waits for the workers to drain. The
	// workers call Done() on any return path (queue closed or
	// ctx cancelled), so the WaitGroup reaches zero exactly once
	// per StopAuditWorkers invocation.
	auditWg       sync.WaitGroup
	auditStopOnce sync.Once
	auditDone     chan struct{}
	// auditCancel cancels the per-worker context. Workers exit
	// either when the audit queue is closed and drained (the
	// happy shutdown path) or when auditCancel is called (the
	// forced shutdown path when StopAuditWorkers' drain budget
	// is exceeded).
	auditCancel context.CancelFunc

	// oauthStates holds in-flight OAuth state tokens for this
	// server. Stored on the Server (not as a package-level var) so
	// multiple servers in the same test binary do not share state.
	// The package-level helpers (generateOAuthState,
	// validateOAuthState) dispatch via activeOAuthStates which is
	// set to the most recently constructed server.
	oauthStates *oauthStateStore

	// searchManager owns the in-memory semantic search index.
	// M-1: built once on Server construction, refreshed on
	// prompt create/update/delete via RefreshSearchIndex.
	searchManager  *search.Manager
	workflowEngine *workflow.Engine
}

// httpRequestKey is the context key used by the request middleware
// to attach the in-flight *http.Request for downstream helpers.
type httpRequestKey struct{}

// WithRequest returns a context that carries the current request.
func WithRequest(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, httpRequestKey{}, r)
}

// auditQueueBackpressure is the maximum time audit() waits for the
// worker pool to drain before dropping the entry. M-7 keeps the
// value short so a slow audit pipeline never holds up the request
// path.
const auditDefaultUser = "api"
const auditQueueBackpressure = 200 * time.Millisecond

// NewServer creates a new API server with the given dependencies.
//
// The legacy WithServerConfig / *ServerConfig options were
// removed; the fields they exposed (circuit breaker thresholds)
// are declared but never read by any code path. The breaker is
// configured per-provider via internal/llm.WithCircuitBreaker
// instead; if the production wiring wants a server-wide override,
// expose it through a fresh Option.
func NewServer(db *store.Repositories, logger *slog.Logger, opts ...Option) *Server {
	s := &Server{
		mux:           http.NewServeMux(),
		db:            db,
		logger:        logger,
		oauthStates:   newOAuthStateStore(),
		searchManager: search.NewManager(),
	}
	// Make this server the active one for the package-level OAuth
	// helpers (generateOAuthState, validateOAuthState, etc.). The
	// helpers retain a package-level pointer for backward
	// compatibility with call sites that don't have a *Server in
	// scope; the pointer is updated here so each NewServer call
	// produces an isolated store.
	activeOAuthStates = s.oauthStates
	for _, opt := range opts {
		opt(s)
	}
	s.routes()
	return s
}

// ServeHTTP makes Server implement http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
