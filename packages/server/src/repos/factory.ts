import type Database from 'better-sqlite3';
import { WorkspaceRepo } from './workspace.js';
import { ProjectRepo } from './project.js';
import { RepoRepo } from './repo.js';
import { BranchRepo } from './branch.js';
import { TagRepo } from './tag.js';
import { RepoStore } from './repo-store.js';
import { CommitRepo } from './commit.js';
import { MergeRequestRepo } from './mr.js';
import { SigningKeyRepo } from './signing-key.js';
import { EvalSuiteRepo, HumanReviewRepo } from './eval-suite.js';
import { VaultRepo, LocalKms } from './vault.js';
import { OrgExportService, CostRollupRepo } from './vault-extras.js';
import { CostBudgetRepo } from './budget.js';
import { TraceRepo } from './trace.js';
import { TraceScoreRepo } from './trace-score.js';
import { UserAnalyticsRepo } from './user-analytics.js';
import { TeamRepo, SsoConfigRepo } from './team.js';
import { PromptScanRepo } from './prompt-scan.js';
import { RedteamRepo } from './redteam.js';
import { ExperimentRepo } from './experiment.js';
import { IncidentRepo } from './incident.js';
import { OrgSettingsRepo } from './org-settings.js';
import { FeatureFlagRepo } from './feature-flag.js';
import { CapabilityRepo } from './capability.js';
import { VersionRepo } from './version.js';
import { ReleaseRepo } from './release.js';
import { ExecutionRepo } from './execution.js';
import { DatasetRepo } from './dataset.js';
import { EvalRepo } from './eval.js';
import { PreconditionRepo } from './precondition.js';
import { AlertRepo } from './alert.js';
import { ScheduleRepo } from './schedule.js';
import { ApprovalRepo } from './approval.js';
import { ApiKeyRepo } from './api-key.js';
import { UserRepo } from './user.js';
import { SystemConfigRepo } from './system-config.js';
import { ManifestRepo } from './manifest.js';
import { MembershipRepo } from './org.js';
import { WebhookRepo } from './webhook.js';
import { IdempotencyRepo } from './idempotency.js';

/**
 * Bundle of every repo in the server. Built once from a
 * Database handle and passed to the route registrar via the
 * dependency bag.
 */
export interface Repos {
  workspace: WorkspaceRepo;
  project: ProjectRepo;
  repo: RepoRepo;
  branch: BranchRepo;
  tag: TagRepo;
  repoStore: RepoStore;
  commit: CommitRepo;
  mergeRequest: MergeRequestRepo;
  signingKey: SigningKeyRepo;
  evalSuite: EvalSuiteRepo;
  humanReview: HumanReviewRepo;
  vault: VaultRepo;
  orgExport: OrgExportService;
  costRollup: CostRollupRepo;
  budget: CostBudgetRepo;
  trace: TraceRepo;
  traceScore: TraceScoreRepo;
  userAnalytics: UserAnalyticsRepo;
  team: TeamRepo;
  ssoConfig: SsoConfigRepo;
  promptScan: PromptScanRepo;
  redteam: RedteamRepo;
  experiment: ExperimentRepo;
  incident: IncidentRepo;
  orgSettings: OrgSettingsRepo;
  featureFlag: FeatureFlagRepo;
  capability: CapabilityRepo;
  version: VersionRepo;
  release: ReleaseRepo;
  execution: ExecutionRepo;
  dataset: DatasetRepo;
  eval: EvalRepo;
  precondition: PreconditionRepo;
  alert: AlertRepo;
  schedule: ScheduleRepo;
  approval: ApprovalRepo;
  apiKey: ApiKeyRepo;
  user: UserRepo;
  systemConfig: SystemConfigRepo;
  manifest: ManifestRepo;
  membership: MembershipRepo;
  webhook: WebhookRepo;
  idempotency: IdempotencyRepo;
}

/**
 * Build every repo from a single Database handle.
 *
 * Replaces the 41 manual `new XRepo(db)` calls that used to
 * live in the composition root. Adding a new repo means
 * adding one line here plus a key in the `Repos` interface —
 * no edits to `src/index.ts`.
 */
export function buildRepos(db: Database.Database): Repos {
  const vault = new VaultRepo(db, new LocalKms(db));
  return {
    workspace: new WorkspaceRepo(db),
    project: new ProjectRepo(db),
    repo: new RepoRepo(db),
    branch: new BranchRepo(db),
    tag: new TagRepo(db),
    repoStore: new RepoStore(db),
    commit: new CommitRepo(db),
    mergeRequest: new MergeRequestRepo(db),
    signingKey: new SigningKeyRepo(db),
    evalSuite: new EvalSuiteRepo(db),
    humanReview: new HumanReviewRepo(db),
    vault,
    orgExport: new OrgExportService(db, vault),
    costRollup: new CostRollupRepo(db),
    budget: new CostBudgetRepo(db),
    trace: new TraceRepo(db),
    traceScore: new TraceScoreRepo(db),
    userAnalytics: new UserAnalyticsRepo(db),
    team: new TeamRepo(db),
    ssoConfig: new SsoConfigRepo(db),
    promptScan: new PromptScanRepo(db),
    redteam: new RedteamRepo(db),
    experiment: new ExperimentRepo(db),
    incident: new IncidentRepo(db),
    orgSettings: new OrgSettingsRepo(db),
    featureFlag: new FeatureFlagRepo(db),
    capability: new CapabilityRepo(db),
    version: new VersionRepo(db),
    release: new ReleaseRepo(db),
    execution: new ExecutionRepo(db),
    dataset: new DatasetRepo(db),
    eval: new EvalRepo(db),
    precondition: new PreconditionRepo(db),
    alert: new AlertRepo(db),
    schedule: new ScheduleRepo(db),
    approval: new ApprovalRepo(db),
    apiKey: new ApiKeyRepo(db),
    user: new UserRepo(db),
    systemConfig: new SystemConfigRepo(db),
    manifest: new ManifestRepo(db),
    membership: new MembershipRepo(db),
    webhook: new WebhookRepo(db),
    idempotency: new IdempotencyRepo(db),
  };
}

// Re-export the classes for callers that still want to
// import them individually (tests, ad-hoc migrations).
export { WorkspaceRepo } from './workspace.js';
export { ProjectRepo } from './project.js';
export { CapabilityRepo } from './capability.js';
export { VersionRepo } from './version.js';
export { ReleaseRepo } from './release.js';
export { ExecutionRepo } from './execution.js';
export { DatasetRepo } from './dataset.js';
export { EvalRepo } from './eval.js';
export { PreconditionRepo } from './precondition.js';
export { AlertRepo } from './alert.js';
export { ScheduleRepo } from './schedule.js';
export { ApiKeyRepo } from './api-key.js';
export { WebhookRepo } from './webhook.js';
export { FeatureFlagRepo } from './feature-flag.js';
export { ApprovalRepo } from './approval.js';
export { SystemConfigRepo } from './system-config.js';
export { IdempotencyRepo } from './idempotency.js';
export { ManifestRepo, computeManifestHash } from './manifest.js';
export type { CutoverReport } from './manifest.js';
export { OrgRepo, TeamRepo, MembershipRepo } from './org.js';
