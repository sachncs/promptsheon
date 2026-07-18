// Package api implements the HTTP REST API for Promptsheon.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sachncs/promptsheon/internal/alerting"
	"github.com/sachncs/promptsheon/internal/auth"
	contextpkg "github.com/sachncs/promptsheon/internal/context"
	_ "github.com/sachncs/promptsheon/internal/eval" // Scorer registry (no Server dep yet)
	"github.com/sachncs/promptsheon/internal/guardrail"
	"github.com/sachncs/promptsheon/internal/invoke"
	"github.com/sachncs/promptsheon/internal/llm"
	"github.com/sachncs/promptsheon/internal/metrics"
	"github.com/sachncs/promptsheon/internal/models"
	"github.com/sachncs/promptsheon/internal/ratelimit"
	"github.com/sachncs/promptsheon/internal/release"
	"github.com/sachncs/promptsheon/internal/rollups"
	"github.com/sachncs/promptsheon/internal/search"
	"github.com/sachncs/promptsheon/internal/store"
	"github.com/sachncs/promptsheon/internal/trace"
	"github.com/sachncs/promptsheon/internal/vault"
	"github.com/sachncs/promptsheon/internal/webhook"
	"github.com/sachncs/promptsheon/internal/ws"
)

// Func is the handler signature that returns errors for centralized handling.
type Func func(http.ResponseWriter, *http.Request) error

// Server holds dependencies and routes for the HTTP API.
type Server struct {
	mux              *http.ServeMux
	db               store.Repository
	logger           *slog.Logger
	cfg              *ServerConfig
	authn            *auth.Authenticator
	requireAuth      bool
	spans            *trace.SQLite
	collector        *metrics.Collector
	webhooks         *webhook.Dispatcher
	vault            *vault.Vault
	oauth            *auth.OAuthManager
	logHub           *ws.Hub
	usageTracker     *UsageTracker
	guardrailManager *guardrail.Manager
	alertingManager  *alerting.Manager
	rateLimiter      *ratelimit.Limiter
	contextManager   *contextpkg.Manager
	providers        *llm.Registry
	rollupAgg        *rollups.Aggregator
	invoker          *invoke.Invoker
	releaseSvc       *release.Service

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
	auditWg sync.WaitGroup

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
	searchManager *search.Manager
}

// httpRequestKey is the context key used by the request middleware
// to attach the in-flight *http.Request for downstream helpers.
type httpRequestKey struct{}

// WithRequest returns a context that carries the current request.
func WithRequest(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, httpRequestKey{}, r)
}

// ServerConfig holds configuration for the API server.
type ServerConfig struct {
	CircuitBreakerFailureThreshold int
	CircuitBreakerSuccessThreshold int
	CircuitBreakerCooldown         int
}

// auditQueueBackpressure is the maximum time audit() waits for the
// worker pool to drain before dropping the entry. M-7 keeps the
// value short so a slow audit pipeline never holds up the request
// path.
const auditDefaultUser = "api"
const auditQueueBackpressure = 200 * time.Millisecond

// Option configures the Server.
type Option func(*Server)

// WithAuth enables authentication and authorization on the server.
func WithAuth(db store.Repository) Option {
	return func(s *Server) {
		adapter := &storeAuthAdapter{db: db}
		logger := &authAuditLogger{server: s}
		s.authn = auth.NewAuthenticatorWithLogger(adapter, logger)
		s.requireAuth = true
	}
}

// WithTracing attaches a trace store and metrics collector to the server.

