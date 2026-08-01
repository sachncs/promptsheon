// Package api implements the HTTP REST API for Promptsheon.
package promptsheon

import (
	"github.com/sachncs/promptsheon/promptsheon/invoke"
	"github.com/sachncs/promptsheon/promptsheon/release"
	"github.com/sachncs/promptsheon/promptsheon/metrics"
	"github.com/sachncs/promptsheon/promptsheon/workflow"
	"github.com/sachncs/promptsheon/promptsheon/settings"
	"github.com/sachncs/promptsheon/promptsheon/alerting"
	"github.com/sachncs/promptsheon/promptsheon/auth"
	"github.com/sachncs/promptsheon/promptsheon/models"
	"github.com/sachncs/promptsheon/promptsheon/llm"
	"github.com/sachncs/promptsheon/promptsheon/ratelimit"
	"github.com/sachncs/promptsheon/promptsheon/harness"
	"github.com/sachncs/promptsheon/promptsheon/store"
	"github.com/sachncs/promptsheon/promptsheon/trace"
	"github.com/sachncs/promptsheon/promptsheon/rollups"
	"github.com/sachncs/promptsheon/promptsheon/webhook"
	"github.com/sachncs/promptsheon/promptsheon/vault"
	"context"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"


	_ "github.com/sachncs/promptsheon/promptsheon/eval" // Scorer registry (no Server dep yet)
)

// Func is the handler signature that returns errors for centralized handling.
type Func func(http.ResponseWriter, *http.Request) error

// Server holds dependencies and routes for the HTTP API.
type Server struct {
	mux             *http.ServeMux
	db              *store.DB
	logger          *slog.Logger
	authn           *auth.Authenticator
	requireAuth     bool
	spans           trace.Tracer
	collector       *metrics.Collector
	webhooks        *webhook.Dispatcher
	vault           *vault.Vault
	oauth           *auth.OAuthManager
	logHub          *Hub
	elector         *Elector
	alertingManager *alerting.Manager
	rateLimiter     *ratelimit.Limiter
	providers       *llm.Registry
	rollupAgg       *rollups.Aggregator
	invoker         *invoke.Invoker
	releaseResolver *release.Resolver
	releaseSvc      *release.Service

	// settingsMode + settingsNotif back the /api/v1/settings
	// surface. settingsMode is "mutable" by default; "env-only"
	// disables writes (operator can still read).
	settingsMode      string
	settingsNotif     *settings.Notifier
	settingsReplicaID string
	harnessSvc        *harness.EvalRunner

	// startTime is captured at server construction. Used by the
	// /health endpoint to report uptime.
	startTime time.Time

	// auditMu guards audit lifecycle state (queue, done chan,
	// stopTriggered flag, cancel function). Replacing the
	// previous sync.Once-based design is the 1.8 fix: sync.Once
	// is unsafe to reset after first use, and the prior code
	// re-zeroed it inside StartAuditWorkers, which silently
	// leaked entries between successive Start calls.
	auditMu            sync.Mutex
	auditQueue         chan *models.AuditEntry
	auditDropped       atomic.Int64
	auditWg            sync.WaitGroup
	auditStopTriggered bool
	auditDone          chan struct{}
	// auditCancel cancels the per-worker context. Workers exit
	// either when the audit queue is closed and drained (the
	// happy shutdown path) or when auditCancel is called (the
	// forced shutdown path when StopAuditWorkers' drain budget
	// is exceeded).
	auditCancel context.CancelFunc

	// oauthStates holds in-flight OAuth state tokens for this
	// server. Stored on the Server (not as a package-level var) so
	// multiple servers in the same test binary do not share state.
	// The helpers (s.generateOAuthState, s.validateOAuthState)
	// dispatch via this field.
	oauthStates *oauthStateStore

	workflowEngine *workflow.Engine
}

// httpRequestKey is the context key used by the request middleware
// to attach the in-flight *http.Request for downstream helpers.
type httpRequestKey struct{}

// WithRequest returns a context that carries the current request.
func WithRequest(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, httpRequestKey{}, r)
}

// NewServer creates a new API server with the given dependencies.
//
// The legacy WithServerConfig / *ServerConfig options were
// removed; the fields they exposed (circuit breaker thresholds)
// are declared but never read by any code path. The breaker is
// configured per-provider via internal/llm.WithCircuitBreaker
// instead; if the production wiring wants a server-wide override,
// expose it through a fresh Option.
func NewServer(db *store.DB, logger *slog.Logger, opts ...Option) *Server {
	s := &Server{
		mux:         http.NewServeMux(),
		db:          db,
		logger:      logger,
		oauthStates: newOAuthStateStore(),
		startTime:   time.Now(),
	}
	// P0-1: the activeOAuthStates package-level pointer is gone.
	// Helpers dispatch through s.oauthStates (set above) and
	// tests construct a Server with the helper rather than
	// mutating a global.
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

// handleMetricsSummary returns the in-memory metrics summary
// (counters, histograms) for the operator dashboard.
// /metrics/dashboard is an alias for the same payload.
func (s *Server) handleMetricsSummary(w http.ResponseWriter, _ *http.Request) error {
	summary := s.collector.GetSummary()
	writeJSON(w, http.StatusOK, summary)
	return nil
}
