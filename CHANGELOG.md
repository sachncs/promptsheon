# Changelog

All notable changes to Promptsheon are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **T3-1 time-travel debugging** (`/api/executions/:id/replay`).
  Re-runs any past execution with the same manifest, model,
  environment, and inputs; the new execution is linked to the
  original via `replay_of` and the original's `replay_count` is
  incremented. A per-node diff summary is returned alongside
  the new execution so callers can see what changed (the diff
  is the value-add because LLMs are not deterministic). A
  companion `GET /api/executions/:id/replays` lists every
  replay attempt with outcome + diff. Migration 049 adds the
  lineage columns + the `execution_replays` log table.
- **`Replay` button on `/app/executions/[id]`** — fires the
  endpoint and navigates to the replay's detail page; replays
  themselves show "Replay of …" in the header and link back to
  the original.
- **T3-5 prompt-security benchmark** — `docs/security/benchmark/dataset.json`
  with 53 curated cases across OWASP LLM01..LLM10 plus MIX/EDGE
  categories. Run `pnpm --filter @promptsheon/server bench:security`
  to execute the corpus against the scanner and emit
  `docs/security/benchmark/RESULTS.md`. The runner exits non-zero
  on any regression so a regex tweak can never silently weaken
  coverage.
- **T4 streamed completions over SSE** — `POST /api/executions`
  now detects `Accept: text/event-stream` and streams per-node
  events (`execution_start`, `node_start`, `node_complete`,
  `execution_complete`, …) followed by a terminal `done` frame.
  Buffered JSON mode remains the default for non-SSE clients.
  The streamer (`packages/server/src/sse/streamer.ts`) implements
  `SseClient` directly so it filters by executionId before
  forwarding. The frontend gains `executionApi.stream(...)` so
  callers can opt into the live token flow from the playground.
- **T4 on-prem RHEL deployment guide** — `docs/operations/air-gap-rhel.md`
  is the §7 deliverable: a step-by-step runbook for taking the
  offline installer tarball to an air-gapped host, covering
  pre-flight checks, FIPS-mode install, the FIPS gate's
  refuse-to-boot contract, upgrades, backups, DR, and the audit
  chain's "valid: false" failure mode.
- **T4 A/B statistical significance** — `packages/server/src/analysis/significance.ts`
  ships frequentist (two-proportion z-test, Wald CI for the
  difference) and Bayesian (beta-binomial Monte Carlo with 10k
  draws, 95% credible intervals) summaries. `ExperimentRepo.summarize(releaseId)`
  produces a typed `SignificanceReport` (per-variant stats,
  pairwise tests, ranking, winner verdict) and
  `GET /api/releases/:id/experiments/summary?alpha=0.05&bayesSamples=10000`
  exposes it. The winner verdict is only set when the pairwise
  test against the runner-up is significant at α=0.05.
- **T4 cost forecast + budget alerts** — per-org `cost_budgets`
  with weekly/monthly periods + `alert_threshold` (fraction of
  `limit_micros`). Migration 050 also adds `cost_forecast_snapshots`
  for the dashboard cache. `analysis/forecast.ts` ships an
  OLS linear-regression projection with a 95% confidence band,
  blended against the naive "current spend × (total/remaining)"
  estimate so day-1 of the period doesn't under-report.
  `CostForecastService.compute(orgId)` returns the snapshot +
  the alerts that would fire right now; `updateLastAlerted`
  applies a 1-hour cooldown so the webhook doesn't storm on
  every dashboard load. CRUD lives at `/api/admin/budgets`;
  the projection at `/api/admin/cost-forecast?organizationId=&windowDays=`.
  26 new vitest cases across the math, repo, and route layers.