// WithTracing attaches a trace store and metrics collector to the server.
func WithTracing(spans *trace.SQLite, collector *metrics.Collector) Option {
	return func(s *Server) {
		s.spans = spans
		s.collector = collector
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
// aggregator. The Tier 2.37 GET /v1/workspaces/{id}/observation
// route queries this aggregator; when nil, the route returns
// an empty summary so the route is observable while the
// production rollup job ships in M3 follow-on.
func WithWorkspaceRollups(a *rollups.Aggregator) Option {
	return func(s *Server) {
		s.rollupAgg = a
	}
}

// WithInvoker attaches the canonical invoke.Invoker that
// production wiring supplies. The Tier 2.36 follow-on route
// /v1/versions/{id}/executions calls the Invoker per request
// for Budget / Quota enforcement; when nil, the route records
// the execution as a stub.
func WithInvoker(i *invoke.Invoker) Option {
	return func(s *Server) {
		s.invoker = i
	}
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

// WithServerConfig sets the server configuration.
func WithServerConfig(cfg *ServerConfig) Option {
	return func(s *Server) {
		s.cfg = cfg
	}
}

// NewServer creates a new API server with the given dependencies.
func NewServer(db store.Repository, logger *slog.Logger, opts ...Option) *Server {
	s := &Server{
		mux:    http.NewServeMux(),
		db:     db,
		logger: logger,
		cfg: &ServerConfig{
			CircuitBreakerFailureThreshold: 5,
			CircuitBreakerSuccessThreshold: 3,
			CircuitBreakerCooldown:         30,
		},
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

func (s *Server) routes() {
	s.registerHealthRoutes()
	s.registerAuthRoutes()
	s.registerAuditRoutes()
	s.registerTracingRoutes()
	s.registerMetricsRoutes()
	s.registerProviderRoutes()
	s.registerVaultRoutes()
	s.registerAlertRoutes()
	s.registerWebhookRoutes()
	s.registerWorkspaceRoutes()
	s.registerProjectRoutes()
	s.registerCapabilityRoutes()
	s.registerVersionRoutes()
	s.registerExecutionRoutes()
	s.registerReleaseRoutes()
}

func (s *Server) registerHealthRoutes() {
	s.mux.HandleFunc("GET /health", s.wrapHandler(s.handleHealth))
	s.mux.HandleFunc("GET /ready", s.wrapHandler(s.handleReady))
	s.mux.HandleFunc("GET /api/v1/version", s.wrapHandler(s.handleVersion))
	if s.collector != nil {
		s.mux.Handle("GET /metrics", s.collector.Handler())
	}
}

func (s *Server) registerAuthRoutes() {
	createKey := s.handleCreateAPIKey
	listKeys := s.handleListAPIKeys
	revokeKey := s.handleRevokeAPIKey
	if s.rateLimiter != nil {
		createKey = s.rateLimit(createKey)
		listKeys = s.rateLimit(listKeys)
		revokeKey = s.rateLimit(revokeKey)
	}
	s.mux.HandleFunc("POST /api/v1/apikeys", s.wrapHandler(createKey))
	s.mux.HandleFunc("GET /api/v1/apikeys", s.wrapHandler(listKeys))
	s.mux.HandleFunc("DELETE /api/v1/apikeys/{id}", s.wrapHandler(revokeKey))
	s.mux.HandleFunc("GET /api/v1/auth/{provider}/login", s.wrapHandler(s.handleOAuthLogin))
	s.mux.HandleFunc("GET /api/v1/auth/{provider}/callback", s.wrapHandler(s.handleOAuthCallback))
	s.mux.HandleFunc("POST /api/v1/setup", s.wrapHandler(s.handleBootstrap))
	s.mux.HandleFunc("GET /api/v1/users", s.wrapHandler(s.requirePerm(auth.PermUserManage)(s.handleListUsers)))
	s.mux.HandleFunc("POST /api/v1/users", s.wrapHandler(s.requirePerm(auth.PermUserManage)(s.handleCreateUser)))
	s.mux.HandleFunc("GET /api/v1/users/{id}", s.wrapHandler(s.requirePerm(auth.PermUserManage)(s.handleGetUser)))
	s.mux.HandleFunc("PUT /api/v1/users/{id}", s.wrapHandler(s.requirePerm(auth.PermUserManage)(s.handleUpdateUser)))
	s.mux.HandleFunc("DELETE /api/v1/users/{id}", s.wrapHandler(s.requirePerm(auth.PermUserManage)(s.handleDeleteUser)))
}

func (s *Server) registerAuditRoutes() {
	s.mux.HandleFunc("GET /api/v1/audit", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleListAudit)))
	s.mux.HandleFunc("GET /api/v1/audit/export", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleExportAudit)))
	s.mux.HandleFunc("GET /api/v1/audit/verify", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleVerifyAuditChain)))
	s.mux.HandleFunc("GET /api/v1/logs/search", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleSearchSpans)))
	s.mux.HandleFunc("GET /api/v1/logs/stream", s.wrapHandler(s.handleLogsStream))
}

func (s *Server) registerTracingRoutes() {
	s.mux.HandleFunc("GET /api/v1/traces", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleListSpans)))
	s.mux.HandleFunc("GET /api/v1/traces/{id}", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleGetSpan)))
	s.mux.HandleFunc("GET /api/v1/traces/tree/{trace_id}", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleGetTraceTree)))
}

