import Fastify, { type FastifyError } from 'fastify';
import { randomUUID } from 'node:crypto';
import cors from '@fastify/cors';
import rateLimit from '@fastify/rate-limit';
import { loadConfig } from './config/env.js';
import { validateConfig } from './config/validate.js';
import { createConnection, runMigrations } from './db/index.js';
import { buildRepos, type Repos } from './repos/factory.js';
import { registerRoutes } from './routes/index.js';
import { authMiddleware } from './middleware/index.js';
import { orgContextMiddleware } from './middleware/org-context.js';
import { SseHub } from './sse/hub.js';
import { SettingsResolver } from './settings/resolver.js';
import { AuditChain } from './audit/chain.js';
import { Scheduler } from './scheduler/scheduler.js';
import { InvocationAgent } from './agents/invocation.js';
import { EvaluationAgent } from './agents/evaluation/evaluation.js';
import { EvolutionAgent } from './agents/evolution/evolution.js';
import { GoalBasedEvolutionAgent } from './agents/evolution/goal-evolver.js';
import { ReasoningCompiler } from './agents/compiler/compiler.js';
import { IdeaPlannerAgent } from './agents/planner/index.js';
import { ManifestGraphExecutor } from './agents/executor/index.js';
import { AutoEval } from './observability/auto-eval.js';
import { CostForecastService } from './analysis/forecast.js';
import { CasStore } from '@promptsheon/shared';
import { setupObservability } from './observability/setup.js';
import type { GoalSummary } from './routes/goals.js';
import { SessionStore } from './sessions/store.js';
import { SnapshotStore } from './snapshots/store.js';
import { CedarAuthorizer, installDefaultAuthorizer } from './policy/gate.js';
import { WebhookReceiver } from './webhooks/receiver.js';
import { ChaosConfig } from './hardening/chaos.js';
import { LlmRouter } from './llm/router.js';
import { Gateway, ResponseCache, FallbackChain, RateLimiter } from './llm/gateway.js';
import type { Agent } from '@strands-agents/sdk';
import type Database from 'better-sqlite3';

/**
 * Load the Cedar policy file at boot and install the singleton
 * authorizer. The policy file is the single source of truth for
 * every authorization decision in the platform; failing to load
 * it is a fatal error.
 */
async function setupPolicy(): Promise<void> {
  const policyPath = process.env['PROMPTSHEON_POLICY_FILE'];
  const authorizer = new CedarAuthorizer({ ...(policyPath ? { policyPath } : {}) });
  authorizer.load();
  installDefaultAuthorizer(authorizer);
}

/**
 * Resolve the webhook secret. Refuses to boot in production
 * with the dev fallback.
 */
function resolveWebhookSecret(nodeEnv: string): string {
  const fromEnv = process.env['PROMPTSHEON_WEBHOOK_SECRET'];
  if (fromEnv && fromEnv.length > 0) return fromEnv;
  if (nodeEnv !== 'production') return 'dev-secret';
  throw new Error(
    'PROMPTSHEON_WEBHOOK_SECRET is required in production. Refusing to boot with the dev fallback.',
  );
}

/**
 * Build the cost forecast service from the repo bundle. Pulled
 * out of main() so the budget handler can stay terse.
 */
function buildForecastService(db: Database.Database, repos: Repos): CostForecastService {
  return new CostForecastService(db, {
    rollups: repos.costRollup,
    persistSnapshot: (snap) => repos.budget.insertForecastSnapshot(snap),
    listBudgets: (orgId) => repos.budget.listForOrg(orgId),
    updateLastAlerted: (id, ts) => repos.budget.updateLastAlerted(id, ts),
  });
}

/**
 * Helper used in every admin-only router dep. Reads the
 * resolved org-context role off the Fastify request and
 * returns true iff the caller is an admin.
 */
function adminOnly(request: unknown): boolean {
  const ctx = request as { orgContext?: { role?: string } } | undefined;
  return ctx?.orgContext?.role === 'admin';
}

