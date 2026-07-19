// promptsheond is the Promptsheon API server daemon.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sachncs/promptsheon/internal/alerting"
	"github.com/sachncs/promptsheon/internal/api"
	"github.com/sachncs/promptsheon/internal/buildinfo"
	"github.com/sachncs/promptsheon/internal/config"
	contextpkg "github.com/sachncs/promptsheon/internal/context"
	"github.com/sachncs/promptsheon/internal/eventbus"
	"github.com/sachncs/promptsheon/internal/executor"
	"github.com/sachncs/promptsheon/internal/guardrail"
	"github.com/sachncs/promptsheon/internal/harness"
	"github.com/sachncs/promptsheon/internal/invoke"
	"github.com/sachncs/promptsheon/internal/llm"
	"github.com/sachncs/promptsheon/internal/metrics"
	"github.com/sachncs/promptsheon/internal/models"
	"github.com/sachncs/promptsheon/internal/observability"
	"github.com/sachncs/promptsheon/internal/observation"
	"github.com/sachncs/promptsheon/internal/plugins/builtins"
	"github.com/sachncs/promptsheon/internal/ratelimit"
	"github.com/sachncs/promptsheon/internal/release"
	"github.com/sachncs/promptsheon/internal/rollups"
	"github.com/sachncs/promptsheon/internal/scheduler"
	"github.com/sachncs/promptsheon/internal/store"
	"github.com/sachncs/promptsheon/internal/supervisor"
	"github.com/sachncs/promptsheon/internal/trace"
	"github.com/sachncs/promptsheon/internal/vault"
	"github.com/sachncs/promptsheon/internal/webhook"
	"github.com/sachncs/promptsheon/internal/workflow"
	"github.com/sachncs/promptsheon/internal/ws"
)

const logLevelDebug = "debug"
const logLevelInfo = "info"
const logLevelWarn = "warn"
const logLevelError = "error"

func main() {
	// Handle --version and --help before loading the rest of the
	// config. Operators commonly run 'promptsheond --version' to
	// confirm a deployment, and we don't want a missing or invalid
	// env var to mask the simple cases.
	showVersion := flag.Bool("version", false, "print version information and exit")
	showHelp := flag.Bool("help", false, "print configuration and runtime flags and exit")
	flag.Parse()

	if *showVersion {
		info := buildinfo.Get()
		fmt.Printf("promptsheond %s (commit %s, built %s, %s/%s)\n",
			info.Version, info.Commit, info.BuildTime, info.OS, info.Arch)
		return
	}
	if *showHelp {
		fmt.Print(serverHelpText())
		return
	}

	cfg := config.LoadConfig()

	// Fail loudly on unsafe configurations. The most common case is
	// PROMPTSHEON_AUTH=false on a non-loopback bind — the bootstrap
	// endpoint would mint an admin key to the first caller. Refuse
	// to start instead of warning and continuing.
	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	// SECURITY: shell tool policy must be configured at startup, not
	// mutated at runtime. An empty allowlist disables the tool
	// regardless of the enabled flag.
	configureShellTool(&cfg)

	// rootCtx is cancelled on shutdown. All background goroutines
	// (retention, oauth janitor, alert monitor) hang off this context.
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	logger := setupLogger(&cfg)

	db := openDB(&cfg, logger)
	defer func() {
		if db != nil {
			_ = db.Close()
		}
	}()

	// Wire the plugin supervisor: register every built-in Guardrail
	// plugin, set the publisher to nil (production wiring adds an
	// EventBus adapter in a follow-on), and start the supervisor in
	// a goroutine that observes rootCtx for shutdown. The supervisor
	// owns plugin lifecycle; the daemon owns the supervisor.
	sup := supervisor.New(nil, logger)
	builtins.Register(sup)
	go func() {
		if err := sup.Run(rootCtx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Warn("plugin supervisor exited with error", "err", err)
		}
	}()

	// Schedule ticker: poll the schedules table for due rows and
	// publish schedule.fired events. The scheduler runs alongside
	// the supervisor and exits cleanly when rootCtx is cancelled.
	sched := scheduler.New(db, eventbus.NewMemory(), 5*time.Second)
	go func() {
		if err := sched.Start(rootCtx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Warn("scheduler exited with error", "err", err)
		}
	}()

	srv, limiter, spans, collector := buildServer(rootCtx, &cfg, db, logger)

	srv.StartAuditWorkers(rootCtx, 2)

	startHTTPServerAndWait(rootCtx, rootCancel, &cfg, srv, logger, limiter, spans, collector)
}

