// Package api implements the HTTP REST API for Promptsheon.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"promptsheon/internal/alerting"
	"promptsheon/internal/auth"
	contextpkg "promptsheon/internal/context"
	"promptsheon/internal/eval"
	"promptsheon/internal/guardrail"
	"promptsheon/internal/metrics"
	"promptsheon/internal/models"
	"promptsheon/internal/ratelimit"
	"promptsheon/internal/search"
	"promptsheon/internal/snapshot"
	"promptsheon/internal/store"
	"promptsheon/internal/trace"
	"promptsheon/internal/vault"
	"promptsheon/internal/webhook"
	"promptsheon/internal/ws"
)

// APIFunc is the handler signature that returns errors for centralized handling.
type APIFunc func(http.ResponseWriter, *http.Request) error

// Server holds dependencies and routes for the HTTP API.
type Server struct {
	mux              *http.ServeMux
	db               store.Repository
	logger           *slog.Logger
	cfg              *ServerConfig
	authn            *auth.Authenticator
	requireAuth      bool
	evalRunner       *eval.Runner
	spans            *trace.SQLite
	collector        *metrics.Collector
	snapshotStore    *snapshot.Store
	webhooks         *webhook.Dispatcher
	vault            *vault.Vault
	oauth            *auth.OAuthManager
	logHub           *ws.Hub
	usageTracker     *UsageTracker
	guardrailManager *guardrail.Manager
	alertingManager  *alerting.Manager
	rateLimiter      *ratelimit.Limiter
	contextManager   *contextpkg.Manager

	// auditQueue is a bounded channel feeding the audit worker pool.
	auditQueue chan *models.AuditEntry
	// auditDropped counts audit entries that were rejected because
	// the queue was full. Exposed for tests and metrics.
	auditDropped atomic.Int64

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

// WithEvalRunner attaches an eval runner to the server.
func WithEvalRunner(runner *eval.Runner) Option {
	return func(s *Server) {
		s.evalRunner = runner
	}
}

// WithTracing attaches a trace store and metrics collector to the server.
func WithTracing(spans *trace.SQLite, collector *metrics.Collector) Option {
	return func(s *Server) {
		s.spans = spans
		s.collector = collector
	}
}

// WithSnapshotStore attaches an output snapshot store.
func WithSnapshotStore(ss *snapshot.Store) Option {
	return func(s *Server) {
		s.snapshotStore = ss
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

// WithRateLimiter sets the rate limiter for the server.
func WithRateLimiter(l *ratelimit.Limiter) Option {
	return func(s *Server) {
		s.rateLimiter = l
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
		searchManager: search.NewManager(0),
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
	// M-1: warm the search index from the current prompt set.
	// This is best-effort: a failure here does not block server
	// startup (the index can be rebuilt lazily on the first
	// search request if the warm-up fails).
	if prompts, err := s.db.ListPrompts(context.Background(), models.PromptFilter{}); err == nil {
		s.searchManager.Rebuild(prompts)
	}
	return s
}

// ServeHTTP makes Server implement http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	// Health (always unauthenticated).
	s.mux.HandleFunc("GET /health", s.wrapHandler(s.handleHealth))
	s.mux.HandleFunc("GET /ready", s.wrapHandler(s.handleReady))

	// Prometheus metrics (always unauthenticated).
	if s.collector != nil {
		s.mux.Handle("GET /metrics", s.collector.Handler())
	}

	// Auth endpoints. Previously these were registered as fully
	// unauthenticated, which let any anonymous caller mint admin keys.
	// The handlers now perform their own caller checks; we still wrap
	// them with the global rate limiter to prevent brute force.
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

	// OAuth endpoints (unauthenticated — used for SSO login).
	s.mux.HandleFunc("GET /api/v1/auth/{provider}/login", s.wrapHandler(s.handleOAuthLogin))
	s.mux.HandleFunc("GET /api/v1/auth/{provider}/callback", s.wrapHandler(s.handleOAuthCallback))

	// Users (admin only)
	s.mux.HandleFunc("GET /api/v1/users", s.wrapHandler(s.requirePerm(auth.PermUserManage)(s.handleListUsers)))
	s.mux.HandleFunc("POST /api/v1/users", s.wrapHandler(s.requirePerm(auth.PermUserManage)(s.handleCreateUser)))
	s.mux.HandleFunc("GET /api/v1/users/{id}", s.wrapHandler(s.requirePerm(auth.PermUserManage)(s.handleGetUser)))
	s.mux.HandleFunc("PUT /api/v1/users/{id}", s.wrapHandler(s.requirePerm(auth.PermUserManage)(s.handleUpdateUser)))
	s.mux.HandleFunc("DELETE /api/v1/users/{id}", s.wrapHandler(s.requirePerm(auth.PermUserManage)(s.handleDeleteUser)))

	// Protected routes — apply auth + permission checks if auth is enabled.
	s.mux.HandleFunc("GET /api/v1/prompts", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleListPrompts)))
	s.mux.HandleFunc("POST /api/v1/prompts", s.wrapHandler(s.requirePerm(auth.PermPromptCreate)(s.handleCreatePrompt)))
	s.mux.HandleFunc("GET /api/v1/prompts/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleGetPrompt)))
	s.mux.HandleFunc("PUT /api/v1/prompts/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleUpdatePrompt)))
	s.mux.HandleFunc("DELETE /api/v1/prompts/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptDelete)(s.handleDeletePrompt)))
	s.mux.HandleFunc("GET /api/v1/prompts/{id}/versions", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleListPromptVersions)))
	s.mux.HandleFunc("POST /api/v1/prompts/{id}/restore", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleRestorePrompt)))
	s.mux.HandleFunc("GET /api/v1/prompts/similar", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleFindSimilarPrompts)))
	s.mux.HandleFunc("POST /api/v1/prompts/{id}/deploy", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleDeployPrompt)))
	s.mux.HandleFunc("POST /api/v1/prompts/{id}/archive", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleArchivePrompt)))
	s.mux.HandleFunc("POST /api/v1/prompts/{id}/run", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleRunPrompt)))
	s.mux.HandleFunc("POST /api/v1/prompts/{id}/stream", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleStreamPrompt)))
	s.mux.HandleFunc("POST /api/v1/prompts/{id}/preview", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handlePreviewPrompt)))
	s.mux.HandleFunc("GET /api/v1/prompts/{id}/diff", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handlePromptDiff)))
	s.mux.HandleFunc("POST /api/v1/prompts/{id}/clone", s.wrapHandler(s.requirePerm(auth.PermPromptCreate)(s.handleClonePrompt)))

	s.mux.HandleFunc("GET /api/v1/agents", s.wrapHandler(s.requirePerm(auth.PermAgentRead)(s.handleListAgents)))
	s.mux.HandleFunc("POST /api/v1/agents", s.wrapHandler(s.requirePerm(auth.PermAgentCreate)(s.handleCreateAgent)))
	s.mux.HandleFunc("GET /api/v1/agents/{id}", s.wrapHandler(s.requirePerm(auth.PermAgentRead)(s.handleGetAgent)))
	s.mux.HandleFunc("PUT /api/v1/agents/{id}", s.wrapHandler(s.requirePerm(auth.PermAgentUpdate)(s.handleUpdateAgent)))
	s.mux.HandleFunc("DELETE /api/v1/agents/{id}", s.wrapHandler(s.requirePerm(auth.PermAgentDelete)(s.handleDeleteAgent)))
	s.mux.HandleFunc("GET /api/v1/agents/{id}/export", s.wrapHandler(s.requirePerm(auth.PermAgentRead)(s.handleExportAgent)))
	s.mux.HandleFunc("POST /api/v1/agents/import-yaml", s.wrapHandler(s.requirePerm(auth.PermAgentCreate)(s.handleImportAgentYAML)))
	s.mux.HandleFunc("POST /api/v1/agents/{id}/fork", s.wrapHandler(s.requirePerm(auth.PermAgentCreate)(s.handleForkAgent)))
	s.mux.HandleFunc("GET /api/v1/agents/templates", s.wrapHandler(s.requirePerm(auth.PermAgentRead)(s.handleListTemplates)))
	s.mux.HandleFunc("GET /api/v1/agents/{id}/versions", s.wrapHandler(s.requirePerm(auth.PermAgentRead)(s.handleListAgentVersions)))
	s.mux.HandleFunc("POST /api/v1/agents/{id}/restore", s.wrapHandler(s.requirePerm(auth.PermAgentUpdate)(s.handleRestoreAgent)))
	s.mux.HandleFunc("POST /api/v1/agents/{id}/deploy", s.wrapHandler(s.requirePerm(auth.PermAgentUpdate)(s.handleDeployAgent)))
	s.mux.HandleFunc("POST /api/v1/agents/{id}/archive", s.wrapHandler(s.requirePerm(auth.PermAgentUpdate)(s.handleArchiveAgent)))
	s.mux.HandleFunc("POST /api/v1/agents/{id}/rerun", s.wrapHandler(s.requirePerm(auth.PermAgentCreate)(s.handleRerunAgent)))
	s.mux.HandleFunc("POST /api/v1/agents/validate", s.wrapHandler(s.requirePerm(auth.PermAgentRead)(s.handleValidateAgentWorkflow)))

	// Agent Execute (full orchestration)
	s.mux.HandleFunc("POST /api/v1/agents/{id}/execute", s.wrapHandler(s.requirePerm(auth.PermAgentCreate)(s.handleExecuteAgent)))

	// Agent Guardrail Configs
	s.mux.HandleFunc("POST /api/v1/agents/{id}/guardrail-config", s.wrapHandler(s.requirePerm(auth.PermAgentUpdate)(s.handleCreateAgentGuardrailConfig)))
	s.mux.HandleFunc("GET /api/v1/agents/{id}/guardrail-config", s.wrapHandler(s.requirePerm(auth.PermAgentRead)(s.handleGetAgentGuardrailConfig)))
	s.mux.HandleFunc("PUT /api/v1/agents/{id}/guardrail-config/{config_id}", s.wrapHandler(s.requirePerm(auth.PermAgentUpdate)(s.handleUpdateAgentGuardrailConfig)))
	s.mux.HandleFunc("DELETE /api/v1/agents/{id}/guardrail-config/{config_id}", s.wrapHandler(s.requirePerm(auth.PermAgentUpdate)(s.handleDeleteAgentGuardrailConfig)))

	// Agent Executions
	s.mux.HandleFunc("GET /api/v1/agents/{id}/executions", s.wrapHandler(s.requirePerm(auth.PermAgentRead)(s.handleListAgentExecutions)))
	s.mux.HandleFunc("GET /api/v1/agents/{id}/executions/{exec_id}", s.wrapHandler(s.requirePerm(auth.PermAgentRead)(s.handleGetAgentExecution)))

	// Execution Logs
	s.mux.HandleFunc("GET /api/v1/execution-logs", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleListExecutionLogs)))
	s.mux.HandleFunc("GET /api/v1/execution-logs/{id}", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleGetExecutionLog)))

	// Contexts
	s.mux.HandleFunc("POST /api/v1/contexts", s.wrapHandler(s.requirePerm(auth.PermPromptCreate)(s.handleCreateContext)))
	s.mux.HandleFunc("GET /api/v1/contexts", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleListContexts)))
	s.mux.HandleFunc("GET /api/v1/contexts/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleGetContext)))
	s.mux.HandleFunc("PUT /api/v1/contexts/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleUpdateContext)))
	s.mux.HandleFunc("DELETE /api/v1/contexts/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptDelete)(s.handleDeleteContext)))
	s.mux.HandleFunc("POST /api/v1/contexts/{id}/messages", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleAppendContextMessage)))
	s.mux.HandleFunc("DELETE /api/v1/contexts/{id}/messages", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleClearContextMessages)))
	s.mux.HandleFunc("POST /api/v1/contexts/{id}/assemble", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleAssembleContext)))

	s.mux.HandleFunc("GET /api/v1/datasets", s.wrapHandler(s.requirePerm(auth.PermDatasetRead)(s.handleListDatasets)))
	s.mux.HandleFunc("POST /api/v1/datasets", s.wrapHandler(s.requirePerm(auth.PermDatasetCreate)(s.handleCreateDataset)))
	s.mux.HandleFunc("GET /api/v1/datasets/{id}", s.wrapHandler(s.requirePerm(auth.PermDatasetRead)(s.handleGetDataset)))
	s.mux.HandleFunc("PUT /api/v1/datasets/{id}", s.wrapHandler(s.requirePerm(auth.PermDatasetUpdate)(s.handleUpdateDataset)))
	s.mux.HandleFunc("DELETE /api/v1/datasets/{id}", s.wrapHandler(s.requirePerm(auth.PermDatasetDelete)(s.handleDeleteDataset)))
	s.mux.HandleFunc("POST /api/v1/datasets/import", s.wrapHandler(s.requirePerm(auth.PermDatasetCreate)(s.handleImportDataset)))
	s.mux.HandleFunc("POST /api/v1/datasets/{id}/import-csv", s.wrapHandler(s.requirePerm(auth.PermDatasetCreate)(s.handleImportCSVDataset)))
	s.mux.HandleFunc("GET /api/v1/datasets/{id}/export", s.wrapHandler(s.requirePerm(auth.PermDatasetRead)(s.handleExportDataset)))

	s.mux.HandleFunc("GET /api/v1/reviews", s.wrapHandler(s.requirePerm(auth.PermReviewCreate)(s.handleListPendingReviews)))
	s.mux.HandleFunc("POST /api/v1/reviews", s.wrapHandler(s.requirePerm(auth.PermReviewCreate)(s.handleCreateReview)))
	s.mux.HandleFunc("PUT /api/v1/reviews/{id}/approve", s.wrapHandler(s.requirePerm(auth.PermReviewApprove)(s.handleApproveReview)))
	s.mux.HandleFunc("PUT /api/v1/reviews/{id}/reject", s.wrapHandler(s.requirePerm(auth.PermReviewApprove)(s.handleRejectReview)))
	s.mux.HandleFunc("POST /api/v1/reviews/{id}/comment", s.wrapHandler(s.requirePerm(auth.PermReviewCreate)(s.handleAddComment)))

	s.mux.HandleFunc("GET /api/v1/audit", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleListAudit)))
	s.mux.HandleFunc("GET /api/v1/audit/export", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleExportAudit)))
	s.mux.HandleFunc("GET /api/v1/audit/verify", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleVerifyAuditChain)))

	// Evaluations
	s.mux.HandleFunc("POST /api/v1/eval/run", s.wrapHandler(s.requirePerm(auth.PermEvalRun)(s.handleRunEval)))
	s.mux.HandleFunc("GET /api/v1/eval/results", s.wrapHandler(s.requirePerm(auth.PermEvalRead)(s.handleListEvalResults)))
	s.mux.HandleFunc("GET /api/v1/eval/report", s.wrapHandler(s.requirePerm(auth.PermEvalRead)(s.handleGetEvalReport)))
	s.mux.HandleFunc("GET /api/v1/eval/compare", s.wrapHandler(s.requirePerm(auth.PermEvalRead)(s.handleCompareEval)))
	s.mux.HandleFunc("GET /api/v1/eval/runs", s.wrapHandler(s.requirePerm(auth.PermEvalRead)(s.handleListEvalRuns)))

	// Workflows
	s.mux.HandleFunc("POST /api/v1/workflows/run", s.wrapHandler(s.requirePerm(auth.PermAgentCreate)(s.handleRunWorkflow)))
	s.mux.HandleFunc("GET /api/v1/workflows", s.wrapHandler(s.requirePerm(auth.PermAgentRead)(s.handleListWorkflows)))
	s.mux.HandleFunc("GET /api/v1/workflows/{id}", s.wrapHandler(s.requirePerm(auth.PermAgentRead)(s.handleGetWorkflow)))
	s.mux.HandleFunc("GET /api/v1/workflows/{id}/steps", s.wrapHandler(s.requirePerm(auth.PermAgentRead)(s.handleGetWorkflowSteps)))
	s.mux.HandleFunc("PUT /api/v1/workflows/{id}/cancel", s.wrapHandler(s.requirePerm(auth.PermAgentUpdate)(s.handleCancelWorkflow)))

	// Tracing
	s.mux.HandleFunc("GET /api/v1/traces", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleListSpans)))
	s.mux.HandleFunc("GET /api/v1/traces/{id}", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleGetSpan)))
	s.mux.HandleFunc("GET /api/v1/traces/tree/{trace_id}", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleGetTraceTree)))

	// Metrics
	s.mux.HandleFunc("GET /api/v1/metrics/summary", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleMetricsSummary)))
	s.mux.HandleFunc("GET /api/v1/metrics/top-prompts", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleTopPrompts)))
	s.mux.HandleFunc("GET /api/v1/metrics/top-agents", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleTopAgents)))
	s.mux.HandleFunc("GET /api/v1/metrics/dashboard", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleDashboardSummary)))

	// Searchable logs
	s.mux.HandleFunc("GET /api/v1/logs/search", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleSearchSpans)))

	// Snapshots
	s.mux.HandleFunc("GET /api/v1/snapshots", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleListSnapshots)))
	s.mux.HandleFunc("GET /api/v1/snapshots/{id}", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleGetSnapshot)))

	// Providers
	s.mux.HandleFunc("GET /api/v1/providers", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleListProviders)))
	s.mux.HandleFunc("GET /api/v1/providers/{name}", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleGetProvider)))
	s.mux.HandleFunc("POST /api/v1/providers/{name}/test", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleTestProvider)))

	// Vault (provider key management)
	s.mux.HandleFunc("POST /api/v1/vault/keys", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleSaveVaultKey)))
	s.mux.HandleFunc("GET /api/v1/vault/keys", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleListVaultKeys)))
	s.mux.HandleFunc("DELETE /api/v1/vault/keys/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleDeleteVaultKey)))

	// Real-time logs (SSE)
	s.mux.HandleFunc("GET /api/v1/logs/stream", s.wrapHandler(s.handleLogsStream))

	// Guardrails (specific routes first)
	s.mux.HandleFunc("POST /api/v1/guardrails/check", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleCheckGuardrails)))
	s.mux.HandleFunc("GET /api/v1/guardrails/violations", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleListGuardrailViolations)))
	s.mux.HandleFunc("PUT /api/v1/guardrails/violations/{id}/resolve", s.wrapHandler(s.requirePerm(auth.PermReviewApprove)(s.handleResolveGuardrailViolation)))
	s.mux.HandleFunc("GET /api/v1/guardrails/rules", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleListGuardrailRules)))
	s.mux.HandleFunc("POST /api/v1/guardrails/rules", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleCreateGuardrailRule)))
	s.mux.HandleFunc("GET /api/v1/guardrails/rules/{id}", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleGetGuardrailRule)))
	s.mux.HandleFunc("PUT /api/v1/guardrails/rules/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleUpdateGuardrailRule)))
	s.mux.HandleFunc("DELETE /api/v1/guardrails/rules/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleDeleteGuardrailRule)))

	// Alerting (specific routes first to avoid pattern conflicts)
	s.mux.HandleFunc("GET /api/v1/alerts/rules", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleListAlertRules)))
	s.mux.HandleFunc("POST /api/v1/alerts/rules", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleCreateAlertRule)))
	s.mux.HandleFunc("GET /api/v1/alerts/rules/{id}", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleGetAlertRule)))
	s.mux.HandleFunc("PUT /api/v1/alerts/rules/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleUpdateAlertRule)))
	s.mux.HandleFunc("DELETE /api/v1/alerts/rules/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleDeleteAlertRule)))
	s.mux.HandleFunc("POST /api/v1/alerts/notifications", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleAddNotificationGroup)))
	s.mux.HandleFunc("GET /api/v1/alerts/active", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleListAlerts)))
	s.mux.HandleFunc("PUT /api/v1/alerts/active/{id}/resolve", s.wrapHandler(s.requirePerm(auth.PermReviewApprove)(s.handleResolveAlert)))

	// Webhooks
	s.mux.HandleFunc("GET /api/v1/webhooks", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleListWebhooks)))
	s.mux.HandleFunc("POST /api/v1/webhooks", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleCreateWebhook)))
	s.mux.HandleFunc("DELETE /api/v1/webhooks/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleDeleteWebhook)))

	// Metrics (Prometheus format, authenticated)
	s.mux.HandleFunc("GET /api/v1/metrics", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleMetricsPrometheus)))

	// Prompt Optimization
	s.mux.HandleFunc("POST /api/v1/prompts/{id}/optimize", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleOptimizePrompt)))
	s.mux.HandleFunc("GET /api/v1/prompts/{id}/analyze", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleAnalyzePrompt)))
	s.mux.HandleFunc("GET /api/v1/optimization/tips", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleGetOptimizationTips)))

	// Prompt Playground
	s.mux.HandleFunc("POST /api/v1/playground/run", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handlePlaygroundRun)))
	s.mux.HandleFunc("POST /api/v1/playground/compare", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handlePlaygroundCompare)))
	s.mux.HandleFunc("GET /api/v1/playground/templates", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handlePlaygroundTemplates)))

	// A/B Testing
	s.mux.HandleFunc("POST /api/v1/ab-tests", s.wrapHandler(s.requirePerm(auth.PermPromptCreate)(s.handleCreateABTest)))
	s.mux.HandleFunc("GET /api/v1/ab-tests", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleListABTests)))
	s.mux.HandleFunc("GET /api/v1/ab-tests/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleGetABTest)))
	s.mux.HandleFunc("POST /api/v1/ab-tests/{id}/stop", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleStopABTest)))
	s.mux.HandleFunc("GET /api/v1/ab-tests/{id}/results", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleGetABTestResults)))

	// Semantic Search
	s.mux.HandleFunc("POST /api/v1/search/semantic", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleSemanticSearch)))
	s.mux.HandleFunc("POST /api/v1/search/index", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleIndexPrompt)))
	s.mux.HandleFunc("GET /api/v1/search/similar/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleFindSimilar)))

	// Real-time Collaboration
	s.mux.HandleFunc("POST /api/v1/collab/sessions", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleCreateCollabSession)))
	s.mux.HandleFunc("GET /api/v1/collab/sessions/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleGetCollabSession)))
	s.mux.HandleFunc("PUT /api/v1/collab/sessions/{id}/cursor", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleUpdateCursor)))
	s.mux.HandleFunc("GET /api/v1/collab/sessions/{id}/changes", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleGetChanges)))
}

