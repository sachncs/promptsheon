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
import { SystemConfigRepo } from './repos/system-config.js';
import { ManifestRepo } from './repos/manifest.js';
import { IdeaPlannerAgent } from './agents/planner/index.js';
import { ManifestGraphExecutor } from './agents/executor/index.js';
import { setupObservability } from './observability/setup.js';

async function main() {
  const config = loadConfig();
  const db = createConnection(config);
  runMigrations(db);
  const auditChain = new AuditChain(db);

  const app = Fastify({ logger: true });

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
  const executor = new ManifestGraphExecutor({ config, hub: sseHub });
  const goalEvolver = new GoalBasedEvolutionAgent({ config, hub: sseHub, executor, cas: casStore });

  app.addHook('preHandler', authMiddleware(config, apiKeyRepo));

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