async function main() {
  const config = loadConfig();
  validateConfig(config);

  const db = createConnection(config);
  await runMigrations(db);

  // Build every repo from the single Database handle. The
  // factory lives in src/repos/factory.ts and replaces the 41
  // manual `new XRepo(db)` calls this function used to carry.
  const repos = buildRepos(db);

  const auditChain = new AuditChain(db, config.server.fipsMode);
  const app = Fastify({ logger: true, bodyLimit: 2_097_152 });

  if (config.server.fipsMode) {
    app.log.warn('PROMPTSHEON_FIPS_MODE=true — audit chain requires a FIPS-validated Node build');
  }

  app.log.info({ corsOrigin: config.server.corsOrigin }, 'CORS configuration');

  await app.register(cors, {
    origin: config.server.corsOrigin,
    methods: ['GET', 'POST', 'PUT', 'DELETE', 'PATCH'],
    allowedHeaders: ['Content-Type', 'Authorization', 'X-Request-Id', 'Idempotency-Key'],
    credentials: true,
  });

  app.addHook('onRequest', async (request, reply) => {
    const requestIdHeader = request.headers['x-request-id'];
    const requestId = typeof requestIdHeader === 'string' && requestIdHeader || randomUUID();
    const requestMetadata = request as unknown as Record<string, string | number>;
    requestMetadata.requestId = requestId;
    requestMetadata.startTime = Date.now();
    reply.header('X-Request-Id', requestId);
  });

  app.addHook('onResponse', async (request, reply) => {
    const requestMetadata = request as unknown as Record<string, string | number>;
    const requestId = requestMetadata.requestId;
    const startTime = requestMetadata.startTime || Date.now();
    app.log.info({
      requestId,
      method: request.method,
      url: request.url,
      status: reply.statusCode,
      durationMs: Date.now() - Number(startTime),
    }, 'request');
  });

  await app.register(rateLimit, {
    max: 100,
    timeWindow: '1 minute',
    keyGenerator: (req) => {
      return (req as unknown as Record<string, string>).userId ?? req.ip ?? 'unknown';
    },
  });

  const sseHub = new SseHub();
  const settingsResolver = new SettingsResolver(
    {},
    process.env as Record<string, string>,
    repos.systemConfig,
  );

  const casStore = new CasStore(config.server.casPath);
  await casStore.init();

  setupObservability(config);
  await setupPolicy();

  const cutoverReport = repos.manifest.ensureCutover({ createdBy: 'system-cutover' });
  app.log.info(
    {
      scanned: cutoverReport.scanned,
      migrated: cutoverReport.migrated,
      skipped: cutoverReport.skipped,
      errors: cutoverReport.errors.length,
    },
    'manifest DAG cutover complete',
  );

  const invocationAgent = new InvocationAgent(config);
  const evalAgent = new EvaluationAgent(config);
  const evolutionAgent = new EvolutionAgent(config, { cas: casStore });
  const compiler = new ReasoningCompiler(config);
  const planner = new IdeaPlannerAgent(config);
  const executor = new ManifestGraphExecutor({ config, hub: sseHub, manifestRepo: repos.manifest });
  const llmRouter = new LlmRouter();
  const autoEval = new AutoEval({ traceRepo: repos.trace, scoreRepo: repos.traceScore, router: llmRouter });
  const gateway = new Gateway({
    cache: new ResponseCache(2048),
    fallback: new FallbackChain(['custom', 'anthropic', 'openai']),
    rateLimiter: new RateLimiter({ capacity: 60, refillPerSecond: 1 }),
    router: llmRouter,
  });
  const chaosConfig = new ChaosConfig();
  const goalEvolver = new GoalBasedEvolutionAgent({
    config,
    hub: sseHub,
    executor,
    cas: casStore,
  });
  const activeGoals = new Map<string, GoalSummary>();
  setInterval(() => {
    for (const [hash, state] of (goalEvolver as unknown as { state: Map<string, unknown> }).state ?? new Map()) {
      const s = state as { currentHash: string; bestHash: string; bestScore: number; iteration: number };
      activeGoals.set(hash, {
        manifestHash: hash,
        bestScore: s.bestScore,
        iterations: s.iteration,
        lastUpdated: new Date().toISOString(),
      });
    }
  }, 1000).unref();
  const sessionStore = new SessionStore({
    storageDir: `${config.server.casPath}/sessions`,
    persist: true,
  });
  await sessionStore.init();

  const snapshotStore = new SnapshotStore({ storageDir: `${config.server.casPath}/snapshots` });
  await snapshotStore.init();

  // In-memory agent registry (single-process); production would use a
  // multi-tenant map keyed by tenantId + capabilityId.
  const agentRegistry = new Map<string, Agent>();

  const webhookReceiver = new WebhookReceiver(
    [
      {
        id: 'github-push',
        url: 'https://example.com/github',
        events: ['push', 'pull_request'],
        active: true,
        secret: resolveWebhookSecret(config.server.nodeEnv),
      },
    ],
    [
      {
        endpointId: 'github-push',
        eventType: 'push',
        manifestHash: '',
        inputMapping: { ref: 'ref' },
      },
    ],
  );

  app.addHook('preHandler', authMiddleware(config, repos.apiKey));
  app.addHook('preHandler', orgContextMiddleware({ membershipRepo: repos.membership }));

  app.setErrorHandler((error: FastifyError, _request, reply) => {
    if (error.name === 'NotFoundError') {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: error.message } });
    }
    if (error.statusCode) {
      return reply.code(error.statusCode).send({ error: { code: 'APP_ERROR', message: error.message } });
    }
    if (error.message.includes('Validation') || error.message.includes('ZodError')) {
      return reply.code(422).send({ error: { code: 'VALIDATION_ERROR', message: error.message } });
    }
    app.log.error(error);
    return reply.code(500).send({ error: { code: 'INTERNAL_ERROR', message: 'Internal server error' } });
  });

  const { RetentionSweeper } = await import('./scheduler/retention-sweeper.js');
  const retention = new RetentionSweeper(
    db,
    {
      append: (entry) => {
        auditChain.append({
          userId: entry.userId,
          action: entry.action,
          resource: entry.resource,
          details: entry.details,
          resourceKind: entry.resourceKind,
          resourceId: entry.resourceId,
        });
      },
    },
    () => new Date(),
  );
  retention.start();

  await registerRoutes(app, {
    db,
    workspaceRepo: repos.workspace,
    projectRepo: repos.project,
    capabilityRepo: repos.capability,
    versionRepo: repos.version,
    releaseRepo: repos.release,
    executionRepo: repos.execution,
    datasetRepo: repos.dataset,
    evalRepo: repos.eval,
    preconditionRepo: repos.precondition,
    alertRepo: repos.alert,
    scheduleRepo: repos.schedule,
    approvalRepo: repos.approval,
    sseHub,
    settingsResolver,
    invocationAgent,
    evalAgent,
    evolutionAgent,
    goalEvolver,
    compiler,
    planner,
    executor,
    manifestRepo: repos.manifest,
    getActiveGoals: () => Array.from(activeGoals.values()),
    sessionStore,
    snapshotStore,
    getAgent: (id: string) => {
      const [executionId, nodeId] = id.includes(':') ? id.split(':') : ['', id];
      if (executionId && nodeId) {
        return executor.getLiveAgent(executionId, nodeId) ?? agentRegistry.get(id) ?? null;
      }
      return agentRegistry.get(id) ?? null;
    },
    membershipRepo: repos.membership,
    webhookReceiver,
    chaosConfig,
    auditChain,
    apiKeyRepo: repos.apiKey,
    userRepo: repos.user,
    llmRouter,
    repoDeps: {
      repoRepo: repos.repo,
      branchRepo: repos.branch,
      tagRepo: repos.tag,
    },
    contentsDeps: {
      repoRepo: repos.repo,
      branchRepo: repos.branch,
      repoStore: repos.repoStore,
    },
    commitDeps: {
      repoRepo: repos.repo,
      branchRepo: repos.branch,
      repoStore: repos.repoStore,
      commitRepo: repos.commit,
    },
    mrDeps: {
      repoRepo: repos.repo,
      branchRepo: repos.branch,
      mrRepo: repos.mergeRequest,
    },
    signingDeps: {
      repoRepo: repos.repo,
      commitRepo: repos.commit,
      signingKeyRepo: repos.signingKey,
    },
    evalSuiteDeps: {
      suiteRepo: repos.evalSuite,
      humanReviewRepo: repos.humanReview,
    },
    vaultDeps: {
      vaultRepo: repos.vault,
      orgExportService: repos.orgExport,
      costRollupRepo: repos.costRollup,
      kms: repos.vault.kms,
      adminOnly,
    },
    retentionDeps: { sweeper: retention, adminOnly },
    redteamDeps: { redteamRepo: repos.redteam, adminOnly },
    experimentDeps: { experimentRepo: repos.experiment },
    incidentDeps: { incidentRepo: repos.incident, actorId: () => 'system' },
    featureFlagRepo: repos.featureFlag,
    traceRepo: repos.trace,
    traceScoreRepo: repos.traceScore,
    autoEval,
    userAnalyticsRepo: repos.userAnalytics,
    teamRepo: repos.team,
    ssoConfigRepo: repos.ssoConfig,
    promptScanRepo: repos.promptScan,
    gateway,
    budgetDeps: {
      budgetRepo: repos.budget,
      forecastService: buildForecastService(db, repos),
    },
    orgSettingsDeps: {
      orgSettingsRepo: repos.orgSettings,
      vaultRepo: repos.vault,
      adminOnly,
    },
  });

  const scheduler = new Scheduler(repos.schedule, sseHub);
  scheduler.start();

  const port = config.server.port;
  const host = config.server.host;
  await app.listen({ port, host });
  app.log.info(`Promptsheon server listening on ${host}:${port}`);

  const shutdown = async (signal: string) => {
    app.log.info(`Received ${signal}, shutting down gracefully`);
    scheduler.stop();
    await app.close();
    db.close();
    process.exit(0);
  };
  process.on('SIGTERM', () => shutdown('SIGTERM'));
  process.on('SIGINT', () => shutdown('SIGINT'));
}

main().catch((err) => {
  console.error('Failed to start server:', err);
  process.exit(1);
});
