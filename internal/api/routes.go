package api

import (
	"net/http"
	"os"

	"github.com/sachncs/promptsheon/internal/auth"
)

func (s *Server) routes() {
	s.registerHealthRoutes()
	s.registerSettingsRoutes()
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
	s.registerHarnessRoutes()
}

func (s *Server) registerHealthRoutes() {
	s.mux.HandleFunc("GET /health", s.wrapHandler(s.handleHealth))
	s.mux.HandleFunc("GET /ready", s.wrapHandler(s.handleReady))
	// API-HEALTH-1 / API-HEALTH-2: Kubernetes-style aliases.
	// K8s probes conventionally hit /livez and /readyz; the
	// legacy /health and /ready paths remain so existing
	// tooling (and the curl examples in docs/) keeps working.
	s.mux.HandleFunc("GET /livez", s.wrapHandler(s.handleHealth))
	s.mux.HandleFunc("GET /readyz", s.wrapHandler(s.handleReady))
	s.mux.HandleFunc("GET /api/v1/version", s.wrapHandler(s.handleVersion))
	s.mux.HandleFunc("POST /api/v1/workflows/run", s.wrapHandler(s.requirePerm(auth.PermPromptCreate)(s.handleRunWorkflow)))
	if s.collector != nil {
		// /metrics is the Prometheus scrape endpoint. It exposes
		// token and cost counters. Two protections:
		//   1. require PermAuditRead when auth is enabled.
		//   2. serve only on the optional METRICS_ADDR loopback
		//      listener (handled by main.go). When the daemon
		//      listens on a public address but METRICS_ADDR is
		//      unset, /metrics is bound to the public mux but is
		//      still protected by requirePerm.
		h := s.collector.Handler()
		s.mux.HandleFunc("GET /metrics", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(func(w http.ResponseWriter, r *http.Request) error {
			h.ServeHTTP(w, r)
			return nil
		})))
	}
}

func (s *Server) registerAuthRoutes() {
	createKey := s.handleCreateAPIKey
	listKeys := s.handleListAPIKeys
	revokeKey := s.handleRevokeAPIKey
	oauthLogin := s.handleOAuthLogin
	oauthCallback := s.handleOAuthCallback
	if s.rateLimiter != nil {
		createKey = s.rateLimit(createKey)
		listKeys = s.rateLimit(listKeys)
		revokeKey = s.rateLimit(revokeKey)
		// Throttle auth and bootstrap too: these are unauthenticated
		// routes that an attacker can spam to provoke DB queries,
		// upstream IdP lookups, or admin-key mint races.
		oauthLogin = s.rateLimit(oauthLogin)
		oauthCallback = s.rateLimit(oauthCallback)
	}
	s.mux.HandleFunc("POST /api/v1/apikeys", s.wrapHandler(createKey))
	s.mux.HandleFunc("GET /api/v1/apikeys", s.wrapHandler(listKeys))
	s.mux.HandleFunc("DELETE /api/v1/apikeys/{id}", s.wrapHandler(revokeKey))
	s.mux.HandleFunc("GET /api/v1/auth/{provider}/login", s.wrapHandler(oauthLogin))
	s.mux.HandleFunc("GET /api/v1/auth/{provider}/callback", s.wrapHandler(oauthCallback))
	// SEC-5b: /api/v1/setup is registered when either auth is off
	// (the documented first-caller-wins path) or the operator
	// explicitly set PROMPTSHEON_BOOTSTRAP_TOKEN (which gates the
	// authenticated bootstrap path). Without both, the route
	// returns 404 — the unauthenticated form is unreachable.
	if !s.requireAuth || os.Getenv("PROMPTSHEON_BOOTSTRAP_TOKEN") != "" {
		s.mux.HandleFunc("POST /api/v1/setup", s.wrapHandler(s.rateLimit(s.handleBootstrap)))
	}
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
	s.mux.HandleFunc("GET /api/v1/logs/stream", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleLogsStream)))
}

func (s *Server) registerTracingRoutes() {
	// The SQLite tracer was removed; /api/v1/traces routes are
	// gone and traces now flow through OTel export. The register
	// function remains so future OTel-based query paths can be
	// wired here.
}

func (s *Server) registerMetricsRoutes() {
	s.mux.HandleFunc("GET /api/v1/metrics/summary", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleMetricsSummary)))
	s.mux.HandleFunc("GET /api/v1/metrics/top-capabilities", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleTopCapabilities)))
	// /api/v1/metrics/dashboard dropped the TraceStats block since
	// there is no SQLite-backed trace read store.
	s.mux.HandleFunc("GET /api/v1/metrics/dashboard", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleMetricsSummary)))
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
	// DB-11b: HTTP surface for alert M2M link/unlink. Operators
	// can now wire a notification group to a rule via the API
	// rather than only via direct DB writes.
	s.mux.HandleFunc("POST /api/v1/alerts/rules/{rule_id}/groups/{group_id}", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleLinkAlertRuleGroup)))
	s.mux.HandleFunc("DELETE /api/v1/alerts/rules/{rule_id}/groups/{group_id}", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleUnlinkAlertRuleGroup)))
}

func (s *Server) registerWebhookRoutes() {
	// SEC-4b: webhook write endpoints require PermWebhookAdmin.
	// Holders of PermPromptUpdate can read webhooks (audit trail)
	// but cannot register new ones — registering a destination is
	// the only way to coax the daemon into dialing outbound URLs,
	// so the operation is locked behind a dedicated permission.
	s.mux.HandleFunc("GET /api/v1/webhooks", s.wrapHandler(s.requirePerm(auth.PermAuditRead)(s.handleListWebhooks)))
	s.mux.HandleFunc("POST /api/v1/webhooks", s.wrapHandler(s.requirePerm(auth.PermWebhookAdmin)(s.handleCreateWebhook)))
	s.mux.HandleFunc("DELETE /api/v1/webhooks/{id}", s.wrapHandler(s.requirePerm(auth.PermWebhookAdmin)(s.handleDeleteWebhook)))
}

func (s *Server) registerWorkspaceRoutes() {
	s.mux.HandleFunc("GET /api/v1/workspaces", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleListWorkspaces)))
	s.mux.HandleFunc("POST /api/v1/workspaces", s.wrapHandler(s.requirePerm(auth.PermPromptCreate)(s.handleCreateWorkspace)))
	s.mux.HandleFunc("GET /api/v1/workspaces/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptRead)(s.handleGetWorkspace)))
	s.mux.HandleFunc("PUT /api/v1/workspaces/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptUpdate)(s.handleUpdateWorkspace)))
	s.mux.HandleFunc("DELETE /api/v1/workspaces/{id}", s.wrapHandler(s.requirePerm(auth.PermPromptDelete)(s.handleDeleteWorkspace)))
	// Per-Workspace rollup: Budget / Quota consumption aggregated
	// at the current moment. The handler returns an empty summary
	// if no Aggregator is configured; production wiring sets
	// rollupAgg via WithWorkspaceRollups.
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
