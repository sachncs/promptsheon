import Fastify, { type FastifyError } from 'fastify';
import { randomUUID } from 'node:crypto';
import cors from '@fastify/cors';
import rateLimit from '@fastify/rate-limit';
import { loadConfig } from './config/env.js';
import { createConnection, runMigrations } from './db/index.js';
import { registerRoutes } from './routes/index.js';
import { authMiddleware } from './middleware/index.js';
import { SseHub } from './sse/hub.js';
import { SettingsResolver } from './settings/resolver.js';
import { AuditChain } from './audit/chain.js';
import { Scheduler } from './scheduler/scheduler.js';
import { InvocationAgent } from './agents/invocation.js';
import { EvaluationAgent } from './agents/evaluation/evaluation.js';
import { EvolutionAgent } from './agents/evolution/evolution.js';
import { GoalBasedEvolutionAgent } from './agents/evolution/goal-evolver.js';
import { ReasoningCompiler } from './agents/compiler/compiler.js';
import { CasStore } from '@promptsheon/shared';
import { WorkspaceRepo } from './repos/workspace.js';
import { ProjectRepo } from './repos/project.js';
import { RepoRepo } from './repos/repo.js';
import { BranchRepo } from './repos/branch.js';
import { TagRepo } from './repos/tag.js';
import { RepoStore } from './repos/repo-store.js';
import { CommitRepo } from './repos/commit.js';
import { MergeRequestRepo } from './repos/mr.js';
import { SigningKeyRepo } from './repos/signing-key.js';
import { EvalSuiteRepo, HumanReviewRepo } from './repos/eval-suite.js';
import { VaultRepo } from './repos/vault.js';
import { TraceRepo } from './repos/trace.js';
import { OrgExportService, CostRollupRepo } from './repos/vault-extras.js';
import { RedteamRepo } from './repos/redteam.js';
import { ExperimentRepo } from './repos/experiment.js';
import { IncidentRepo } from './repos/incident.js';
import { OrgSettingsRepo } from './repos/org-settings.js';
import { FeatureFlagRepo } from './repos/feature-flag.js';
import { CapabilityRepo } from './repos/capability.js';
import { VersionRepo } from './repos/version.js';
import { ReleaseRepo } from './repos/release.js';
import { ExecutionRepo } from './repos/execution.js';
import { DatasetRepo } from './repos/dataset.js';
import { EvalRepo } from './repos/eval.js';
import { PreconditionRepo } from './repos/precondition.js';
import { AlertRepo } from './repos/alert.js';
import { ScheduleRepo } from './repos/schedule.js';
import { ApprovalRepo } from './repos/approval.js';
import { ApiKeyRepo } from './repos/api-key.js';
import { UserRepo } from './repos/user.js';
import { SystemConfigRepo } from './repos/system-config.js';
import { ManifestRepo } from './repos/manifest.js';
import { MembershipRepo } from './repos/org.js';
import { IdeaPlannerAgent } from './agents/planner/index.js';
import { ManifestGraphExecutor } from './agents/executor/index.js';
import { setupObservability } from './observability/setup.js';
import type { GoalSummary } from './routes/goals.js';
import { SessionStore } from './sessions/store.js';
import { SnapshotStore } from './snapshots/store.js';
import { orgContextMiddleware } from './middleware/org-context.js';
import { WebhookReceiver } from './webhooks/receiver.js';
import { ChaosConfig } from './hardening/chaos.js';
import { registerChaosRoutes } from './routes/chaos.js';
import { LlmRouter } from './llm/router.js';
import type { Agent } from '@strands-agents/sdk';

