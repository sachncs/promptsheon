// Package backend errors are centralised here so callers can check
// errors.Is without importing every sub-package. Each error carries
// a domain prefix for clear diagnostics.
package backend

import "errors"

// ── Approval ────────────────────────────────────────────────────────
var (
	ErrorApprovalDuplicateIdentity = errors.New("approval: duplicate voter")
	ErrorApprovalCreatorVoted      = errors.New("approval: creator voted on own release (separation of duties)")
	ErrorApprovalQuorumNotMet      = errors.New("approval: quorum not yet satisfied")
	ErrorApprovalUnknownDecision   = errors.New("approval: unknown decision")
	ErrorApprovalNotFound          = errors.New("approval: not found")
)

// ── Budget ──────────────────────────────────────────────────────────
var (
	ErrorBudgetCapNotPositive = errors.New("budget: cap must be > 0")
	ErrorBudgetCapExceeded    = errors.New("budget: cap exceeded")
)

// ── Capability ──────────────────────────────────────────────────────
var (
	ErrorCapabilityInvalidBlastRadius = errors.New("capability: invalid blast radius")
	ErrorCapabilityEmptyContract      = errors.New("capability: empty contract")
	ErrorCapabilityInheritanceTooDeep = errors.New("capability: inheritance chain too deep")
	ErrorCapabilityEmptyManifest      = errors.New("manifest is empty")
)

// ── Context ─────────────────────────────────────────────────────────
var ErrorContextBudgetExhausted = errors.New("context: token budget exhausted after truncation")

// ── Election ────────────────────────────────────────────────────────
var ErrorElectionNotLeader = errors.New("election: not the leader")

// ── Eval ────────────────────────────────────────────────────────────
var ErrorEvalUnsupportedSchema = errors.New("json_schema: schema uses unsupported keywords")

// ── EventBus ────────────────────────────────────────────────────────
var ErrorEventBusAlreadyCanceled = errors.New("eventbus: subscription canceled")

// ── Executor ────────────────────────────────────────────────────────
var ErrorExecutorProviderMissing = errors.New("executor: provider missing")

// ── Harness ─────────────────────────────────────────────────────────
var ErrorHarnessPreconditionFailed = errors.New("harness: precondition failed")

// ── Lineage ─────────────────────────────────────────────────────────
var (
	ErrorLineageUnknownSource          = errors.New("lineage: unknown source")
	ErrorLineageSelfReference          = errors.New("lineage: child cannot be its own parent")
	ErrorLineageDuplicateEdge          = errors.New("lineage: edge already exists")
	ErrorLineageInconsistentCapability = errors.New("lineage: edge references a different capability")
)

// ── MCP ─────────────────────────────────────────────────────────────
var (
	ErrorMCPEmptyName = errors.New("mcplist: empty name")
	ErrorMCPBadName   = errors.New("mcplist: bad name")
	ErrorMCPBadURL    = errors.New("mcplist: bad url")
	ErrorMCPUnknownName = errors.New("mcplist: unknown name")
)

// ── Plugin Manifest ─────────────────────────────────────────────────
var (
	ErrorManifestEmpty  = errors.New("manifest: no plugins")
	ErrorManifestBadName = errors.New("manifest: bad plugin name")
	ErrorManifestBadUDS = errors.New("manifest: UDS path must be under /tmp/promptsheon/")
)

// ── Quota ───────────────────────────────────────────────────────────
var (
	ErrorQuotaLimitNotPositive = errors.New("quota: limit must be > 0")
	ErrorQuotaOverLimit        = errors.New("quota: over limit")
)

// ── Reasoning ───────────────────────────────────────────────────────
var (
	ErrorReasoningNoMatch            = errors.New("reasoning: no capability matches intent")
	ErrorReasoningConstraintViolation = errors.New("reasoning: candidates violate constraints")
)

// ── Recommendation ──────────────────────────────────────────────────
var (
	ErrorRecommendationUnknownOutcome = errors.New("decision: unknown outcome")
	ErrorRecommendationNotFound       = errors.New("recommendation: not found")
)

// ── Release ─────────────────────────────────────────────────────────
var (
	ErrorReleaseNotFound           = errors.New("release: not found")
	ErrorReleaseNotPending         = errors.New("release: transition requires Pending status")
	ErrorReleaseUnknownEnvironment = errors.New("release: unknown environment")
	ErrorReleaseNotActive          = errors.New("release: not active")
)

// ── Schedule ────────────────────────────────────────────────────────
var ErrorScheduleInvalidCron = errors.New("schedule: invalid cron expression")

// ── Store ───────────────────────────────────────────────────────────
var (
	ErrorStoreNotFound          = errors.New("not found")
	ErrorStoreConflict          = errors.New("conflict")
	ErrorStoreIdempotencyMiss   = errors.New("idempotency: miss")
)

// ── Vault ───────────────────────────────────────────────────────────
var (
	ErrorVaultStopped          = errors.New("vault: stopped")
	ErrorVaultUnknownSecret    = errors.New("vault: unknown secret")
	ErrorVaultKeyUnavailable   = errors.New("vault: master key unavailable")
	ErrorVaultKMSClientRequired = errors.New("kmsbyok: KMSClient required (production); tests must set AllowTestDouble")
)
