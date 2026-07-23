// promptsheond is the Promptsheon API server daemon.
package main

import (
	"context"
	"database/sql"
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

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/sachncs/promptsheon/internal/alerting"
	"github.com/sachncs/promptsheon/internal/api"
	"github.com/sachncs/promptsheon/internal/auth"
	"github.com/sachncs/promptsheon/internal/election"
	"github.com/sachncs/promptsheon/internal/buildinfo"
	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/optimizer/rules"
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
	"github.com/sachncs/promptsheon/internal/recommendation"
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
	_ = showVersion // referenced via flag.Parse below
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

	// OTel tracer must outlive buildServer. The previous
	// implementation deferred tp.Shutdown inside buildServer,
	// which means it fired at the end of buildServer — before the
	// HTTP server accepted a single request. The result: every
	// span was a no-op even with a configured endpoint.
	var tp *sdktrace.TracerProvider
	if cfg.OTelEndpoint != "" {
		var terr error
		tp, terr = trace.InitTracerProvider("promptsheond", cfg.OTelEndpoint, cfg.OTelInsecure)
		if terr != nil {
			fmt.Fprintf(os.Stderr, "OTel tracer init failed: %v\n", terr)
		} else {
			defer func() {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = tp.Shutdown(shutdownCtx)
			}()
		}
	}

	// SECURITY: shell tool policy must be configured at startup, not
	// mutated at runtime. An empty allowlist disables the tool
	// regardless of the enabled flag.
	configureShellTool(&cfg)

	// rootCtx is cancelled on shutdown. All background goroutines
	// (retention, oauth janitor, alert monitor) hang off this context.
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	// Log hub is created early so the slog chain can broadcast
	// every daemon log line over the SSE /api/v1/logs/stream
	// endpoint (OBS-4). The hub is also passed to api.NewServer
	// via WithLogHub so handlers can subscribe.
	logHub := ws.NewHub(slog.Default())
	go logHub.Run()
	defer logHub.Stop()

	logger := setupLogger(&cfg, logHub)
	db := openDB(&cfg, logger)
	// OBS-LOG-3: load the persisted SSE client-id counter
	// once the DB is open. The hub runs with nextID=1 until
	// SetStore completes; the next HandleSSE call gets the
	// correct value.
	if err := logHub.SetStore(rootCtx, db); err != nil {
		logger.Warn("ws: load nextID failed; using process-local counter", "err", err)
	}
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
	// DEAD-Plg-2: wire the supervisor lifecycle events through
	// OBS-5a: share one eventbus.Memory between the supervisor,
	// scheduler, and executor. The supervisor's plugin-lifecycle
	// events, scheduler's schedule.fired events, and the
	// executor's HandleScheduleEvent subscriber all see the same
	// shared bus, so a scheduler tick reaches the executor without
	// a separate bridge.
	sharedBus := eventbus.NewMemory()
	sup := supervisor.New(supervisor.NewAdapter(sharedBus), logger)
	builtins.Register(sup)
	go func() {
		if err := sup.Run(rootCtx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Warn("plugin supervisor exited with error", "err", err)
		}
	}()

	// Schedule ticker: poll the schedules table for due rows and
	// publish schedule.fired events. The scheduler runs alongside
	// the supervisor and exits cleanly when rootCtx is cancelled.
	sched := scheduler.New(db, sharedBus, 5*time.Second)
	go func() {
		if err := sched.Start(rootCtx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Warn("scheduler exited with error", "err", err)
		}
	}()

	// Leader election for multi-replica deployments. Construct
	// the elector here (before buildServer) so the option can be
	// attached to the API server. The elector is only started
	// when PROMPTSHEON_LEADER_ELECTION=true; the default is
	// single-replica, where the daemon holds the lock itself
	// the moment it starts.
	var elector *election.Elector
	if os.Getenv("PROMPTSHEON_LEADER_ELECTION") == "true" {
		podName := os.Getenv("POD_NAME")
		if podName == "" {
			podName, _ = os.Hostname()
		}
		elector = election.New(db.DB(), podName, 30*time.Second)
		if err := elector.EnsureTable(rootCtx); err != nil {
			logger.Warn("leader-election table init failed", "err", err)
		}
		go func() {
			errs := make(chan error, 8)
			go elector.Run(rootCtx, errs)
			for e := range errs {
				logger.Warn("leader-election error", "err", e)
			}
		}()
		if err := elector.Acquire(rootCtx); err != nil && !errors.Is(err, election.ErrNotLeader) {
			logger.Warn("initial leader acquire failed", "err", err)
		}
		logger.Info("leader-election active", "pod", podName)
	}

	// DB-CONC-2: open a separate *sql.DB for the retention loop so
	// the DELETE on traces never competes for the same write
	// connection as the request path. SQLite serialises writers,
	// so the cleanup must not share the main pool. The lifetime
	// of this handle matches the daemon: opened before buildServer
	// (so the background retention loop outlives buildServer's
	// stack), closed in main's defer after the server shuts down.
	retentionDB, err := sql.Open("sqlite", cfg.DBPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		logger.Warn("retention: open dedicated db failed; falling back to main db", "err", err)
		retentionDB = db.DB()
	} else {
		defer func() { _ = retentionDB.Close() }()
	}

	srv, limiter, tracer, collector := buildServer(rootCtx, &cfg, db, logger, tp, logHub, elector, retentionDB, sharedBus)

	srv.StartAuditWorkers(rootCtx, 2)

	// API-IDEMP-1: wire the SQLite-backed IdempotencyStore so
	// multi-replica deployments see the same Idempotency-Key
	// replay window. Falls back to the in-process cache when
	// the store can't be opened (e.g. read-only filesystem);
	// the middleware degrades gracefully in that case.
	idempStore := store.NewSQLiteIdempotencyStore(db.DB())
	startHTTPServerAndWait(rootCtx, rootCancel, &cfg, srv, logger, limiter, tracer, collector, idempStore)
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

func setupLogger(cfg *config.Config, hub *ws.Hub) *slog.Logger {
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
	// OBS-4: chain the JSON stderr handler through a StreamHandler
	// that broadcasts every log line to the SSE hub. The hub has
	// its own broadcast loop and is concurrency-safe.
	var base slog.Handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	if hub != nil {
		streamer := &ws.LogStreamer{Hub: hub}
		base = streamer.StreamHandler(base)
	}
	return slog.New(base)
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

func buildServer(rootCtx context.Context, cfg *config.Config, db *store.SQLite, logger *slog.Logger, tp *sdktrace.TracerProvider, logHub *ws.Hub, elector *election.Elector, retentionDB *sql.DB, sharedBus eventbus.Publisher) (*api.Server, *ratelimit.Limiter, trace.Tracer, *metrics.Collector) {
	// OBS-TR-1: no SQLite tracer; OTel-only export.
	collector := metrics.NewCollector()
	// OBS-LOG-2: wire the SSE hub's drop counter into the
	// collector so the Prometheus scrape and the
	// /api/v1/metrics/summary JSON expose it as
	// promptsheon_log_hub_drops_total.
	collector.SetLogHub(logHub)

	// OBS-2: OTel-only export. The SQLite tracer no longer writes;
	// /api/v1/traces/{id} reads are answered by the read-side
	// *SQLite above. Spans are NOT mirrored to both backends.
	var tracer trace.Tracer = trace.NewNoopTracer()
	if cfg.OTelEndpoint != "" && tp != nil {
		tracer = trace.NewOTelTracer("promptsheond")
		logger.Info("OTel tracer initialised", "endpoint", cfg.OTelEndpoint, "insecure", cfg.OTelInsecure)
	}

	retentionPolicy := observability.LoadRetentionPolicyFromEnv()
	// DB-CONC-2: retentionDB is passed in from main() so its
	// lifetime matches the daemon lifetime, not buildServer's.
	// Closing it inside buildServer would tear down the DB
	// before the retention goroutine ever runs a tick.
	retention := observability.NewRetentionManager(retentionDB, retentionPolicy, logger)
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

	_ = logHub

	contextMgr := contextpkg.NewManager()
	usageTracker := api.NewUsageTracker()
	guardrailManager := guardrail.NewManager(logger, collector)
	alertingManager := alerting.NewManagerWithDB(logger, collector, db)
	alertingManager.StartMonitoring(rootCtx, collector, 1*time.Minute)

	// Wire alert delivery. The delivery function emits
	// "webhook" channels through the dispatcher; "log" channels
	// are routed to the structured logger. Anything else is
	// logged as "no delivery handler" so operators see the gap
	// rather than the alert being silently dropped. Without
	// this wiring the audit noted the previous behaviour: alerts
	// persisted to the DB but no notification was sent.
	alertingManager.SetDeliveryFunc(func(alert *alerting.Alert, channels []string) error {
		for _, ch := range channels {
			switch ch {
			case "webhook":
				webhookDispatcher.EmitContext(rootCtx, &webhook.Event{
					ID:   alert.ID,
					Type: "alert.fired",
					Data: map[string]any{
						"alert_id":  alert.ID,
						"rule_id":   alert.RuleID,
						"severity":  alert.Severity,
						"message":   alert.Message,
						"details":   alert.Details,
						"triggered": alert.TriggeredAt,
					},
					Timestamp: time.Now().UTC(),
				})
			case "log":
				logger.Warn("alert fired",
					"alert_id", alert.ID,
					"rule_id", alert.RuleID,
					"severity", alert.Severity,
					"message", alert.Message,
				)
			default:
				logger.Warn("alert channel has no delivery handler; persisted to DB only",
					"alert_id", alert.ID,
					"channel", ch,
				)
			}
		}
		return nil
	})

	limiter := ratelimit.NewLimiter(ratelimit.LoadConfigFromEnv())

	providers := llm.NewRegistry()
	providers.LoadFromEnv()
	// SEC-LLM-1: refuse to start when an LLM base URL is http://
	// but the daemon binds a non-loopback address. Production
	// deployments must talk to LLM providers over TLS.
	if err := providers.ValidateBaseURLs(cfg.Addr, config.IsLoopbackAddr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	// Per-Workspace rollup aggregator (Tier 2.37). The production
	// wiring supplies a backend-backed Budget/Quota repository.
	// When PROMPTSHEON_CLICKHOUSE_DSN is set and the binary is
	// built with the `clickhouse` tag, the rollup job persists
	// summaries to ClickHouse via internal/rollups/clickhouse.
	// When the binary is built without the tag, buildClickHouseWriter
	// returns a clear diagnostic so the operator knows to rebuild.
	rollupAgg := rollups.New(nil, nil)
	if dsn := os.Getenv("PROMPTSHEON_CLICKHOUSE_DSN"); dsn != "" {
		if writer, werr := buildClickHouseWriter(rootCtx, dsn, "promptsheon", logger); werr != nil {
			logger.Warn("clickhouse writer disabled", "err", werr)
		} else if writer != nil {
			logger.Info("clickhouse writer initialised")
			// Start a 30-second flusher that drains the
			// workspace rollup queue into ClickHouse. The
			// default interval matches Prometheus's typical
			// scrape cadence so dashboards see fresh data.
			// The buildClickHouseWriter placeholder returns
			// (nil, error). The real writer under -tags
			// clickhouse will set up a Sink that writes the
			// periodic WorkspaceSummary rollups.
			_ = writer
		}
	}

	// Canonical invoke.Invoker (Tier 2.36). The Caller resolves
	// the model + provider from the request, then routes through
	// the LLM registry so OpenAI / Anthropic / Ollama / etc. all
	// go through the same path. When no provider is configured
	// the Caller returns an explicit error rather than a fake
	// success — the route surfaces it as 502 Bad Gateway.
	// OBS-13: persisted enforcer. Budget + quota state survives
	// process restarts via the enforcer_state table (migration 012).
	enforcer := invoke.NewPersistedEnforcer(rootCtx, db, nil, logger)
	agg := observation.NewAggregator(nil)
	// Recommendation loop: each invocation that lands in the
	// observation aggregator can produce a Recommendation
	// (raise max_tokens, drop a guardrail, change temperature).
	// The producer is wired with the SQLite-backed
	// recommendation.Repository so recommendations survive
	// process restarts (migration 042). Decisions are written by
	// the HTTP API and surface via the existing
	// /recommendations routes.
	recRepo := recommendation.NewSQLiteRepository(db.DB())
	recBus := eventbus.NewMemory()
	recSink := func(ctx context.Context, r *capability.Recommendation) error {
		return recRepo.CreateRecommendation(ctx, r)
	}
	recProducer := recommendation.New(agg, rules.NewEngine(), recBus, recSink, logger)
	// OBS-12 follow-up: subscribe the producer to the
	// execution.finished events on the same bus so it reacts to
	// live invocations instead of only running on the periodic
	// ticker. The ticker still runs every 5 minutes as a backstop.
	if _, err := recProducer.Subscribe(recBus, capability.EventExecutionFinished); err != nil {
		logger.Warn("recommendation subscription failed", "err", err)
	}
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-rootCtx.Done():
				return
			case t := <-ticker.C:
				if _, err := recProducer.Tick(rootCtx, t); err != nil {
					logger.Warn("recommendation tick failed", "err", err)
				}
			}
		}
	}()

	exec := executor.New(sharedBus, func(ctx context.Context, req executor.InvokeRequest) (executor.InvokeResult, error) {
		// Provider routing: the canonical request now carries an
		// explicit Provider field (set by the release Resolver).
		// We look up the provider by that field rather than by
		// req.Model, so a request that names a model registered
		// under multiple providers lands on the one the release
		// actually approved. The previous "fall back to the first
		// registered provider" was the source of the wrong-provider
		// bug: an unconfigured or upstream-error call would land
		// on a random provider, then be recorded as a successful
		// "stub execution" because the Caller swallowed the error.
		if req.Provider == "" {
			return executor.InvokeResult{Status: "error", Error: "no provider specified in invocation"}, executor.ErrProviderMissing
		}
		p, err := providers.Get(req.Provider)
		if err != nil {
			return executor.InvokeResult{Status: "error", Error: "provider not registered: " + req.Provider}, executor.ErrProviderMissing
		}
		if req.Model == "" || req.Model == "<unspecified>" {
			return executor.InvokeResult{Status: "error", Error: "no model configured"}, executor.ErrProviderMissing
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
	})

	// OBS-5a: subscribe the executor to schedule.fired events on
	// the shared bus so scheduler ticks route through it without
	// needing a separate bridge. The sharedBus may be nil in unit
	// tests that don't wire observability; skip the subscription in
	// that case.
	if sharedBus != nil {
		if _, err := sharedBus.Subscribe(
			func(ev capability.Event) { _ = exec.HandleScheduleEvent(rootCtx, ev) },
			capability.EventType("schedule.fired"),
		); err != nil {
			logger.Warn("executor: subscribe schedule.fired failed", "err", err)
		}
	}

	inv := invoke.New(enforcer, agg, exec).
		WithObservability(collector, tracer, logger)

	var opts []api.Option
	if cfg.Auth {
		opts = append(opts, api.WithAuth(db))
		logger.Info("authentication enabled")
	}
	if tracer != nil {
		opts = append(opts, api.WithTracing(tracer, collector))
	}
	if elector != nil {
		opts = append(opts, api.WithElector(elector))
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
		api.WithWorkflowEngine(
		workflow.NewEngine(workflow.DefaultRegistry()).
			WithObservability(collector, tracer),
	),
		api.WithOAuth(buildOAuthManager(cfg)),
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
	return srv, limiter, tracer, collector
}

// buildOAuthManager constructs an *auth.OAuthManager from
// environment variables. Google and GitHub providers are
// registered when all three of their env vars (CLIENT_ID,
// CLIENT_SECRET, REDIRECT_URL) are present; otherwise they are
// silently skipped. The login/callback routes work for any
// provider that is registered.
func buildOAuthManager(cfg *config.Config) *auth.OAuthManager {
	mgr := auth.NewOAuthManager()
	if google := buildGoogleOAuth(cfg); google != nil {
		mgr.RegisterProvider("google", google)
	}
	if github := buildGitHubOAuth(cfg); github != nil {
		mgr.RegisterProvider("github", github)
	}
	return mgr
}

func buildGoogleOAuth(cfg *config.Config) *auth.OAuthProvider {
	clientID := os.Getenv("PROMPTSHEON_OAUTH_GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("PROMPTSHEON_OAUTH_GOOGLE_CLIENT_SECRET")
	redirectURL := os.Getenv("PROMPTSHEON_OAUTH_GOOGLE_REDIRECT_URL")
	if clientID == "" || clientSecret == "" || redirectURL == "" {
		return nil
	}
	return &auth.OAuthProvider{
		Name:         "google",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		UserInfoURL:  "https://openidconnect.googleapis.com/v1/userinfo",
		Scopes:       []string{"openid", "email", "profile"},
	}
}

func buildGitHubOAuth(cfg *config.Config) *auth.OAuthProvider {
	clientID := os.Getenv("PROMPTSHEON_OAUTH_GITHUB_CLIENT_ID")
	clientSecret := os.Getenv("PROMPTSHEON_OAUTH_GITHUB_CLIENT_SECRET")
	redirectURL := os.Getenv("PROMPTSHEON_OAUTH_GITHUB_REDIRECT_URL")
	if clientID == "" || clientSecret == "" || redirectURL == "" {
		return nil
	}
	return &auth.OAuthProvider{
		Name:         "github",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		AuthURL:      "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
		UserInfoURL:  "https://api.github.com/user",
		Scopes:       []string{"read:user", "user:email"},
	}
}

// buildReleaseService constructs the application-layer release.Service
// using the configured policy. Returns nil if the policy string is
// unrecognized; the daemon continues to run with release routes
// unregistered (404) rather than failing the boot.
func buildReleaseService(db *store.SQLite, policy string) *release.Service {
	switch policy {
	case "majority":
		return release.NewServiceMajority(db, db, 1)
	case "", "maker_checker":
		return release.NewServiceMakerChecker(db, db, 1)
	default:
		// unknown policy: log nothing here; the daemon's job is to
		// keep running. Operators see the misconfiguration when they
		// hit /activate.
		return nil
	}
}

// buildClickHouseWriter constructs a ClickHouse writer when the
// binary is built with the `clickhouse` build tag. The function
// returns nil + nil when the tag is absent so production
// binaries without the tag still boot cleanly.
func buildClickHouseWriter(ctx context.Context, dsn, database string, logger *slog.Logger) (any, error) {
	return nil, fmt.Errorf("clickhouse writer not compiled in (rebuild with -tags clickhouse)")
}

func startHTTPServerAndWait(rootCtx context.Context, rootCancel func(), cfg *config.Config, srv *api.Server, logger *slog.Logger, limiter *ratelimit.Limiter, tracer trace.Tracer, collector *metrics.Collector, idempStore store.IdempotencyStore) {
	handler := api.ChainHTTP(srv,
		api.Recovery(logger),
		api.MaxBytesReader(10<<20),
		api.SecurityHeaders,
		api.IdempotencyMiddleware(idempStore),
		limiter.Middleware,
		metrics.HTTPMiddleware(collector, tracer, logger),
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
			"tls", cfg.TLSCertFile != "",
		)
		if !cfg.Auth {
			logger.Warn("authentication is DISABLED; POST /api/v1/setup will mint an admin key to the first caller. Set PROMPTSHEON_AUTH=true before exposing this server.",
				"setup_endpoint", "POST /api/v1/setup")
		}
		var serveErr error
		if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
			serveErr = httpServer.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			serveErr = httpServer.ListenAndServe()
		}
		if serveErr != nil && serveErr != http.ErrServerClosed {
			logger.Error("server error", "err", serveErr)
			os.Exit(1)
		}
	}()

	api.StartOAuthStateJanitor(rootCtx)

	// OBS-1b (deferred): with OBS-TR-1 there is no SQLite writer to
	// expose a drop count from. The metric wiring stays so a
	// future OTel-aware drop counter can drop in without API
	// changes. For now, promptsheon_trace_dropped_total stays at
	// zero and the loop is omitted.
	_ = collector // collector.SetTraceDropped exists; see metrics.Collector.

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

	// OBS-TR-1: no SQLite writer to drain. The trace export is
	// already closed (deferred to wherever the OTel provider's
	// Shutdown lives).

	auditStopCtx, cancelAuditStop := context.WithTimeout(context.Background(), 10*time.Second)
	if err := srv.StopAuditWorkers(auditStopCtx); err != nil {
		logger.Warn("audit workers did not drain in time", "err", err)
	}
	cancelAuditStop()

	// Stop the webhook dispatcher AFTER HTTP shutdown so in-flight
	// handler-triggered Emit calls have a chance to enqueue. The
	// dispatcher's WaitGroup drains in-flight HTTP deliveries before
	// the goroutine returns.
	srv.StopDependents()

	limiter.Stop()
	api.StopOAuthStateJanitor()
	if srv.Authenticator() != nil {
		srv.Authenticator().Stop()
	}
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
	// SEC-7a: persist the ciphertext, never the plaintext. The
	// plaintext Secret lives only in the in-memory Endpoint
	// during this process's lifetime.
	return a.db.SaveWebhookEndpoint(ctx, &models.WebhookEndpointRecord{
		ID:               ep.ID,
		URL:              ep.URL,
		Secret:           "",
		SecretCiphertext: ep.SecretCiphertext,
		Events:           events,
		Active:           ep.Active,
		CreatedAt:        ep.CreatedAt,
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
			ID:               r.ID,
			URL:              r.URL,
			SecretCiphertext: r.SecretCiphertext,
			Events:           evs,
			Active:           r.Active,
			CreatedAt:        r.CreatedAt,
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
