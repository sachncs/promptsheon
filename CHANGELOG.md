# Changelog

All notable changes to Promptsheon are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.4.0] - 2026-08-20

### Added
- **Repositories** — a workspace-scoped Git-native store: branches, tags, content-addressed commits, merge requests, file content tree. Auto-creates the default branch on repo creation.
- **Operator signing keys** — per-org ed25519 public-key registry. The platform records a detached signature on commits; verification re-derives the canonical payload.
- **6-state release machine** — `draft → review → approved → canary → active → rolled_back` with env overlays, rule-based canary routing, and a transition log.
- **Merge requests with maker-checker** — author ≠ approver enforced; inline comments; approvals/decisions surfaced to the audit chain.
- **Vault + KMS** — AES-256-GCM at rest with a swappable KMS. The `Kms` interface lets production deployments swap in `AwsSecretsManagerKms`, `HashiCorpVaultKms`, or `DopplerKms` without touching the read path.
- **Org settings** — per-org residency (`local | us | eu | ap | sa | me | af`), `encryption_at_rest` toggle, `kms_provider` selection.
- **Eval harness** — versioned suites, four deterministic graders (regex, schema, tool-call, transcript), `passAtK` and `pass^k`, calibration via Cohen's κ and Krippendorff's α, human-review queue.
- **Eval gate** — `POST /api/repos/:id/eval-gate` is callable from any external CI; returns `{ ok, score, regressions, suites }`.
- **Red-team packs** — seeded corpora for prompt-injection, jailbreak, and data-exfiltration with resistance scoring and run history.
- **Experiment tracking** — A/B-style variants per release with side-by-side compare; assignments record per-case outcomes.
- **Incident-driven eval case proposals** — operators or background subscribers can flag a failed transcript; reviewers accept or reject; accepted cases commit into the suite.
- **Pareto-frontier meta learner** — across experiment variants; `pickCheapestFrontier(threshold)` selects the cheapest frontier point above a target pass-rate.
- **Retention cron** — `RetentionSweeper` runs on server start and every 6 h; prunes `eval_results` + `human_review_queue` past the configured horizon; the audit chain is never swept.
- **OpenAPI 3.1 emitter** — `/api/openapi.json` documents 27 operations across repos, contents, commits, MRs, signing, eval suites, and org settings; new routes call `registerRouteDoc({...})`.
- **Public docs site** at `/docs/*` — quickstart, repos, releases, evals, vault, signing, retention, CLI, SDK. Brand-matched dark theme.
- **CLI (`@promptsheon/cli`)** — `login`, `repos list`, `eval gate <repoId>`, `release approve <id>`. Auth via `PROMPTSHEON_API_KEY` bearer.
- **SDK (`@promptsheon/sdk`)** — typed fetch wrapper: `PromptsheonClient` for repos, branches, contents, commits, MRs, signing keys, sign/verify, eval suites, gate.
- **Front-end surfaces** for the new backend: `/app/repos`, `/app/eval/suites`, `/app/vault`, `/app/search`, `/app/admin/cost` (with bar chart by capability and by day), `/app/api-keys` (live issuance + revoke), file-content viewer in the repo tree.
- **Brand primitives** — `BarChart`, refined `Drawer`, and per-surface empty states with the same dark/cool-accent system.

### Changed
- **Routes (sidebar)** — added Repositories, Merge requests, Search, Suites, Vault, Cost & analytics, Signing keys; kept legacy Workspaces, Releases, Audit, etc.
- **Auth middleware** — Bearer tokens are now the canonical path; `X-User-Id`/`X-Org-Id` remains as the dev-tools fallback.
- **Releases** — `create()` now writes `draft` (was `pending`); `rollbackAtomically()` transitions to `rolled_back` (was `superseded`).

### Fixed
- `MembershipRepo.findOrgMembers()` now maps snake_case columns to camelCase, so cross-org requests no longer return 403 NOT_ORG_MEMBER.
- Migration 031 derives `repo_id` from `c.project_id` and uses `c.project_id` (not `c.repo_id` which doesn't exist on `capabilities`).
- `CostRollupRepo.rollupsForOrg()` joins through `projects` to `workspaces.org_id`; ingestion + read round-trip verified.
- `fingerprintSpki` returns the full 64-char hex to match what `openssl`/`ssh-keygen` report.
- `audit_log` writes no longer blow up with a missing `system` user; tests that drive the helper path seed it.

### Security
- Vault rotation mints a fresh key and re-encrypts every secret under it; old keys remain readable for historical verification.
- Org-level `encryption_at_rest` toggle + `kms_provider` allow per-tenant policies without code changes.
