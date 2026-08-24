import type { FastifyInstance } from 'fastify';
import { registerWorkspaceRoutes } from './workspace.js';
import { registerProjectRoutes } from './project.js';
import { registerCapabilityRoutes } from './capability.js';
import { registerVersionRoutes } from './version.js';
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
import { registerGoalEvolveRoutes } from './goal-evolve.js';
import { registerManifestApprovalRoutes } from './manifest-approval.js';
import { registerGoalObservabilityRoutes } from './goals.js';
import { registerSessionRoutes } from './sessions.js';
import { registerSnapshotRoutes } from './snapshots.js';
import { registerManifestHashRoutes } from './manifest-hash.js';
import { registerOrgTeamRoutes } from './org-team.js';
import { registerWebhookRoutes } from './webhooks-incoming.js';
import { registerWebhookCrudRoutes } from './webhooks-crud.js';
import { OrgRepo, TeamRepo } from '../repos/org.js';
import { WebhookReceiver } from '../webhooks/receiver.js';
import { registerChaosRoutes } from './chaos.js';
import { AuditChain } from '../audit/chain.js';
import { registerAuditRoutes } from './audit.js';
import { registerUserRoutes } from './users.js';
import { registerApiKeyRoutes } from './api-keys.js';
import { registerBootstrapRoutes } from './bootstrap.js';
import type { LlmRouter } from '../llm/router.js';
import { registerRepoRoutes, type RepoDeps } from './repo.js';
import { registerContentsRoutes, type ContentsDeps } from './contents.js';
import { registerCommitRoutes, type CommitDeps } from './commits.js';
import { registerMergeRequestRoutes, type MRDeps } from './mr.js';
import { registerSigningRoutes, type SigningDeps } from './signing.js';
import { registerEvalSuiteRoutes, type EvalSuiteRouteDeps } from './eval-suite.js';
import { registerVaultRoutes, type VaultRouteDeps } from './vault.js';
import { registerOpenApiRoutes } from '../openapi.js';
import { registerRetentionRoutes, type RetentionRouteDeps } from './retention.js';
import { registerRedteamRoutes, type RedteamDeps } from './redteam.js';
import { registerExperimentRoutes, type ExperimentDeps } from './experiment.js';
import { registerIncidentRoutes, type IncidentDeps } from './incident.js';
import { registerOrgSettingsRoutes, type OrgSettingsRouteDeps } from './org-settings.js';
import type { UserRepo } from '../repos/user.js';
import type { ApiKeyRepo } from '../repos/api-key.js';

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
import type { GoalBasedEvolutionAgent } from '../agents/evolution/goal-evolver.js';
import type { ChaosConfig } from '../hardening/chaos.js';
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
  goalEvolver: GoalBasedEvolutionAgent;
  getActiveGoals: () => Array<{ manifestHash: string; bestScore: number; iterations: number; lastUpdated: string }>;
  sessionStore: import('../sessions/store.js').SessionStore;
  snapshotStore: import('../snapshots/store.js').SnapshotStore;
  getAgent: (id: string) => import('@strands-agents/sdk').Agent | null;
  membershipRepo: import('../repos/org.js').MembershipRepo;
  webhookReceiver: WebhookReceiver;
  chaosConfig?: ChaosConfig;
  auditChain: AuditChain;
  apiKeyRepo: ApiKeyRepo;
  userRepo: UserRepo;
  llmRouter: LlmRouter;
  repoDeps: RepoDeps;
  contentsDeps: ContentsDeps;
  commitDeps: CommitDeps;
  mrDeps: MRDeps;
  signingDeps: SigningDeps;
  evalSuiteDeps: EvalSuiteRouteDeps;
  vaultDeps: VaultRouteDeps;
  retentionDeps: RetentionRouteDeps;
  redteamDeps: RedteamDeps;
  experimentDeps: ExperimentDeps;
  incidentDeps: IncidentDeps;
  orgSettingsDeps: OrgSettingsRouteDeps;
}

export async function registerRoutes(app: FastifyInstance, deps: AppDeps): Promise<void> {
  registerWorkspaceRoutes(app, deps.workspaceRepo);
  registerProjectRoutes(app, deps.projectRepo);
  registerCapabilityRoutes(app, deps.capabilityRepo);
  registerVersionRoutes(app, deps.versionRepo, deps.manifestRepo);
  registerReleaseRoutes(app, deps.releaseRepo, { manifestRepo: deps.manifestRepo, auditChain: deps.auditChain });
  registerExecutionRoutes(app, {
    executionRepo: deps.executionRepo,
    releaseRepo: deps.releaseRepo,
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
  registerGoalEvolveRoutes(app, { goalEvolver: deps.goalEvolver, manifestRepo: deps.manifestRepo });
  registerManifestApprovalRoutes(app, { manifestRepo: deps.manifestRepo, auditChain: deps.auditChain });
  registerGoalObservabilityRoutes(app, {
    goalEvolver: deps.goalEvolver,
    getActiveGoals: deps.getActiveGoals,
  });
  registerSessionRoutes(app, { store: deps.sessionStore });
  registerSnapshotRoutes(app, { store: deps.snapshotStore, getAgent: deps.getAgent });
  registerManifestHashRoutes(app, { manifestRepo: deps.manifestRepo });
  registerOrgTeamRoutes(app, {
    orgRepo: new OrgRepo(deps.db),
    teamRepo: new TeamRepo(deps.db),
    membershipRepo: deps.membershipRepo,
  });
  registerWebhookRoutes(app, { receiver: deps.webhookReceiver, executor: deps.executor, manifestRepo: deps.manifestRepo });
  registerWebhookCrudRoutes(app, { auditChain: deps.auditChain });
  registerAuditRoutes(app, { auditChain: deps.auditChain, db: deps.db });
  registerUserRoutes(app, { userRepo: deps.userRepo, auditChain: deps.auditChain });
  registerApiKeyRoutes(app, { apiKeyRepo: deps.apiKeyRepo, auditChain: deps.auditChain });
  registerBootstrapRoutes(app, {
    db: deps.db,
    userRepo: deps.userRepo,
    settingsResolver: deps.settingsResolver,
    llmRouter: deps.llmRouter,
  });

  registerRepoRoutes(app, deps.repoDeps);
  registerContentsRoutes(app, deps.contentsDeps);
  registerCommitRoutes(app, deps.commitDeps);
  registerMergeRequestRoutes(app, deps.mrDeps);
  registerSigningRoutes(app, deps.signingDeps);
  registerEvalSuiteRoutes(app, deps.evalSuiteDeps);
  registerVaultRoutes(app, deps.vaultDeps);
  registerOpenApiRoutes(app);
  registerRetentionRoutes(app, deps.retentionDeps);
  registerRedteamRoutes(app, deps.redteamDeps);
  registerExperimentRoutes(app, deps.experimentDeps);
  registerIncidentRoutes(app, deps.incidentDeps);
  registerOrgSettingsRoutes(app, deps.orgSettingsDeps);

  if (deps.chaosConfig) {
    registerChaosRoutes(app, {
      chaos: deps.chaosConfig,
      isAdmin: (request) => {
        const meta = request as unknown as Record<string, unknown>;
        const role = meta['userRole'];
        return role === 'admin';
      },
    });
  }
}