- **T3-2 prompt firewall sidecar** — `packages/server/src/firewall/`
  ships the sidecar as both a Fastify plugin (mount on the same
  app as the rest of the routes) and a standalone CLI launcher
  (`pnpm --filter @promptsheon/server firewall`). The policy
  extracts the prompt text out of OpenAI- and Anthropic-shaped
  bodies, runs the T2-3 scanner, and either forwards as-is,
  forwards with an `X-Promptsheon-Warning` header, or rejects
  with `422 PROMPT_BLOCKED`. Every call — allow, warn, block —
  writes an `audit_entries` row tagged `action: 'firewall'` so
  `/api/audit/verify` covers sidecar activity end-to-end. CLI
  reads `PROMPTSHEON_FIREWALL_UPSTREAM_URL`,
  `PROMPTSHEON_FIREWALL_PORT`,
  `PROMPTSHEON_FIREWALL_BLOCK_THRESHOLD`, and a JSON-encoded
  `PROMPTSHEON_FIREWALL_UPSTREAM_HEADERS` for auth passthrough.
  18 new vitest cases covering `extractPromptText` (OpenAI +
  Anthropic shapes), `FirewallPolicy` (clean / warn / block
  thresholds), and the middleware round-trip with a stub
  upstream asserting the forwarded body + the audit row.
- **T4 CLI improvements** — `packages/cli/` restructured into
  `version.ts` + `errors.ts` + `output.ts` + `commands.ts` +
  dispatcher `index.ts`. Stable `PROMPTSHEON_CLI_VERSION = 0.5.0`
  constant. Locked exit-code contract (`EXIT.OK = 0`,
  `EXIT.BAD_ARGS = 2`, `EXIT.API_ERROR = 3`, `EXIT.NETWORK_ERROR = 4`,
  `EXIT.AUTH_ERROR = 5`, `EXIT.NOT_FOUND = 6`,
  `EXIT.CONFLICT = 7`, `EXIT.PRECONDITION_FAILED = 8`).
  Every command accepts `--json` (raw response) and mutating
  commands accept `--dry-run` (print the would-be request,
  exit 0). New commands: `release get <id>`, `release approve
  <id>`, `manifest scan <hash>`. 22 vitest cases covering
  argument validation, dry-run paths, and exit-code mapping.
- **T4 framework integrations** — `packages/sdk/src/integrations/`
  ships adapters for Vercel AI SDK (`withPromptsheon` wraps
  any `LanguageModelV1`), LlamaIndex (`PromptsheonLLM` exposing
  `complete` / `chat`), and Haystack (`PromptsheonGenerator`
  with `run` / `stream`). All three route through promptsheon's
  OpenAI-compatible gateway so caching + audit chain apply
  transparently. Structural typing keeps the SDK
  framework-package-free; consumers install the framework
  themselves. 9 vitest cases exercise the wire format against
  an in-process OpenAI-shaped stub.
- **T4 VS Code extension** — `extensions/promptsheon/` ships an
  extension that activates on `.promptsheon.json|yaml|yml`
  files. Pure validation logic in `src/validate.ts` runs the
  shared `ManifestSchema` locally, then posts the body to the
  server's new `POST /api/manifests/validate` endpoint for
  the canonical verdict; issues surface as VS Code
  diagnostics. Hover provider surfaces `Planner` / `Agent` /
  `Tool` / `Guardrail` documentation. The `promptsheon: Send to
  Playground` command opens `/app/playground?manifest=...` with
  the current buffer pre-loaded. The repo root's pnpm-workspace
  config now also lists `extensions/promptsheon` so the extension
  shares the workspace's `@promptsheon/shared` dep. Node:test
  coverage of the pure validation (9 cases).