func (s *Server) registerMetricsRoutes() {
	s.mux.HandleFunc("GET /api/v1/metrics/summary", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleMetricsSummary)))
	s.mux.HandleFunc("GET /api/v1/metrics/top-capabilities", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleTopCapabilities)))
	s.mux.HandleFunc("GET /api/v1/metrics/dashboard", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleDashboardSummary)))
	s.mux.HandleFunc("GET /api/v1/metrics", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleMetricsPrometheus)))
}

func (s *Server) registerProviderRoutes() {
	s.mux.HandleFunc("GET /api/v1/providers", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleListProviders)))
	s.mux.HandleFunc("GET /api/v1/providers/{name}", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleGetProvider)))
	s.mux.HandleFunc("POST /api/v1/providers/{name}/test", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleTestProvider)))
}

func (s *Server) registerVaultRoutes() {
	s.mux.HandleFunc("POST /api/v1/vault/keys", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleSaveVaultKey)))
	s.mux.HandleFunc("GET /api/v1/vault/keys", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleListVaultKeys)))
	s.mux.HandleFunc("DELETE /api/v1/vault/keys/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleDeleteVaultKey)))
}

func (s *Server) registerAlertRoutes() {
	s.mux.HandleFunc("GET /api/v1/alerts/rules", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleListAlertRules)))
	s.mux.HandleFunc("POST /api/v1/alerts/rules", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleCreateAlertRule)))
	s.mux.HandleFunc("GET /api/v1/alerts/rules/{id}", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleGetAlertRule)))
	s.mux.HandleFunc("PUT /api/v1/alerts/rules/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleUpdateAlertRule)))
	s.mux.HandleFunc("DELETE /api/v1/alerts/rules/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleDeleteAlertRule)))
	s.mux.HandleFunc("POST /api/v1/alerts/notifications", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleAddNotificationGroup)))
	s.mux.HandleFunc("GET /api/v1/alerts/active", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleListAlerts)))
	s.mux.HandleFunc("PUT /api/v1/alerts/active/{id}/resolve", s.wrapHandler(s.requirePerm(auth.PermReviewApprove)(s.handleResolveAlert)))
}

func (s *Server) registerWebhookRoutes() {
	s.mux.HandleFunc("GET /api/v1/webhooks", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleListWebhooks)))
	s.mux.HandleFunc("POST /api/v1/webhooks", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleCreateWebhook)))
	s.mux.HandleFunc("DELETE /api/v1/webhooks/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleDeleteWebhook)))
}

func (s *Server) registerWorkspaceRoutes() {
	s.mux.HandleFunc("GET /api/v1/workspaces", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleListWorkspaces)))
	s.mux.HandleFunc("POST /api/v1/workspaces", s.wrapHandler(s.requirePerm(auth.PermPromptCreate)(s.handleCreateWorkspace)))
	s.mux.HandleFunc("GET /api/v1/workspaces/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleGetWorkspace)))
	s.mux.HandleFunc("PUT /api/v1/workspaces/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleUpdateWorkspace)))
	s.mux.HandleFunc("DELETE /api/v1/workspaces/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptDelete)(s.handleDeleteWorkspace)))
	// Per-Workspace rollup (Tier 2.37): Budget / Quota consumption
	// aggregated at the current moment. The handler returns an
	// empty summary if no Aggregator is configured; production
	// wiring sets rollupAgg via WithWorkspaceRollups in a follow-on.
	s.mux.HandleFunc("GET /api/v1/workspaces/{id}/observation", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleGetWorkspaceObservation)))
}

func (s *Server) registerProjectRoutes() {
	s.mux.HandleFunc("GET /api/v1/workspaces/{workspace_id}/projects", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleListProjects)))
	s.mux.HandleFunc("POST /api/v1/workspaces/{workspace_id}/projects", s.wrapHandler(s.requirePerm(auth.PermPromptCreate)(s.handleCreateProject)))
	s.mux.HandleFunc("GET /api/v1/projects/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleGetProject)))
	s.mux.HandleFunc("PUT /api/v1/projects/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleUpdateProject)))
	s.mux.HandleFunc("DELETE /api/v1/projects/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptDelete)(s.handleDeleteProject)))
}

func (s *Server) registerCapabilityRoutes() {
	s.mux.HandleFunc("GET /api/v1/projects/{project_id}/capabilities", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleListCapabilities)))
	s.mux.HandleFunc("POST /api/v1/projects/{project_id}/capabilities", s.wrapHandler(s.requirePerm(auth.PermPromptCreate)(s.handleCreateCapability)))
	s.mux.HandleFunc("GET /api/v1/capabilities/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleGetCapability)))
	s.mux.HandleFunc("PUT /api/v1/capabilities/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleUpdateCapability)))
	s.mux.HandleFunc("DELETE /api/v1/capabilities/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptDelete)(s.handleDeleteCapability)))
}

func (s *Server) registerVersionRoutes() {
	s.mux.HandleFunc("GET /api/v1/capabilities/{capability_id}/versions", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleListVersions)))
	s.mux.HandleFunc("POST /api/v1/capabilities/{capability_id}/versions", s.wrapHandler(s.requirePerm(auth.PermPromptCreate)(s.handleCreateVersion)))
	s.mux.HandleFunc("GET /api/v1/versions/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleGetVersion)))
	s.mux.HandleFunc("GET /api/v1/capabilities/{capability_id}/versions/latest", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleGetLatestVersion)))
}

