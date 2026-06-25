// promptsheond is the Promptsheon API server daemon.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"promptsheon/internal/alerting"
	"promptsheon/internal/api"
	"promptsheon/internal/config"
	contextpkg "promptsheon/internal/context"
	"promptsheon/internal/guardrail"
	"promptsheon/internal/metrics"
	"promptsheon/internal/models"
	"promptsheon/internal/observability"
	"promptsheon/internal/ratelimit"
	"promptsheon/internal/snapshot"
	"promptsheon/internal/store"
	"promptsheon/internal/trace"
	"promptsheon/internal/vault"
	"promptsheon/internal/webhook"
	"promptsheon/internal/workflow"
	"promptsheon/internal/ws"
)

func main() {
	cfg := config.LoadConfig()

	// SECURITY: shell tool policy must be configured at startup, not
	// mutated at runtime. An empty allowlist disables the tool
	// regardless of the enabled flag.
	configureShellTool(&cfg)

	// rootCtx is cancelled on shutdown. All background goroutines
	// (retention, oauth janitor, alert monitor) hang off this context.
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	// Set up structured logging.
	var logLevel slog.Level
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

	// Open database.
	db, err := store.NewSQLite(cfg.DBPath)
	if err != nil {
		logger.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	// Initialize tracing and metrics.
	spans, err := trace.NewSQLite(db.DB())
	if err != nil {
		logger.Warn("tracing disabled", "err", err)
	}
	collector := metrics.NewCollector()

	// Initialize output snapshots.
	snapStore, err := snapshot.NewStore(db.DB())
	if err != nil {
		logger.Warn("snapshot store disabled", "err", err)
	}

	// Initialize retention manager for cleanup of old data.
	retentionPolicy := observability.LoadRetentionPolicyFromEnv()
	retention := observability.NewRetentionManager(db.DB(), retentionPolicy, logger)
	retention.Start(rootCtx)

	// Initialize webhooks. The dispatcher is configured with the
	// repository as its persistence backend so registered endpoints
	// survive a restart.
	webhookDispatcher := webhook.NewDispatcher(logger).
		WithEndpointStore(&webhookStoreAdapter{db: db})
	if err := webhookDispatcher.LoadFromStore(rootCtx); err != nil {
		logger.Warn("webhook: load endpoints from store failed", "err", err)
	}

	// Initialize vault for provider key encryption.
	var v *vault.Vault
	if vaultKey := os.Getenv("PROMPTSHEON_VAULT_KEY"); vaultKey != "" {
		var err error
		v, err = vault.New(vaultKey)
		if err != nil {
			logger.Warn("vault disabled", "err", err)
		} else {
			logger.Info("vault enabled for provider key encryption")
		}
	}

	// Initialize log hub for real-time streaming.
	logHub := ws.NewHub(logger)
	go logHub.Run()

	// Initialize context manager. The manager is stateless and
	// fetches contexts from the database on demand via Assemble, so
	// no warm-up step is required at startup.
	contextMgr := contextpkg.NewManager(db)

	// Initialize usage tracker for top-used resources.
	usageTracker := api.NewUsageTracker()

	// Initialize guardrail manager.
	guardrailManager := guardrail.NewManager(logger, collector)

	// Initialize alerting manager. NewManagerWithDB loads existing rules
	// from the database into the in-memory map; AddRule persists back.
	alertingManager := alerting.NewManagerWithDB(logger, collector, db)
	alertingManager.StartMonitoring(rootCtx, collector, 1*time.Minute)

	// Construct the rate limiter early so the API server can share
	// the same bucket map as the global middleware.
	limiter := ratelimit.NewLimiter(ratelimit.LoadConfigFromEnv())

	// Create API server.
	var opts []api.Option
	if cfg.Auth {
		opts = append(opts, api.WithAuth(db))
		logger.Info("authentication enabled")
	}
	if spans != nil {
		opts = append(opts, api.WithTracing(spans, collector))
	}
	if snapStore != nil {
		opts = append(opts, api.WithSnapshotStore(snapStore))
	}
	opts = append(opts, api.WithWebhooks(webhookDispatcher))
	if v != nil {
		opts = append(opts, api.WithVault(v))
	}
	opts = append(opts, api.WithLogHub(logHub))
	opts = append(opts, api.WithUsageTracker(usageTracker))
	opts = append(opts, api.WithGuardrailManager(guardrailManager))
	opts = append(opts, api.WithAlertingManager(alertingManager))
	opts = append(opts, api.WithContextManager(contextMgr))
	opts = append(opts, api.WithRateLimiter(limiter))
	srv := api.NewServer(db, logger, opts...)
	// Start the bounded audit worker pool. The pool drains the queue
	// until the root context is cancelled.
	srv.StartAuditWorkers(rootCtx, 2)

	// Set up HTTP server with middleware.
	handler := api.ChainHTTP(srv,
		api.Recovery(logger),
		api.MaxBytesReader(10<<20), // 10 MB max request body
		api.SecurityHeaders,
		limiter.Middleware,
		metrics.HTTPMiddleware(collector, spans, logger),
		api.Logging(logger),
		api.CORS(cfg.CORSOrigins),
	)

	// Server timeouts come from config so operators can tune them via
	// PROMPTSHEON_SERVER_*_TIMEOUT env vars. Defaults are validated to
	// be non-negative by LoadConfig.
	writeTimeout := time.Duration(cfg.WriteTimeout) * time.Second
	readTimeout := time.Duration(cfg.ReadTimeout) * time.Second
	readHeaderTimeout := time.Duration(cfg.ReadHeaderTimeout) * time.Second
	idleTimeout := time.Duration(cfg.IdleTimeout) * time.Second

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadTimeout:       readTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	// Start server in goroutine.
	go func() {
		logger.Info("starting server", "addr", cfg.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// Start the OAuth state janitor. Cancelled by rootCtx on shutdown.
	api.StartOAuthStateJanitor(rootCtx)

	// Wait for interrupt signal.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Cancel background goroutines (retention, oauth janitor, etc).
	rootCancel()

	// Graceful shutdown with timeout.
	ctx, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelShutdown()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", "err", err)
	}

	// Stop the rate limiter and OAuth janitor cleanly.
	limiter.Stop()
	api.StopOAuthStateJanitor()

	logger.Info("server exited")
}

// configureShellTool loads the shell tool policy from environment. The
// tool is disabled unless BOTH PROMPTSHEON_SHELL_ENABLED=true and
// PROMPTSHEON_SHELL_ALLOWLIST contains at least one command.
func configureShellTool(cfg *config.Config) {
	enabled := os.Getenv("PROMPTSHEON_SHELL_ENABLED") == "true"
	raw := os.Getenv("PROMPTSHEON_SHELL_ALLOWLIST")
	var allow []string
	if raw != "" {
		for _, c := range strings.Split(raw, ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				allow = append(allow, c)
			}
		}
	}
	if enabled && len(allow) == 0 {
		// Fail-closed: an empty allowlist with the tool "enabled" would
		// otherwise behave as disabled, which is confusing. Force the
		// enabled flag to match the actual policy.
		enabled = false
	}
	workflow.SetShellToolPolicy(enabled, allow)
}

// webhookStoreAdapter bridges store.Repository to webhook.EndpointStore.
type webhookStoreAdapter struct {
	db *store.SQLite
}

func (a *webhookStoreAdapter) SaveWebhookEndpoint(ctx context.Context, ep *webhook.Endpoint) error {
	events := make([]string, 0, len(ep.Events))
	for _, e := range ep.Events {
		events = append(events, string(e))
	}
	return a.db.SaveWebhookEndpoint(ctx, &models.WebhookEndpointRecord{
		ID:        ep.ID,
		URL:       ep.URL,
		Secret:    ep.Secret,
		Events:    events,
		Active:    ep.Active,
		CreatedAt: ep.CreatedAt,
	})
}

func (a *webhookStoreAdapter) DeleteWebhookEndpoint(ctx context.Context, id string) error {
	return a.db.DeleteWebhookEndpoint(ctx, id)
}

func (a *webhookStoreAdapter) ListWebhookEndpoints(ctx context.Context) ([]*webhook.Endpoint, error) {
	recs, err := a.db.ListWebhookEndpoints(ctx)
	if err != nil {
		return nil, err
	}
	eps := make([]*webhook.Endpoint, 0, len(recs))
	for _, r := range recs {
		evs := make([]webhook.EventType, 0, len(r.Events))
		for _, e := range r.Events {
			evs = append(evs, webhook.EventType(e))
		}
		eps = append(eps, &webhook.Endpoint{
			ID:        r.ID,
			URL:       r.URL,
			Secret:    r.Secret,
			Events:    evs,
			Active:    r.Active,
			CreatedAt: r.CreatedAt,
		})
	}
	return eps, nil
}
