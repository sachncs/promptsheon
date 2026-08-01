// Package errs — centralised sentinel errors. Each error carries a
// domain prefix for clear diagnostics. Naming convention is `Err*`
// (idiomatic Go); the previous `Error*` prefix was non-standard
// and renamed in PLAN-49 c2.8 (then a follow-up commit completed
// the remaining sentinels when pkg/promptsheon/ needed the Err*
// names to re-export).
package errs

import "errors"

// ── Approval ────────────────────────────────────────────────────────
var (
	ErrApprovalDuplicate = errors.New("approval: duplicate voter")
	ErrSelfVote          = errors.New("approval: creator voted on own release (separation of duties)")
	ErrQuorum            = errors.New("approval: quorum not yet satisfied")
	ErrApprovalUnknown   = errors.New("approval: unknown decision")
	ErrApprovalNotFound  = errors.New("approval: not found")
)

// ── Budget ──────────────────────────────────────────────────────────
var (
	ErrBudgetInvalid = errors.New("budget: cap must be > 0")
	ErrBudget        = errors.New("budget: cap exceeded")
)

// ── Capability ──────────────────────────────────────────────────────
var (
	ErrInvalidBlastRadius = errors.New("capability: invalid blast radius")
	ErrEmptyContract      = errors.New("capability: empty contract")
	ErrInheritanceTooDeep = errors.New("capability: inheritance chain too deep")
	ErrEmptyManifest      = errors.New("manifest is empty")
)

// ── Context ─────────────────────────────────────────────────────────
var ErrContextExhausted = errors.New("context: token budget exhausted after truncation")

// ── Election ────────────────────────────────────────────────────────
var ErrNotLeader = errors.New("election: not the leader")

// ── Eval ────────────────────────────────────────────────────────────
var ErrEvalUnsupportedSchema = errors.New("json_schema: schema uses unsupported keywords")

// ── EventBus ────────────────────────────────────────────────────────
var ErrEventBusCanceled = errors.New("eventbus: subscription canceled")

// ── Executor ────────────────────────────────────────────────────────
var ErrProviderMissing = errors.New("executor: provider missing")

// ── Harness ─────────────────────────────────────────────────────────
var ErrPrecondition = errors.New("harness: precondition failed")

// ── Lineage ─────────────────────────────────────────────────────────
var (
	ErrLineageUnknown      = errors.New("lineage: unknown source")
	ErrLineageSelfRef      = errors.New("lineage: child cannot be its own parent")
	ErrLineageDuplicate    = errors.New("lineage: edge already exists")
	ErrLineageInconsistent = errors.New("lineage: edge references a different capability")
)

// ── MCP ─────────────────────────────────────────────────────────────
var (
	ErrMCPEmptyName = errors.New("mcplist: empty name")
	ErrMCPBadName   = errors.New("mcplist: bad name")
	ErrMCPBadURL    = errors.New("mcplist: bad url")
	ErrMCPUnknown   = errors.New("mcplist: unknown name")
)

// ── Plugin Manifest ─────────────────────────────────────────────────
var (
	ErrManifestEmpty   = errors.New("manifest: no plugins")
	ErrManifestBadName = errors.New("manifest: bad plugin name")
	ErrManifestBadUDS  = errors.New("manifest: UDS path must be under /tmp/promptsheon/")
)

// ── Quota ───────────────────────────────────────────────────────────
var (
	ErrQuotaInvalid = errors.New("quota: limit must be > 0")
	ErrQuota        = errors.New("quota: over limit")
)

// ── Reasoning ───────────────────────────────────────────────────────
var (
	ErrReasoningNoMatch             = errors.New("reasoning: no capability matches intent")
	ErrReasoningConstraintViolation = errors.New("reasoning: candidates violate constraints")
)

// ── Recommendation ──────────────────────────────────────────────────
var (
	ErrRecommendationUnknown  = errors.New("decision: unknown outcome")
	ErrRecommendationNotFound = errors.New("recommendation: not found")
)

// ── Release ─────────────────────────────────────────────────────────
var (
	ErrReleaseNotFound           = errors.New("release: not found")
	ErrReleaseNotPending         = errors.New("release: transition requires Pending status")
	ErrReleaseUnknownEnvironment = errors.New("release: unknown environment")
	ErrReleaseNotActive          = errors.New("release: not active")
)

// ── Schedule ────────────────────────────────────────────────────────
var ErrInvalidCron = errors.New("schedule: invalid cron expression")

// ── Store ───────────────────────────────────────────────────────────
var (
	ErrStoreNotFound        = errors.New("not found")
	ErrStoreConflict        = errors.New("conflict")
	ErrStoreIdempotencyMiss = errors.New("idempotency: miss")
)

// ── Vault ───────────────────────────────────────────────────────────
var (
	ErrVaultStopped    = errors.New("vault: stopped")
	ErrVaultUnknown    = errors.New("vault: unknown secret")
	ErrVaultKeyUnavail = errors.New("vault: master key unavailable")
	ErrKMSClient       = errors.New("kmsbyok: KMSClient required (production); tests must set AllowTestDouble")
)