// configureShellTool loads the shell tool policy from environment. The
// tool is disabled unless BOTH PROMPTSHEON_SHELL_ENABLED=true and
// PROMPTSHEON_SHELL_ALLOWLIST contains at least one command.
func configureShellTool(_ *config.Config) {
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

func setupLogger(cfg *config.Config) *slog.Logger {
	var logLevel slog.Level
	switch cfg.LogLevel {
	case logLevelDebug:
		logLevel = slog.LevelDebug
	case logLevelInfo:
		logLevel = slog.LevelInfo
	case logLevelWarn:
		logLevel = slog.LevelWarn
	case logLevelError:
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))
}

func openDB(cfg *config.Config, logger *slog.Logger) *store.SQLite {
	db, err := store.NewSQLite(cfg.DBPath)
	if err != nil {
		logger.Error("failed to open database", "err", err)
		if db != nil {
			_ = db.Close()
		}
		os.Exit(1)
	}
	return db
}

func buildServer(rootCtx context.Context, cfg *config.Config, db *store.SQLite, logger *slog.Logger) (*api.Server, *ratelimit.Limiter, *trace.SQLite, *metrics.Collector) {
	spans, err := trace.NewSQLite(db.DB())
	if err != nil {
		logger.Warn("tracing disabled", "err", err)
	}
	collector := metrics.NewCollector()

	if cfg.OTelEndpoint != "" {
		tp, terr := trace.InitTracerProvider("promptsheond", cfg.OTelEndpoint, cfg.OTelInsecure)
		if terr != nil {
			logger.Warn("OTel tracer init failed", "endpoint", cfg.OTelEndpoint, "err", terr)
		} else {
			logger.Info("OTel tracer initialised", "endpoint", cfg.OTelEndpoint, "insecure", cfg.OTelInsecure)
			defer func() {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if e := tp.Shutdown(shutdownCtx); e != nil {
					logger.Warn("OTel tracer shutdown failed", "err", e)
				}
			}()
		}
	}

	retentionPolicy := observability.LoadRetentionPolicyFromEnv()
	retention := observability.NewRetentionManager(db.DB(), retentionPolicy, logger)
	retention.Start(rootCtx)

	webhookDispatcher := webhook.NewDispatcher(logger).
		WithEndpointStore(&webhookStoreAdapter{db: db})
	if err := webhookDispatcher.LoadFromStore(rootCtx); err != nil {
		logger.Warn("webhook: load endpoints from store failed", "err", err)
	}

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

	logHub := ws.NewHub(logger)
	go logHub.Run()

	contextMgr := contextpkg.NewManager()
	usageTracker := api.NewUsageTracker()
	guardrailManager := guardrail.NewManager(logger, collector)
	alertingManager := alerting.NewManagerWithDB(logger, collector, db)
	alertingManager.StartMonitoring(rootCtx, collector, 1*time.Minute)

	limiter := ratelimit.NewLimiter(ratelimit.LoadConfigFromEnv())

	providers := llm.NewRegistry()
	providers.LoadFromEnv()

	// Per-Workspace rollup aggregator (Tier 2.37). The production
	// wiring supplies a backend-backed Budget/Quota repository; the
	// rollup job that writes to ClickHouse is M3 follow-on. Today's
	// wiring attaches a nil-safe aggregator that the route handles
	// gracefully when unset (returns an empty summary).
	rollupAgg := rollups.New(nil, nil)

	// Canonical invoke.Invoker (Tier 2.36). The Caller resolves
	// the model + provider from the request, then routes through
	// the LLM registry so OpenAI / Anthropic / Ollama / etc. all
	// go through the same path. When no provider is configured
	// the Caller returns an explicit error rather than a fake
	// success — the route surfaces it as 502 Bad Gateway.
	enforcer := invoke.NewDefaultEnforcer(nil)
	agg := observation.NewAggregator(nil)
	inv := invoke.New(enforcer, agg, executor.New(nil, func(ctx context.Context, req executor.InvokeRequest) (executor.InvokeResult, error) {
		if req.Model == "" || req.Model == "<unspecified>" {
			return executor.InvokeResult{Status: "error", Error: "no model configured"}, nil
		}
		p, err := providers.Get(req.Model)
		if err != nil {
			// Fall back to the first registered provider when
			// the model name is unrecognized; production wiring
			// fills the canonical name from the active Release.
			names := providers.Providers()
			if len(names) == 0 {
				return executor.InvokeResult{Status: "error", Error: "no provider registered"}, nil
			}
			p, err = providers.Get(names[0])
			if err != nil {
				return executor.InvokeResult{Status: "error", Error: err.Error()}, nil
			}
		}
		llmReq := &llm.Request{
			Messages: []llm.Message{{Role: "user", Content: string(req.Input)}},
			Model:    req.Model,
		}
		resp, err := p.Complete(ctx, llmReq)
		if err != nil {
			return executor.InvokeResult{Status: "error", Error: err.Error()}, nil
		}
		usage := resp.Usage
		costUSD := llm.EstimateCost(usage.PromptTokens, usage.CompletionTokens, resp.Model)
		return executor.InvokeResult{
			Output:       []byte(resp.Content),
			PromptTokens: usage.PromptTokens,
			OutputTokens: usage.CompletionTokens,
			CostUSDMicro: int64(costUSD * 1e6),
			Status:       "ok",
		}, nil
	}))

	var opts []api.Option
	if cfg.Auth {
		opts = append(opts, api.WithAuth(db))
		logger.Info("authentication enabled")
	}
	if spans != nil {
		opts = append(opts, api.WithTracing(spans, collector))
	}
	opts = append(opts, api.WithWebhooks(webhookDispatcher))
	if v != nil {
		opts = append(opts, api.WithVault(v))
	}
	opts = append(opts,
		api.WithLogHub(logHub),
		api.WithUsageTracker(usageTracker),
		api.WithGuardrailManager(guardrailManager),
		api.WithAlertingManager(alertingManager),
		api.WithContextManager(contextMgr),
		api.WithRateLimiter(limiter),
		api.WithProviders(providers),
		api.WithWorkspaceRollups(rollupAgg),
		api.WithInvoker(inv),
	)

	// releaseSvc is the application layer for the Release + Approval
	// endpoints. Default policy is MakerCheckerPolicy{RequiredApprovers: 1}:
	// the creator cannot approve their own release, and at least one
	// other identity must approve before activation. Override with
	// PROMPTSHEON_APPROVAL_POLICY=majority for a flat majority count.
	releaseSvc := buildReleaseService(db, cfg.ApprovalPolicy)
	if releaseSvc != nil {
		opts = append(opts, api.WithReleaseService(releaseSvc))
	}

	// Harness engineering surface (datasets, preconditions, evals).
	// When a ReleaseInvoker is available (i.e. an LLM provider is
	// configured), we wire the EvalRunner into the daemon. The
	// PreconditionRunner gates Activate; failures surface as 409.
	if releaseSvc != nil {
		precondRunner := harness.NewPreconditionRunner()
		releaseSvc.WithHarness(precondRunner, db)
		evalRunner := harness.NewEvalRunner(db, &apiReleaseInvoker{db: db, inv: inv, svc: releaseSvc})
		opts = append(opts, api.WithHarnessRunner(evalRunner))
	}

	srv := api.NewServer(db, logger, opts...)
	return srv, limiter, spans, collector
}