async function main() {
  const config = loadConfig();
  const db = createConnection(config);
  await runMigrations(db);
  const auditChain = new AuditChain(db);

  const app = Fastify({ logger: true, bodyLimit: 2_097_152 });

  const corsOrigin = config.server.corsOrigin || 'http://localhost:5173';
  if (!config.server.corsOrigin) {
    app.log.warn(`CORS origin not set, defaulting to ${corsOrigin}`);
  }

  await app.register(cors, {
    origin: corsOrigin,
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

  const workspaceRepo = new WorkspaceRepo(db);
  const projectRepo = new ProjectRepo(db);
  const repoRepo = new RepoRepo(db);
  const branchRepo = new BranchRepo(db);
  const tagRepo = new TagRepo(db);
  const repoStore = new RepoStore(db);
  const commitRepo = new CommitRepo(db);
  const mrRepo = new MergeRequestRepo(db);
  const signingKeyRepo = new SigningKeyRepo(db);
  const evalSuiteRepo = new EvalSuiteRepo(db);
  const humanReviewRepo = new HumanReviewRepo(db);
  const vaultRepo = new VaultRepo(
  db,
  new (await import('./repos/vault.js')).LocalKms(db),
);
  const orgExportService = new OrgExportService(db, vaultRepo);
  const costRollupRepo = new CostRollupRepo(db);
  const traceRepo = new TraceRepo(db);
  const redteamRepo = new RedteamRepo(db);
  const experimentRepo = new ExperimentRepo(db);
  const incidentRepo = new IncidentRepo(db);
  const orgSettingsRepo = new OrgSettingsRepo(db);
  const featureFlagRepo = new FeatureFlagRepo(db);
  const capabilityRepo = new CapabilityRepo(db);
  const versionRepo = new VersionRepo(db);
  const releaseRepo = new ReleaseRepo(db);
  const executionRepo = new ExecutionRepo(db);
  const datasetRepo = new DatasetRepo(db);
  const evalRepo = new EvalRepo(db);
  const preconditionRepo = new PreconditionRepo(db);
  const alertRepo = new AlertRepo(db);
  const scheduleRepo = new ScheduleRepo(db);
  const approvalRepo = new ApprovalRepo(db);
  const apiKeyRepo = new ApiKeyRepo(db);
  const membershipRepo = new MembershipRepo(db);
  const systemConfigRepo = new SystemConfigRepo(db);

  const sseHub = new SseHub();
  const settingsResolver = new SettingsResolver(
    {},
    process.env as Record<string, string>,
    systemConfigRepo,
  );

  const casStore = new CasStore(config.server.casPath);
  await casStore.init();

  const manifestRepo = new ManifestRepo(db);

  setupObservability(config);
  const cutoverReport = manifestRepo.ensureCutover({ createdBy: 'system-cutover' });
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
  const executor = new ManifestGraphExecutor({ config, hub: sseHub, manifestRepo });
  const chaosConfig = new ChaosConfig();
  const goalEvolver = new GoalBasedEvolutionAgent({ config, hub: sseHub, executor, cas: casStore });
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
        secret: (() => {
          const v = process.env['PROMPTSHEON_WEBHOOK_SECRET'];
          if (v && v.length > 0) return v;
          if (config.server.nodeEnv !== 'production') return 'dev-secret';
          throw new Error(
            'PROMPTSHEON_WEBHOOK_SECRET is required in production. Refusing to boot with the dev fallback.',
          );
        })(),
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

  app.addHook('preHandler', authMiddleware(config, apiKeyRepo));
  app.addHook('preHandler', orgContextMiddleware({ membershipRepo }));

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

  const RetentionSweeperModule = await import('./scheduler/retention-sweeper.js');
  const RetentionSweeper = RetentionSweeperModule.RetentionSweeper;
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
    workspaceRepo,
    projectRepo,
    capabilityRepo,
    versionRepo,
    releaseRepo,
    executionRepo,
    datasetRepo,
    evalRepo,
    preconditionRepo,
    alertRepo,
    scheduleRepo,
    approvalRepo,
    sseHub,
    settingsResolver,
    invocationAgent,
    evalAgent,
    evolutionAgent,
    goalEvolver,
    compiler,
    planner,
    executor,
    manifestRepo,
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
    membershipRepo,
    webhookReceiver,
    chaosConfig,
    auditChain,
    apiKeyRepo,
    userRepo: new UserRepo(db),
    llmRouter: new LlmRouter(),
    repoDeps: {
      repoRepo,
      branchRepo,
      tagRepo,
    },
    contentsDeps: {
      repoRepo,
      branchRepo,
      repoStore,
    },
    commitDeps: {
      repoRepo,
      branchRepo,
      repoStore,
      commitRepo,
    },
    mrDeps: {
      repoRepo,
      branchRepo,
      mrRepo,
    },
    signingDeps: {
      repoRepo,
      commitRepo,
      signingKeyRepo,
    },
    evalSuiteDeps: {
      suiteRepo: evalSuiteRepo,
      humanReviewRepo,
    },
    vaultDeps: {
      vaultRepo,
      orgExportService,
      costRollupRepo,
      kms: vaultRepo.kms,
      adminOnly: (request: unknown) => {
        const ctx = request as { orgContext?: { role?: string } } | undefined;
        return ctx?.orgContext?.role === 'admin';
      },
    },
    retentionDeps: (() => {
      const adminOnly = (request: unknown): boolean => {
        const ctx = request as { orgContext?: { role?: string } } | undefined;
        return ctx?.orgContext?.role === 'admin';
      };
      return { sweeper: retention, adminOnly };
    })(),
    redteamDeps: {
      redteamRepo,
      adminOnly: (request: unknown) => {
        const ctx = request as { orgContext?: { role?: string } } | undefined;
        return ctx?.orgContext?.role === 'admin';
      },
    },
    experimentDeps: { experimentRepo },
    incidentDeps: { incidentRepo, actorId: () => 'system' },
    featureFlagRepo,
    traceRepo,
    orgSettingsDeps: {
      orgSettingsRepo,
      vaultRepo,
      adminOnly: (request: unknown) => {
        const ctx = request as { orgContext?: { role?: string } } | undefined;
        return ctx?.orgContext?.role === 'admin';
      },
    },
  });

  const scheduler = new Scheduler(scheduleRepo, sseHub);
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