- **T4 multi-region replication + failover** —
  `packages/server/src/replication/` ships the audit-chain
  replicator. `FileTarget` ships frames to a sibling SQLite
  replica (idempotent on `id`; works around the
  `audit_chain_state_no_update` trigger by setting
  `updated_by_app = 1`). `HttpTarget` POSTs frames to a
  remote replica's new `/api/audit/ingest` endpoint with a
  `/api/audit/replication-state` companion that returns the
  replica's high-water mark. The replicator daemon
  (`pnpm --filter @promptsheon/server db:replicate`) tails
  the primary in batches of 500 frames, polling every
  `PROMPTSHEON_REPLICA_INTERVAL_MS` ms; `PROMPTSHEON_REPLICA_ONESHOT=1`
  exits after a single batch for tests. The failover CLI
  (`pnpm --filter @promptsheon/server db:failover`) compares
  primary vs replica `audit_chain_state.last_rowid`, refuses
  to cut over if the replica is behind, and supports a
  `PROMPTSHEON_FAILOVER_FORCE=1` override for the documented
  accept-the-data-loss case. 7 vitest cases cover
  single-frame ship, idempotency, 10k-frame batch, chain
  continuity, and bounded lag measurement. Scope decision:
  the doc bundled "replication" + "active-active failover" in
  one bullet; better-sqlite3 is single-writer, so we ship
  audit-chain-only replication ( (the only data auditors
  require durable multi-region copies of) and treat failover
  as a documented operator runbook + safety belt rather than
  transparent active-active.

### Changed
- `POST /api/executions` and `POST /api/invoke` now persist
  the full inputs JSON in `executions.inputs` (was a SHA-256
  hash that prevented replay). The hash is preserved in a new
  `input_hash` column for dedup.
- Extended the scanner regexes to close real gaps the benchmark
  exposed: phone numbers with dot separators, multi-word
  role-switch / tool-abuse patterns, "from registry https://…attacker"
  supply-chain payloads, "no auth check" plugin misconfigurations,
  "jailbroken" (the adjective, not just "jailbreak"), and a
  tightened `jailbreak.dan` regex that no longer fires on the
  DAN-group acronym.

## [v0.4.2] - 2026-08-25

The end-of-audit release. Closes every issue surfaced by the
production audit (initial pass + R1–R6 follow-up). Maker-checker
flow now actually works end-to-end: bootstrap → workspace →
project → capability → version → release → approval → activation
gate, fully verified by curl + the Playwright tier suite.

### Added
- **`/api/webhooks` CRUD** — org-scoped store with `label, url,
  events, active`; GET/POST/PUT/DELETE handlers and audit
  entries on every mutation.
- **`/api/feature-flags` CRUD** — backed by an extended
  `FeatureFlagRepo`; migration 042 adds a JSON `value` column so
  flags can carry richer payloads than a boolean toggle.
- **`GET /api/capability-versions/:versionId/manifest`** — the
  drilldown for `/app/manifests/[versionId]`; returns the parsed
  manifest object plus metadata, size, and capability linkage.
- **`PUT /api/preconditions/:id`** — toggle `enabled` and other
  fields. Migration 043 adds `updated_at`.
- **`/api/capabilities/:id/self-evolve` (state) and
  `/api/capabilities/:id/self-evolve/run` (cycle)** — capability-scoped
  shortcuts that mirror the legacy paths.
- **`POST /api/invoke` alias** — accepts `{capabilityVersionId,
  inputs, …}`, resolves the version's `manifestHash` and forwards
  to the canonical execute path. SDK + curl examples that
  referenced `/api/invoke` now work.
- **`GET /api/goals/:hash` drilldown** — returns real iteration
  history + snapshots persisted on `GoalEvolutionState`. Frontend
  `/app/goals/[hash]` renders Overview / Iteration history /
  Snapshots tabs.
- **`/app/manifests/[versionId]`** — real 3-tab page (Overview,
  Approvals, History) instead of the "Manifest not found" empty
  state it used to render.
- **`/app/repos` "New repository" dialog** — typed Zod form, posts
  to `repoApi.create`, invalidates `['repos']`.
- **`/app/releases` "New release" dialog** — react-hook-form + Zod,
  capability + version + env + manifest fields; invalidates
  `['releases']`.
- **`requireAdmin()` middleware** in `packages/server/src/middleware/admin.ts` —
  applied to 14 management routes.
- **`new-repository-dialog.tsx` and `new-release-dialog.tsx`** — typed
  `react-hook-form + zodResolver` form primitives.
- **`DataTable<T>`** — generic with `Column<T>[]`, `<caption>`,
  `scope="col"`, sortable headers, focus-visible ring.
- **`unwrapList<T>` / `unwrapFirst<T>` helpers** in `lib/api.ts` —
  normalize the three backend list response shapes
  (`Array` / `{items,total}` / `{<plural>}`).
- **Migration 042** — `feature_flags.value` JSON column.
- **Migration 043** — `preconditions.updated_at` column.
- **Migration 044** — `manifest_dag.capability_id` nullable +
  FK dropped, so DAG-editor drafts can persist before a
  capability is selected.
- **Frontend lint config** — `frontend/.eslintrc.json` extending
  `next/core-web-vitals`, with `as unknown as <T>` banned via
  `no-restricted-syntax`.

### Changed
- **`AGENTS.md`** — corrected stack claim from "React 19 + Vite +
  React Router v7" to "Next.js 16 App Router + TanStack Query +
  axios + shadcn/ui".
- **`/api/workspaces` and friends** — pagination now coerces
  `?page=1` (string) to a number at the schema level, so the
  frontend's request shape passes without 422.
- **`.env.example`** — documents `PROMPTSHEON_WEBHOOK_SECRET` and
  the `PROMPTSHEON_ALLOW_SYSTEM_ACTOR` toggle.
- **`workspaceApi.list/get/create/update`** — return the same
  `{data: T}` shape the rest of the codebase expects from axios
  callers; the list endpoint unwraps `{items,total}` via the
  shared `unwrapList<T>` helper.

### Fixed
- **Maker-checker gate no longer silently passes** —
  `BaseRepo.findById` now camelizes snake_case columns before
  returning, so the gate correctly reads `release.createdBy`
  (was `undefined` under the snake_case rows). Self-approval is
  blocked again.
- **DAG editor Save** — `mergeDraftManifest()` in shared
  synthesises safe defaults for every required `Manifest` field
  when the editor saves a partial draft. `/api/manifests` POST
  now returns 201 for a `{nodes, edges, prompt}` payload instead
  of 422.
- **Release-route `require('node:crypto')`** — replaced with a
  static ESM `import { createHash } from 'node:crypto'`. The
  lazy require threw `ReferenceError: require is not defined`
  under the `/api/releases/:id/approvals` adapter.
- **Manifest-hash canonicalization** — both version-create and
  release-create now use raw-string SHA-256 to match what the
  activation gate looks up. The previous key-sorted
  canonicalization wrote the manifest under a hash the gate
  couldn't find.
- **Org-context-middleware key mismatch** — `routes/webhooks-crud.ts`
  `orgOf()` was reading `orgContext.orgId` but the middleware
  sets `organizationId`. Read the right key.
- **System-actor bypass disabled in production** — the
  always-on `X-User-Id: api` → admin context fallback is now
  scoped to non-production; configurable via
  `PROMPTSHEON_ALLOW_SYSTEM_ACTOR`.
- **Webhook secret no default in production** — `index.ts`
  throws on missing `PROMPTSHEON_WEBHOOK_SECRET` when
  `PROMPTSHEON_NODE_ENV=production`.
- **LLM-key echo to `process.env` removed** — `bootstrap.ts`
  no longer mirrors user-supplied LLM keys via `mirrorEnv()`;
  persistence is via `SettingsResolver` only.
- **Admin gate on 14 management routes** — `/api/users`,
  `/api/api-keys` (POST/DELETE), `/api/settings/:key` (PUT),
  `/api/webhooks` (CRUD), `/api/feature-flags` (CRUD). Non-admin
  callers now get 403.
- **Role escalation cap on api-keys POST** — even an admin
  can't mint a key with a role higher than their own; non-admins
  are silently demoted to `reader`.
- **`/api/audit/verify` and `/api/audit/state` whitelisted as
  public** — the docs link opens in a new tab and the browser
  can't attach session headers; the bypass makes the link work.
- **Frontend `subscribeSSE` cancel** — the old implementation
  closed a stale `EventSource` reference because re-subscribe
  wrote to a local. Captures the active source in a closure
  and clears the reconnect timer.
- **`/app/goals`** — was using raw `fetch('/api/goals')`,
  bypassing the axios interceptor that injects session headers.
  Switched to `client.get('/goals')`.
- **`/docs` search** — pruned 4 dead links (`/docs/onboarding`,
  `/docs/dag`, `/docs/grading`, `/docs/calibration`); added
  `/docs` to the index.
- **`docs/sdk` page** — no longer reads
  `process.env.PROMPTSHEON_API_KEY!` (would have inlined a real
  key into the client bundle if one ever landed in the build
  env). Replaced with a literal placeholder.
- **Frontend `data-table` snake/camel** — `/app/projects` and
  several other pages no longer needed `as unknown as X` casts
  after the generic `<T>` rewrite.
- **Empty `manifest_dag` registrations on editor Save** — the
  manifest now gets upserted with the correct hash on every
  version/release save, so the maker-checker gate can find it.

### Security
- **12 new admin-gated routes** — see above.
- **Snake-case → camelCase mapper in `BaseRepo.findById`** — fixes
  a class of latent bugs where `update()` methods bound undefined
  into NOT NULL columns (the `release.updateStatus` flow, the
  `/api/releases/:id/rollback` lookup, the `Capability.update`
  PUT path, etc.).
- **Audit chain verify + state publicly callable** —
  `PUBLIC_PATHS` in `auth.ts` includes both endpoints so the
  documentation link works without a session.

### Tests
- **Server: 322 → 377 vitest cases, 0 fail.** 10 new spec files:
  - `webhooks-crud.test.ts` (6) — CRUD + org-scope.
  - `feature-flag-routes.test.ts` (7) — CRUD + value JSON round-trip.
  - `audit-verify-public.test.ts` (3) — Public bypass + tampering.
  - `approval-reconcile.test.ts` (8) — Legacy path + adapter + 422.
  - `self-evolve-reconcile.test.ts` (6) — Both paths + idle stub.
  - `compiler-prompt.test.ts` (4) — `{prompt}` legacy body accepted.
  - `preconditions-update.test.ts` (4) — Toggle + bad input + 404.
  - `version-manifest.test.ts` (2) — Row round-trip + 404.
  - `admin-gating.test.ts` (8) — Admin vs reader on 5 routes.
  - `repo-routes-extended.test.ts` (3) — Workspace/project/capability CRUD coverage.
- **Frontend Playwright tier suite rewritten** against the new
  contracts:
  - `helpers/seed-session.ts` — bootstraps admin via
    `/api/bootstrap/admin`, writes the session to localStorage,
    so every tier runs without an LLM probe.
  - `tier-1-routes.spec.ts` — 41 routes, every path under an
    authenticated session.
  - `tier-4-forms.spec.ts` — workspace / api-key / webhook /
    feature-flag / schedule forms.
  - `tier-7-manifest-detail.spec.ts` (new) — DAG editor Save →
    redirect to `/app/editor/<hash>` → manifest-detail page.
  - `tier-8-approvals.spec.ts` (new) — reconciled approval route
    end-to-end.
  - `tier-9-admin-gating.spec.ts` (new) — admin sessions reach
    `/app/users`, `/app/api-keys`, `/app/webhooks`, `/app/settings`.

### Build & Typecheck
- `pnpm --dir packages typecheck` — clean across shared + server
  + frontend.
- `pnpm --dir packages/server test` — 59 files / 377 cases / 0
  fail.
- `pnpm --dir packages/shared test` — 3 files / 29 cases / 0 fail.
- `cd frontend && pnpm build` — `next build` emits the full
  route table; `/app/diff` is dynamically rendered.

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