func (s *Server) registerExecutionRoutes() {
	s.mux.HandleFunc("GET /api/v1/versions/{version_id}/executions", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleListExecutions)))
	s.mux.HandleFunc("POST /api/v1/versions/{version_id}/executions", s.wrapHandler(s.requirePerm(auth.PermPromptCreate)(s.handleCreateExecution)))
	s.mux.HandleFunc("GET /api/v1/executions/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleGetExecution)))
}

// requirePerm returns middleware that requires a specific permission.
func (s *Server) requirePerm(perm auth.Permission) func(Func) Func {
	return func(fn Func) Func {
		return func(w http.ResponseWriter, r *http.Request) error {
			if s.requireAuth && s.authn != nil {
				user, err := s.authn.Authenticate(r)
				if err != nil {
					return &HTTPError{Status: http.StatusUnauthorized, Message: "unauthorized: " + err.Error()}
				}
				r = r.WithContext(auth.WithUserContext(r.Context(), user))
			}
			user, ok := auth.UserFromContext(r.Context())
			if !ok && s.requireAuth {
				return &HTTPError{Status: http.StatusUnauthorized, Message: "no user in context"}
			}
			if ok && !auth.HasPermission(user.Role, perm) {
				return &HTTPError{Status: http.StatusForbidden, Message: "insufficient permissions"}
			}
			return fn(w, r)
		}
	}
}

// wrapHandler wraps a Func into an http.HandlerFunc with error handling.
func (s *Server) wrapHandler(fn Func) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			s.logger.Error("handler error",
				"err", err,
				"method", r.Method,
				"path", r.URL.Path,
			)
			writeError(w, err)
		}
	}
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode json response", "err", err)
	}
}

// writeError writes a JSON error response, inferring the status code from
// known error types.
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	var httpErr *HTTPError
	switch {
	case errors.As(err, &httpErr):
		status = httpErr.Status
	case errors.Is(err, ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, ErrBadRequest):
		status = http.StatusBadRequest
	case errors.Is(err, ErrConflict):
		status = http.StatusConflict
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if encErr := json.NewEncoder(w).Encode(map[string]string{valError: err.Error()}); encErr != nil {
		slog.Error("failed to encode error json response", "err", encErr)
	}
}

// readJSON decodes the request body into target.
func readJSON(r *http.Request, target any) error {
	defer func() { _ = r.Body.Close() }()
	return json.NewDecoder(r.Body).Decode(target)
}

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
	if s.logger != nil {
		s.logger.Warn("audit queue full, entry dropped",
			"action", action, "resource", resource, "user_id", userID)
	}
}

// StartAuditWorkers launches the bounded worker pool. Call once at
// server startup. Cancel the context to stop the workers gracefully.
func (s *Server) StartAuditWorkers(ctx context.Context, n int) {
	if n < 1 {
		n = 2
	}
	s.auditQueue = make(chan *models.AuditEntry, 1024)
	for i := 0; i < n; i++ {
		s.auditWg.Add(1)
		// #nosec G118 -- ctx is the server-level context from StartAuditWorkers
		// (passed from main.go), not a request-scoped context. The caller
		// cancels it on shutdown to stop the workers.
		go s.auditWorker(ctx)
	}
}

