import type { FastifyInstance } from 'fastify';
import { registerWorkspaceRoutes } from './workspace.js';
import { registerProjectRoutes } from './project.js';
import { registerCapabilityRoutes } from './capability.js';
import { registerVersionRoutes } from './version.js';
import { registerManifestRoutes } from './manifest.js';
import { registerReleaseRoutes } from './release.js';
import { registerExecutionRoutes } from './execution.js';
import { registerDatasetRoutes } from './dataset.js';
import { registerEvalRoutes } from './eval.js';
import { registerPreconditionRoutes } from './precondition.js';
import { registerAlertRoutes } from './alert.js';
import { registerScheduleRoutes } from './schedule.js';
import { registerSettingsRoutes } from './settings.js';
import { registerSseRoutes } from './sse.js';
import { registerSelfEvolveRoutes } from './self-evolve.js';
import { registerApprovalRoutes } from './approval.js';
import { registerCompilerRoutes } from './compiler.js';
import { registerHealthRoutes } from './health.js';
import { registerIdeaRoutes } from './idea.js';

import type { WorkspaceRepo } from '../repos/workspace.js';
import type { ProjectRepo } from '../repos/project.js';
import type { CapabilityRepo } from '../repos/capability.js';
import type { VersionRepo } from '../repos/version.js';
import type { ReleaseRepo } from '../repos/release.js';
import type { ExecutionRepo } from '../repos/execution.js';
import type { DatasetRepo } from '../repos/dataset.js';
import type { EvalRepo } from '../repos/eval.js';
import type { PreconditionRepo } from '../repos/precondition.js';
import type { AlertRepo } from '../repos/alert.js';
import type { ScheduleRepo } from '../repos/schedule.js';
import type { ApprovalRepo } from '../repos/approval.js';
import type { SseHub } from '../sse/hub.js';
import type { SettingsResolver } from '../settings/resolver.js';
import type { InvocationAgent } from '../agents/invocation.js';
import type { EvaluationAgent } from '../agents/evaluation/evaluation.js';
import type { EvolutionAgent } from '../agents/evolution/evolution.js';
import type { ReasoningCompiler } from '../agents/compiler/compiler.js';
import type { IdeaPlannerAgent } from '../agents/planner/index.js';
import type { ManifestGraphExecutor } from '../agents/executor/index.js';
import type { ManifestRepo } from '../repos/manifest.js';
import type Database from 'better-sqlite3';

export interface AppDeps {
  db: Database.Database;
  workspaceRepo: WorkspaceRepo;
  projectRepo: ProjectRepo;
  capabilityRepo: CapabilityRepo;
  versionRepo: VersionRepo;
  releaseRepo: ReleaseRepo;
  executionRepo: ExecutionRepo;
  datasetRepo: DatasetRepo;
  evalRepo: EvalRepo;
  preconditionRepo: PreconditionRepo;
  alertRepo: AlertRepo;
  scheduleRepo: ScheduleRepo;
  approvalRepo: ApprovalRepo;
  sseHub: SseHub;
  settingsResolver: SettingsResolver;
  invocationAgent: InvocationAgent;
  evalAgent: EvaluationAgent;
  evolutionAgent: EvolutionAgent;
  compiler: ReasoningCompiler;
  planner: IdeaPlannerAgent;
  executor: ManifestGraphExecutor;
  manifestRepo: ManifestRepo;
}

export async function registerRoutes(app: FastifyInstance, deps: AppDeps): Promise<void> {
  registerWorkspaceRoutes(app, deps.workspaceRepo);
  registerProjectRoutes(app, deps.projectRepo);
  registerCapabilityRoutes(app, deps.capabilityRepo);
  registerVersionRoutes(app, deps.versionRepo);
  registerManifestRoutes(app, deps.versionRepo);
  registerReleaseRoutes(app, deps.releaseRepo);
  registerExecutionRoutes(app, {
    executionRepo: deps.executionRepo,
    manifestRepo: deps.manifestRepo,
    executor: deps.executor,
  });
  registerDatasetRoutes(app, deps.datasetRepo);
  registerEvalRoutes(app, deps.evalRepo, deps.evalAgent);
  registerPreconditionRoutes(app, deps.preconditionRepo);
  registerAlertRoutes(app, deps.alertRepo);
  registerScheduleRoutes(app, deps.scheduleRepo);
  registerSettingsRoutes(app, deps.settingsResolver);
  registerSseRoutes(app, deps.sseHub);
  registerSelfEvolveRoutes(app, deps.evolutionAgent, deps.capabilityRepo, deps.evalRepo);
  registerApprovalRoutes(app, deps.approvalRepo);
  registerCompilerRoutes(app, deps.compiler);
  registerHealthRoutes(app, deps.db);
  registerIdeaRoutes(app, { planner: deps.planner });
}