// buildReleaseService constructs the application-layer release.Service
// using the configured policy. Returns nil if the policy string is
// unrecognized; the daemon continues to run with release routes
// unregistered (404) rather than failing the boot.
func buildReleaseService(db *store.SQLite, policy string) *release.Service {
	switch policy {
	case "majority":
		return release.NewServiceFromKind(db, db, release.PolicyMajority, 1)
	case "", "maker_checker":
		return release.NewServiceFromKind(db, db, release.PolicyMakerChecker, 1)
	default:
		// unknown policy: log nothing here; the daemon's job is to
		// keep running. Operators see the misconfiguration when they
		// hit /activate.
		return nil
	}
}

func startHTTPServerAndWait(rootCtx context.Context, rootCancel func(), cfg *config.Config, srv *api.Server, logger *slog.Logger, limiter *ratelimit.Limiter, spans *trace.SQLite, collector *metrics.Collector) {
	handler := api.ChainHTTP(srv,
		api.Recovery(logger),
		api.MaxBytesReader(10<<20),
		api.SecurityHeaders,
		limiter.Middleware,
		metrics.HTTPMiddleware(collector, spans, logger),
		api.Logging(logger),
		api.CORS(cfg.CORSOrigins...),
	)

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

	go func() {
		info := buildinfo.Get()
		logger.Info("starting server",
			"version", info.Version,
			"commit", info.Commit,
			"addr", cfg.Addr,
			"db_path", cfg.DBPath,
			"auth", cfg.Auth,
			"otel_endpoint", cfg.OTelEndpoint,
		)
		if !cfg.Auth {
			logger.Warn("authentication is DISABLED; POST /api/v1/setup will mint an admin key to the first caller. Set PROMPTSHEON_AUTH=true before exposing this server.",
				"setup_endpoint", "POST /api/v1/setup")
		}
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	api.StartOAuthStateJanitor(rootCtx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	rootCancel()

	ctx, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelShutdown()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", "err", err)
	}

	auditStopCtx, cancelAuditStop := context.WithTimeout(context.Background(), 10*time.Second)
	if err := srv.StopAuditWorkers(auditStopCtx); err != nil {
		logger.Warn("audit workers did not drain in time", "err", err)
	}
	cancelAuditStop()

	limiter.Stop()
	api.StopOAuthStateJanitor()
	logger.Info("server exited")
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

// serverHelpText returns the human-readable usage block printed
// by --help. We keep the text in a function so it can be
// exercised by a test (e.g. a future doc-snippet test) without
// having to actually exec the binary.
func serverHelpText() string {
	return `promptsheond — Promptsheon API server

Usage:
  promptsheond [flags]

Flags:
  --version           print version information and exit
  --help              print this help text and exit

Configuration is read entirely from environment variables. The
most common variables are:

  PROMPTSHEON_ADDR           listen address (default ":8080")
  PROMPTSHEON_DB_PATH        SQLite database path (default "promptsheon.db")
  PROMPTSHEON_AUTH           enable authentication (default true)
  PROMPTSHEON_LOG_LEVEL      debug | info | warn | error (default info)
  PROMPTSHEON_VAULT_KEY      32-byte hex AES key for the provider vault
  PROMPTSHEON_OTEL_ENDPOINT  OTLP gRPC endpoint for traces
  PROMPTSHEON_CORS_ORIGINS   comma-separated CORS allowlist, or "*"

The full list is documented in docs/configuration.md.

Once running, the server exposes:

  GET /health              liveness probe (always unauthenticated)
  GET /ready               readiness probe (checks the database)
  GET /api/v1/version      build info (always unauthenticated)
  GET /metrics             Prometheus metrics (always unauthenticated)
  POST /api/v1/setup       first-run admin bootstrap; only active
                           when PROMPTSHEON_AUTH=false and the
                           user table is empty
  /api/v1/...              REST API (see api/openapi.yaml)

SECURITY: setting PROMPTSHEON_AUTH=false disables all
authentication. The first caller of /api/v1/setup receives an
admin key. Do not expose this server until you have set
PROMPTSHEON_AUTH=true, rotated the bootstrap key, and configured
a non-empty PROMPTSHEON_CORS_ORIGINS allowlist.
`
}