// StopAuditWorkers closes the audit queue and waits for the
// workers to drain the entries that are already enqueued. The
// wait is bounded by ctx: if ctx is cancelled before the workers
// finish, the function returns ctx.Err() and the workers
// continue draining in the background (their context is the
// one passed to StartAuditWorkers, which main.go cancels on
// process shutdown).
//
// StopAuditWorkers is safe to call multiple times. Subsequent
// calls are no-ops.
//
// The reason this function exists: tests were seeing
// "failed to write audit entry err='sql: database is closed'"
// because the test teardown closed the SQLite handle before the
// audit workers had drained the queue. The fix is to drain the
// queue first, then close the DB.
func (s *Server) StopAuditWorkers(ctx context.Context) error {
	if s.auditQueue == nil {
		return nil
	}
	// Closing the channel signals "no more entries" to the workers,
	// which then exit their for-range loops cleanly.
	select {
	case <-s.auditQueue: // already closed
	default:
		close(s.auditQueue)
	}
	done := make(chan struct{})
	go func() {
		s.auditWg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
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
			if err := s.db.AppendAudit(context.Background(), entry); err != nil {
				if s.logger != nil {
					s.logger.Error("failed to write audit entry",
						"err", err, "entry_id", entry.ID, "action", entry.Action)
				}
			}
		}
	}
}

// httpRequestFromContext returns the *http.Request stored in the
// context by the request middleware, if any. Returns nil if there is
// none (e.g. background work).
func httpRequestFromContext(ctx context.Context) *http.Request {
	if r, ok := ctx.Value(httpRequestKey{}).(*http.Request); ok {
		return r
	}
	return nil
}

func (s *Server) handleLogsStream(w http.ResponseWriter, r *http.Request) error {
	if s.logHub == nil {
		return badRequest("log streaming not configured")
	}
	s.logHub.HandleSSE(w, r)
	return nil
}

func (s *Server) handleMetricsPrometheus(w http.ResponseWriter, r *http.Request) error {
	if s.collector == nil {
		return badRequest("metrics not configured")
	}
	s.collector.Handler().ServeHTTP(w, r)
	return nil
}

// Common API errors.
var (
	ErrNotFound   = errors.New("resource not found")
	ErrBadRequest = errors.New("bad request")
	ErrConflict   = errors.New("resource already exists")
)

// HTTPError represents an HTTP error with a specific status code.
type HTTPError struct {
	Status  int
	Message string
}

func (e *HTTPError) Error() string { return e.Message }

func badRequest(msg string) error { return &HTTPError{Status: http.StatusBadRequest, Message: msg} }
func notFound(msg string) error   { return &HTTPError{Status: http.StatusNotFound, Message: msg} }
func unauthorized() error {
	return &HTTPError{Status: http.StatusUnauthorized, Message: "authentication required"}
}
func forbidden(msg string) error { return &HTTPError{Status: http.StatusForbidden, Message: msg} }

// callerID returns the authenticated user's ID, or "api" if no user
// is in the request context. Used to populate CreatedBy fields
// without each handler re-implementing the lookup.
func callerID(r *http.Request) string {
	if u, ok := auth.UserFromContext(r.Context()); ok && u != nil && u.ID != "" {
		return u.ID
	}
	return auditDefaultUser
}

// --- Rate Limiting ---

// rateLimit wraps a Func with rate limiting.
func (s *Server) rateLimit(next Func) Func {
	return func(w http.ResponseWriter, r *http.Request) error {
		if s.rateLimiter != nil && !s.rateLimiter.Allow(r.RemoteAddr) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return nil
		}
		return next(w, r)
	}
}

// --- Auth Adapter ---

// storeAuthAdapter adapts store.Repository to auth.APIKeyStore by converting
// between store's models.APIKey and auth's APIKeyRecord.
type storeAuthAdapter struct {
	db store.Repository
}

func (a *storeAuthAdapter) GetAPIKeyByHash(ctx context.Context, keyHash string) (*auth.APIKeyRecord, error) {
	key, err := a.db.GetAPIKeyByHash(ctx, keyHash)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, nil
	}
	return &auth.APIKeyRecord{
		ID:        key.ID,
		UserID:    key.UserID,
		Role:      key.Role,
		KeyPrefix: key.KeyPrefix,
		ExpiresAt: key.ExpiresAt,
		Revoked:   key.Revoked,
	}, nil
}

func (a *storeAuthAdapter) UpdateAPIKeyLastUsed(ctx context.Context, id string) error {
	return a.db.UpdateAPIKeyLastUsed(ctx, id)
}

// --- Auth Audit Logger ---

// authAuditLogger adapts the server's audit method to auth.logger.
type authAuditLogger struct {
	server *Server
}

func (l *authAuditLogger) LogAuthFailure(ctx context.Context, keyPrefix, reason, remoteAddr string) {
	l.server.audit(ctx, "auth_failure", "api_key", map[string]any{
		"key_prefix":  keyPrefix,
		"reason":      reason,
		"remote_addr": remoteAddr,
	})
}
