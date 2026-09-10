# Changelog

All notable changes to Promptsheon are documented in this file.
The format follows [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Docker packaging** — multi-stage `Dockerfile` that compiles
  the shared + server + frontend workspaces into a single
  non-root container. The runtime image is based on
  `node:26-alpine`, exposes `:8080`, ships a healthcheck
  against `/api/health`, and stores the SQLite + CAS on a
  `/data` volume. The CI workflow's docker job now builds
  the image on every push to master and runs a smoke check
  (`node --check` + a live `/api/health` curl). README
  Installation > Option 3 documents the `docker build` /
  `docker run` flow.

- **Cedar authorization (IN-0)** — `packages/server/src/policy/`
  ships the single source of truth for every authorization
  decision in the platform: `authorizer.ts` (loads + caches the
  policy + schema), `principal.ts` (extracts the principal from
  request headers, including a `Role` path for tests + replay
  tooling), `gate.ts` (Fastify preHandler that calls the
  authorizer), `regression.ts` + `eval-cli.ts` (the
  `pnpm policy:eval` script that fails CI on any drift in
  `policies/promptsheon.cedar`). The boot sequence
  (`src/index.ts`) installs the singleton authorizer at boot.
  9 authorizer + 15 principal + 2 regression + 4 gate = 30 new
  vitest cases. `pnpm policy:eval` exits 0 with 14/14 cases
  passing.

- **Identity & SPIFFE Verifiable Identity Documents (AG-4)** —
  `packages/server/src/identity/` ships the Agent-identity
  primitives. `apikey.ts` mints and verifies opaque API keys,
  `svid.ts` signs and verifies ed25519 SVIDs, and `verify.ts`
  unifies the two into a single verifier. The auth middleware
  (`packages/server/src/middleware/auth.ts`) now accepts
  `Authorization: SVID <token>` and stamps the request with a
  Cedar `Agent` principal (id + org + classification). The
  SVID public key is loaded from `PROMPTSHEON_SVID_PUBLIC_KEY_PEM`;
  per-org signing keys wire in through `SigningKeyRepo` once
  the per-org key rotation lands. Migration 051 introduces the
  identity tables.

### Changed
- `packages/server/src/policy/authorizer.ts` resolves the
  default policy path via `import.meta.url` so the boot
  sequence works from any cwd (was `resolve('packages/server/...')`
  which silently broke when launched from a different
  directory).

## [v0.4.2] - 2026-08-25

The end-of-audit release. Closes every issue surfaced by the
production audit (initial pass + R1–R6 follow-up). Maker-checker
flow now actually works end-to-end: bootstrap → workspace →
project → capability → version → release → approval → activation
gate, fully verified by curl + the Playwright tier suite.

### Added
- **`/api/webhooks` CRUD** — org-scoped store with
  `label, url, events, active`; GET/POST/PUT/DELETE handlers
  and audit entries on every mutation.
- **`/api/feature-flags` CRUD** — backed by an extended
  `FeatureFlagRepo`; migration 042 adds a JSON `value` column
  so flags can carry richer payloads than a boolean toggle.
- **`GET /api/capability-versions/:versionId/manifest`** — the
  drilldown for `/app/manifests/[versionId]`; returns the
  parsed manifest object plus metadata, size, and capability
  linkage.
- **`PUT /api/preconditions/:id`** — toggle `enabled` and
  other fields. Migration 043 adds `updated_at`.
- **`/api/capabilities/:id/self-evolve` (state) and
  `/api/capabilities/:id/self-evolve/run` (cycle)** —
  capability-scoped shortcuts that mirror the legacy paths.
- **`POST /api/invoke` alias** — accepts `{capabilityVersionId,
  inputs, …}`, resolves the version's `manifestHash` and
  forwards to the canonical execute path.
- **`GET /api/goals/:hash` drilldown** — returns real
  iteration history + snapshots persisted on
  `GoalEvolutionState`. Frontend `/app/goals/[hash]` renders
  Overview / Iteration history / Snapshots tabs.
- **`/app/manifests/[versionId]`** — real 3-tab page
  (Overview, Approvals, History) instead of the
  "Manifest not found" empty state.
- **`/app/repos` "New repository" dialog** — typed Zod form.
- **`/app/releases` "New release" dialog** — react-hook-form
  + Zod.
- **`requireAdmin()` middleware** — applied to 14 management
  routes.
- **`new-repository-dialog.tsx` and `new-release-dialog.tsx`**
  — typed `react-hook-form + zodResolver` form primitives.
- **`DataTable<T>`** — generic with `Column<T>[]`,
  `<caption>`, `scope="col"`, sortable headers,
  focus-visible ring.
- **`unwrapList<T>` / `unwrapFirst<T>` helpers** in
  `lib/api.ts` — normalize the three backend list response
  shapes (`Array` / `{items,total}` / `{<plural>}`).
- **Migrations 042 / 043 / 044** — feature-flag value column,
  preconditions updated_at, manifest_dag drafts column.
- **Frontend lint config** — `frontend/.eslintrc.json`
  extending `next/core-web-vitals`, with `as unknown as <T>`
  banned via `no-restricted-syntax`.

### Changed
- **`AGENTS.md`** — corrected stack claim from
  "React 19 + Vite + React Router v7" to "Next.js 16 App
  Router + TanStack Query + axios + shadcn/ui".
- **`/api/workspaces` and friends** — pagination now
  coerces `?page=1` (string) to a number at the schema
  level, so the frontend's request shape passes without 422.
- **`.env.example`** — documents `PROMPTSHEON_WEBHOOK_SECRET`
  and the `PROMPTSHEON_ALLOW_SYSTEM_ACTOR` toggle.
- **`workspaceApi.list/get/create/update`** — return the
  same `{data: T}` shape the rest of the codebase expects
  from axios callers.

### Fixed
- **Maker-checker gate no longer silently passes** —
  `BaseRepo.findById` camelizes snake_case columns before
  returning, so the gate correctly reads
  `release.createdBy`. Self-approval is blocked again.
- **DAG editor Save** — `mergeDraftManifest()` synthesises
  safe defaults for every required `Manifest` field when the
  editor saves a partial draft.
- **Release-route `require('node:crypto')`** — replaced with
  a static ESM `import { createHash } from 'node:crypto'`.
- **Manifest-hash canonicalization** — both version-create
  and release-create use raw-string SHA-256.
- **Org-context-middleware key mismatch** —
  `routes/webhooks-crud.ts` now reads `organizationId` (the
  key the middleware sets), not `orgId`.
- **System-actor bypass disabled in production** — the
  always-on `X-User-Id: api` → admin context fallback is
  scoped to non-production via
  `PROMPTSHEON_ALLOW_SYSTEM_ACTOR`.
- **Webhook secret no default in production** — `index.ts`
  throws on missing `PROMPTSHEON_WEBHOOK_SECRET` when
  `PROMPTSHEON_NODE_ENV=production`.
- **LLM-key echo to `process.env` removed** — `bootstrap.ts`
  no longer mirrors user-supplied LLM keys via `mirrorEnv()`.
- **Admin gate on 14 management routes** — `/api/users`,
  `/api/api-keys`, `/api/settings/:key`, `/api/webhooks`,
  `/api/feature-flags`. Non-admin callers get 403.
- **Role escalation cap on api-keys POST** — even an admin
  cannot mint a key with a role higher than their own;
  non-admins are silently demoted to `reader`.
- **`/api/audit/verify` and `/api/audit/state` whitelisted
  as public** — the docs link opens in a new tab and the
  browser cannot attach session headers.
- **Frontend `subscribeSSE` cancel** — captures the active
  source in a closure and clears the reconnect timer.
- **`/app/goals`** — was using raw `fetch('/api/goals')`,
  bypassing the axios interceptor. Switched to
  `client.get('/goals')`.
- **`/docs` search** — pruned 4 dead links; added `/docs` to
  the index.
- **`docs/sdk` page** — no longer reads
  `process.env.PROMPTSHEON_API_KEY!`. Replaced with a
  literal placeholder.
- **Frontend `data-table` snake/camel** — `/app/projects` and
  several other pages no longer needed `as unknown as X`
  casts.
- **Empty `manifest_dag` registrations on editor Save** — the
  manifest now gets upserted with the correct hash on every
  version/release save.

### Security
- **12 new admin-gated routes** — see above.
- **Snake-case → camelCase mapper in `BaseRepo.findById`** —
  fixes a class of latent bugs where `update()` methods bound
  undefined into NOT NULL columns.
- **Audit chain verify + state publicly callable** —
  `PUBLIC_PATHS` in `auth.ts` includes both endpoints.

### Tests
- **Server: 322 → 619 vitest cases, 0 fail.** New spec files
  cover webhooks, feature-flags, audit-verify public,
  approval reconcile, self-evolve reconcile, compiler prompt
  legacy body, preconditions update, version manifest,
  admin gating, repo routes, identity, SVID, authorizer,
  principal extraction, policy regression, gateway,
  scheduler, retention, firewall.
- **Shared: 4 files / 36 vitest cases, 0 fail.**
- **Frontend Playwright tier suite rewritten** against the
  new contracts — 10 spec files covering 41 routes,
  forms, manifest detail, approvals, admin gating.

## [v0.4.1] - 2026-08-23

Hotfix release between v0.4.0 and v0.4.2. Re-shipped the audit
gate that had been removed in v0.4.0, fixed two migration
runner bugs that fired on the new deployments, and added
`requireAdmin()` (the in-tree seed of the v0.4.2 admin gate).

### Fixed
- Audit gate re-introduced on `/api/audit/state` so
  unauthenticated reads of the chain state return 401.
- Migration runner no longer crashes on an empty
  `migrations` table; it now seeds with `version = 0`.
- `BaseRepo.findById` returns `null` instead of throwing on
  a missing row, matching the typed `T | null` contract.

## [v0.4.0] - 2026-08-20

### Added
- **Repositories** — a workspace-scoped Git-native store:
  branches, tags, content-addressed commits, merge requests,
  file content tree. Auto-creates the default branch on repo
  creation.
- **Operator signing keys** — per-org ed25519 public-key
  registry. The platform records a detached signature on
  commits; verification re-derives the canonical payload.
- **6-state release machine** —
  `draft → review → approved → canary → active → rolled_back`
  with env overlays, rule-based canary routing, and a
  transition log.
- **Merge requests with maker-checker** — author ≠ approver
  enforced; inline comments; approvals/decisions surfaced to
  the audit chain.
- **Vault + KMS** — AES-256-GCM at rest with a swappable
  KMS. The `Kms` interface lets production deployments swap
  in `AwsSecretsManagerKms`, `HashiCorpVaultKms`, or
  `DopplerKms` without touching the read path.
- **Org settings** — per-org residency
  (`local | us | eu | ap | sa | me | af`),
  `encryption_at_rest` toggle, `kms_provider` selection.
- **Eval harness** — versioned suites, four deterministic
  graders (regex, schema, tool-call, transcript), `passAtK`
  and `pass^k`, calibration via Cohen's κ and Krippendorff's
  α, human-review queue.
- **Eval gate** — `POST /api/repos/:id/eval-gate` is
  callable from any external CI; returns
  `{ ok, score, regressions, suites }`.
- **Red-team packs** — seeded corpora for prompt-injection,
  jailbreak, and data-exfiltration with resistance scoring
  and run history.
- **Experiment tracking** — A/B-style variants per release
  with side-by-side compare; assignments record per-case
  outcomes.
- **Incident-driven eval case proposals** — operators or
  background subscribers can flag a failed transcript;
  reviewers accept or reject; accepted cases commit into
  the suite.
- **Pareto-frontier meta learner** — across experiment
  variants; `pickCheapestFrontier(threshold)` selects the
  cheapest frontier point above a target pass-rate.
- **Retention cron** — `RetentionSweeper` runs on server
  start and every 6 h; prunes `eval_results` +
  `human_review_queue` past the configured horizon; the
  audit chain is never swept.
- **OpenAPI 3.1 emitter** — `/api/openapi.json` documents 27
  operations across repos, contents, commits, MRs, signing,
  eval suites, and org settings.
- **Public docs site** at `/docs/*` — quickstart, repos,
  releases, evals, vault, signing, retention, CLI, SDK.
  Brand-matched dark theme.
- **CLI (`@promptsheon/cli`)** — `login`, `repos list`,
  `eval gate <repoId>`, `release approve <id>`. Auth via
  `PROMPTSHEON_API_KEY` bearer.
- **SDK (`@promptsheon/sdk`)** — typed fetch wrapper:
  `PromptsheonClient` for repos, branches, contents,
  commits, MRs, signing keys, sign/verify, eval suites,
  gate.
- **Front-end surfaces** for the new backend: `/app/repos`,
  `/app/eval/suites`, `/app/vault`, `/app/search`,
  `/app/admin/cost`, `/app/api-keys`, file-content viewer
  in the repo tree.
- **Brand primitives** — `BarChart`, refined `Drawer`, and
  per-surface empty states with the same dark/cool-accent
  system.

### Changed
- **Routes (sidebar)** — added Repositories, Merge
  requests, Search, Suites, Vault, Cost & analytics, Signing
  keys; kept legacy Workspaces, Releases, Audit, etc.
- **Auth middleware** — Bearer tokens are now the canonical
  path; `X-User-Id`/`X-Org-Id` remains as the dev-tools
  fallback.
- **Releases** — `create()` now writes `draft` (was
  `pending`); `rollbackAtomically()` transitions to
  `rolled_back` (was `superseded`).

### Fixed
- `MembershipRepo.findOrgMembers()` now maps snake_case
  columns to camelCase, so cross-org requests no longer
  return 403 NOT_ORG_MEMBER.
- Migration 031 derives `repo_id` from `c.project_id` and
  uses `c.project_id` (not `c.repo_id` which does not exist
  on `capabilities`).
- `CostRollupRepo.rollupsForOrg()` joins through `projects`
  to `workspaces.org_id`; ingestion + read round-trip
  verified.
- `fingerprintSpki` returns the full 64-char hex to match
  what `openssl`/`ssh-keygen` report.
- `audit_log` writes no longer blow up with a missing
  `system` user; tests that drive the helper path seed it.

### Security
- Vault rotation mints a fresh key and re-encrypts every
  secret under it; old keys remain readable for historical
  verification.
- Org-level `encryption_at_rest` toggle + `kms_provider`
  allow per-tenant policies without code changes.

## [v0.3.x] - 2026-07

The 0.3.x line shipped the Fastify + Strands backend,
the Next.js frontend shell, the DAG editor prototype, and
the first-pass releases / canary / maker-checker loop.
Detailed release notes predate the adoption of
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
refer to the [closed-issue map](docs/MARKET-GAPS.md) and
git tag history for per-change attribution.

## [v0.2.0] - 2026-05

Initial self-hosted prompt-management platform. SQLite
storage, content-addressed store, basic audit chain,
single-org admin. Pre-Changelog.

## [v0.1.0] - 2026-03

First cut of the platform. Bootstrapping scripts,
filesystem-backed CAS, REST surface. Pre-Changelog.

---

The format of this file follows
[Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/).
Sections in each release use the canonical
`Added` / `Changed` / `Deprecated` / `Removed` / `Fixed`
/ `Security` ordering. Versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).