// requirePerm returns middleware that requires a specific permission.
func (s *Server) requirePerm(perm auth.Permission) func(APIFunc) APIFunc {
	return func(fn APIFunc) APIFunc {
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

// wrapHandler wraps an APIFunc into an http.HandlerFunc with error handling.
func (s *Server) wrapHandler(fn APIFunc) http.HandlerFunc {
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
	if errors.As(err, &httpErr) {
		status = httpErr.Status
	} else if errors.Is(err, ErrNotFound) {
		status = http.StatusNotFound
	} else if errors.Is(err, ErrBadRequest) {
		status = http.StatusBadRequest
	} else if errors.Is(err, ErrConflict) {
		status = http.StatusConflict
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if encErr := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}); encErr != nil {
		slog.Error("failed to encode error json response", "err", encErr)
	}
}

// readJSON decodes the request body into target.
func readJSON(r *http.Request, target any) error {
	defer r.Body.Close()
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
	userID := "api"
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
		go s.auditWorker(ctx)
	}
}

func (s *Server) auditWorker(ctx context.Context) {
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

// refreshSearchIndex updates the in-memory search index with the
// latest version of a single prompt. Called from the prompt
// create/update/delete handlers. M-1: without this hook, the
// index would drift out of sync with the database.
func (s *Server) refreshSearchIndex(ctx context.Context, op string, p *models.Prompt) {
	if s.searchManager == nil {
		return
	}
	switch op {
	case "remove":
		if p != nil {
			s.searchManager.Remove(p.ID)
		}
	case "upsert":
		if p == nil {
			return
		}
		// Fetch the latest version from the DB so the index always
		// reflects the persisted state, not the request body
		// (which may not have all fields populated).
		fresh, err := s.db.GetPrompt(ctx, p.ID)
		if err != nil {
			s.searchManager.Remove(p.ID)
			return
		}
		s.searchManager.Add(fresh)
	}
}

func (s *Server) auditDiff(ctx context.Context, action, resource string, prev, new any) {
	details := make(map[string]any)
	if prev != nil {
		details["previous"] = prev
	}
	if new != nil {
		details["new"] = new
	}
	s.audit(ctx, action, resource, details)
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
func badRequestf(format string, args ...any) error {
	return &HTTPError{Status: http.StatusBadRequest, Message: fmt.Sprintf(format, args...)}
}
func notFound(msg string) error     { return &HTTPError{Status: http.StatusNotFound, Message: msg} }
func unauthorized(msg string) error { return &HTTPError{Status: http.StatusUnauthorized, Message: msg} }
func forbidden(msg string) error    { return &HTTPError{Status: http.StatusForbidden, Message: msg} }
func serverError(msg string) error {
	return &HTTPError{Status: http.StatusInternalServerError, Message: msg}
}

// callerID returns the authenticated user's ID, or "api" if no user
// is in the request context. Used to populate CreatedBy fields
// without each handler re-implementing the lookup.
func callerID(r *http.Request) string {
	if u, ok := auth.UserFromContext(r.Context()); ok && u != nil && u.ID != "" {
		return u.ID
	}
	return "api"
}

// --- Rate Limiting ---

// rateLimit wraps an APIFunc with rate limiting.
func (s *Server) rateLimit(next APIFunc) APIFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if s.rateLimiter != nil && !s.rateLimiter.Allow(r.RemoteAddr) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limit exceeded"}`)) //nolint:errcheck
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

// authAuditLogger adapts the server's audit method to auth.AuthLogger.
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
