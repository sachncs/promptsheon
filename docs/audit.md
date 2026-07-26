# promptsheon — Code Audit

**Module:** `github.com/sachncs/promptsheon`
**Scope:** `pkg/`, `internal/`, `cmd/`, `sdk/`
**Method:** 8 parallel explore agents, full-file reads, no behaviour tests
**Date:** 2026-07-26

## Executive Summary

| Severity | Count | Primary themes |
|----------|-------|----------------|
| **Critical** | 56 | Concurrency races, data integrity, auth bypass, crash safety, security |
| **Major** | ~150 | Error handling, lifecycle, validation, economics (rate/cost), observability |
| **Minor** | ~200 | Naming, docs, idiomatic Go, polish, allocation patterns |

## Cross-Cutting Themes

1. **Concurrency races** — TOCTOU between read/write, atomic block checks, lock-then-act-inconsistently. Affects `pkg/cas`, `internal/api`, `internal/invoke`, `internal/election`, `internal/selfevolve`, `internal/store`, `internal/eventbus`, `internal/ratelimit`, `internal/redactor`, `internal/metrics`, `internal/guardrail`.
2. **Atomicity / transactional integrity** — partial writes between object, ref, HEAD, audit, release. Affects `pkg/cas`, `internal/store`, `internal/release`, `internal/selfevolve`, `internal/rollups`.
3. **Raw errors leaked to clients** — `err.Error()` echoed in HTTP responses, including provider SDK errors. Affects `internal/api` (10+ handlers), `internal/llm` (judge), `internal/guardrail`.
4. **Missing multi-tenant isolation** — no workspace ACL across `handleGet*` / `handleCreate*` paths.
5. **Missing size/rate limits** — decompression bombs, unbounded audit export, no body limits. Affects `pkg/cas`, `internal/api`, `internal/vault`.
6. **Predictable / weak IDs** — `UnixNano()`, time-derived IDs, no `crypto/rand`. Affects `internal/executor`, `internal/selfevolve`, `internal/guardrail`, `internal/alerting`.
7. **Sync `pkg` globals + init-time config** — `init()` reads env, package-level mutable state, test contamination.
8. **Critical paths missing panic recovery** — audit workers, executor goroutines, evolver, WebSocket handlers, eventbus subscribers.
9. **Outdated / incorrect GoDoc** — claims that don't match implementation.
10. **Dead/unreachable code** — `Authorizer`, `ErrAlreadyCanceled`, `judgeCache`, `mergeCIDRs`, `_ = showVersion`, `parseFloat64` mutable global, `errIs` hand-rolled.

---

## CRITICAL Findings

### `pkg/cas/`

| ID | File:line | Issue | Recommended fix |
|----|-----------|-------|-----------------|
| **C-cas-1** | `pkg/cas/verify.go:149-165` | `walkReachable` does not follow `TreeHash` → `Entries` or `Commit.TreeHash` → tree. Every real tree/blob is flagged as an orphan. | Add recursive walk through `Commit.TreeHash`, then iterate `Tree.Entries` and recurse for subtrees; treat blobs and trees as reachable when their hash appears in the entry map. |
| **C-cas-2** | `pkg/cas/log.go:22-23, 70` | `Log` documents linear history but appends `parents...` (all parents). Output is BFS-by-parent-sort, not `git log`-style. | Decide contract. If linear: `queue = append(queue, parents[0])`. If BFS: rewrite doc. |
| **C-cas-3** | `pkg/cas/commit.go:25-83` | `Commit` is non-atomic. No lock between writing commit object and updating ref/HEAD. Crash between two writes leaves orphan commits. Concurrent commits clobber. | Add repo-wide flock; write ref/HEAD to temp file, fsync, atomic rename. |
| **C-cas-4** | `pkg/cas/store.go:225-233, 294-302` | `WriteRef`/`WriteHEAD` are not crash-safe (no fsync, no atomic rename). | Write to `*.tmp`, `f.Sync()`, `os.Rename` over the target. |
| **C-cas-5** | `pkg/cas/store.go:103-111` | `WriteObject` does not `fsync`; OS-buffered bytes can vanish on crash. | Call `f.Sync()` before `Close()` (or sync the parent directory). |
| **C-cas-6** | `pkg/cas/store.go:143-157` | `ReadObject` has no size limit; decompression bomb / OOM possible. | Cap on-disk read at e.g. 64 MiB; use `io.LimitReader` around `gzip.NewReader`; return `ErrObjectCorrupted` on overflow. |
| **C-cas-7** | `pkg/cas/branch.go:35-66` | TOCTOU between `ReadRef` and `WriteRef` in `CreateBranch` — concurrent callers both succeed and one wins. | Use write-temp + atomic rename; combine with C-cas-3 repo lock. |
| **C-cas-8** | `pkg/cas/branch.go:80-96` | TOCTOU between `readHEADRef` and `os.Remove` in `DeleteBranch`. Concurrent `Checkout` can delete the now-current branch. | Under repo lock: call `os.Remove` directly, translate `IsNotExist` to `ErrRefNotFound`. |
| **C-cas-9** | `pkg/cas/branch.go:115-125, 134-142` | `Checkout` does not verify target hash is a commit, and has TOCTOU between `ReadRef` and `WriteHEAD`. | Read the object and check `IsCommit()`; refuse if ref is missing; atomic HEAD write. |
| **C-plugin-1** | `pkg/plugin/plugin.go:82-96` | `validateDescriptor` does not check `MinCoreVersion`. `ErrVersionTooOld` is dead code. | Add semver compare; return `ErrVersionTooOld` wrapped. |

### `pkg/plugin/`

| ID | File:line | Issue | Recommended fix |
|----|-----------|-------|-----------------|
| **C-plugin-1** | `pkg/plugin/plugin.go:82-96` | See above. | See above. |

### `internal/api/`

| ID | File:line | Issue | Recommended fix |
|----|-----------|-------|-----------------|
| **C-api-1** | `internal/api/handlers_auth.go:580-648` | `handleOAuthCallback` creates the OAuth user **twice**. Block A inserts `newUser`; block B (unreachable-looking but live code) re-inserts a *second* user with same email. | Delete block B; compute `autoProvisioned` from block A's success. |
| **C-api-2** | `internal/api/handlers_capabilities.go:612-627` | `classifyInvokeError` 502 path echoes `err.Error()` to clients (provider SDK internals, possibly URLs/tokens). | Drop `Details["error"]`; keep generic message; log wrapped error server-side. |
| **C-api-3** | `internal/api/handlers_auth.go:295-395` | `handleBootstrap` is unauthenticated admin-key mint when `PROMPTSHEON_AUTH=false` and bind is non-loopback. | Require `PROMPTSHEON_BOOTSTRAP_TOKEN` unconditionally; or bind bootstrap route to loopback unless explicitly exposed. |
| **C-api-4** | `internal/api/handlers_capabilities.go:84-340`, `handlers_releases.go:73-92`, `handlers_harness.go:101-333`, `handler_observation.go:23-51`, `routes.go:156-202` | Multi-tenant isolation absent. Every read/write gated only on `PermPromptRead`/`PermAuditRead` — no workspace-membership check on `{id}` URLs. | Add `RequireWorkspaceAccess(caller, wsID)` helper; apply to every handler that resolves an ID; reject cross-tenant with 404. |
| **C-api-5** | `internal/api/handlers_audit.go:71-107` | `handleExportAudit` is unbounded — no `since`/`until`/`limit`/`offset` floor. Memory exhaustion possible. | Require `since`; enforce pagination; bypass idempotency buffering for CSV exports. |

### `internal/auth/`

| ID | File:line | Issue | Recommended fix |
|----|-----------|-------|-----------------|
| **C-auth-1** | `internal/auth/oauth.go:70-92` | OAuth `state` parameter is caller-supplied; no server-side generation, storage, expiration, or validation. CSRF on the auth-code flow. | Server-side crypto-random single-use state; bind to session+provider+redirectURI; require+validate on callback. |
| **C-auth-2** | `internal/auth/oauth.go:15-24, 117-124, 156-163` | `TokenURL`/`UserInfoURL` are used directly with no SSRF restrictions. Bearer tokens sent to attacker-controlled endpoints. | Restrict to HTTPS + provider allowlist; reject loopback/private/link-local/metadata ranges; revalidate after DNS. |
| **C-auth-3** | `internal/vault/kmsbyok/provider.go:329-351` | KMS fallback persists `ciphertext == nil`. Subsequent reads treat state as missing and regenerate keys. Data unrecoverable. | Remove fallback unless adapter returns valid blob; validate non-empty before persist. |
| **C-auth-4** | `internal/vault/kmsbyok/provider.go:76-84, 124-132, 135-143` | Plaintext AES keys remain in cache after eviction/invalidation; not zeroized. | Overwrite plaintext buffers before eviction, replacement, invalidation; define ownership semantics for returned key material. |
| **C-auth-5** | `internal/vault/vault.go:164-181, 184-212` | AES-GCM ciphertext not bound to secret ID / tenant / provider via associated data. Ciphertext swap enables cross-secret decryption. | Add stable context as AAD: tenant ID, secret ID, provider, format/version. |

### `internal/llm/`

| ID | File:line | Issue | Recommended fix |
|----|-----------|-------|-----------------|
| **C-llm-1** | `internal/llm/anthropic.go:109-120` | Hidden `Thinking` blocks promoted to `Response.Content` when no visible text. Reasoning disclosure. | Never promote thinking blocks; return only visible text or expose via separate non-public field. |
| **C-llm-2** | `internal/llm/openai.go:90-115`, `provider.go:47-50` | OpenAI messages flattened into a single user string with textual markers. No actual role hierarchy; `[SYSTEM]` injection possible. | Use structured role/content input items; if SDK can't, return explicit unsupported error. |
| **C-llm-3** | `internal/llm/provider.go:83-88` | `ProviderConfig.APIKey` is JSON-serialised. Bearer credential can leak via logging/persistence. | Add `json:"-"`; separate secret material from serialisable config; provide redacted diagnostic view. |
| **C-llm-4** | `internal/llm/circuitbreaker.go:85-103` | Half-open permits unlimited concurrent probes — recovering provider can be re-outaged instantly. | Permit exactly one probe; reject or return distinct half-open error for others. |
| **C-llm-5** | `internal/llm/cost.go:18-36, 63-90` | `PricingTable` documented as concurrency-safe but uses unsynchronised map. `Register` panics on zero-value (nil map). | Add `RWMutex`; or use immutable copy-on-write; initialise map in `Register`. |
| **C-llm-6** | `internal/llm/judge.go:60-65`, `openai.go:117-123`, `anthropic.go:72-75` | Production judge sends no `Model`; both providers pass empty `Model` to APIs. | Add validated default model per provider; populate `Request.Model`. |
| **C-llm-7** | `internal/llm/capability/inheritance.go:131-145`, `hash.go:9-23` | `mergeArtifactSlice` iterates Go map → nondeterministic order → different manifest hashes for same inheritance chain. | Preserve deterministic order (base order + new overrides, or sort by `(Kind, Hash)`). |
| **C-llm-8** | `internal/capability/contract.go:60-63, 107-119` | `CanAutoAdopt` allows `BlastLow` with empty `SLOTarget`. Auto-promote of ungoverned capability. | Require non-empty, valid SLO in `CanAutoAdopt`. |
| **C-llm-9** | `internal/capability/version.go:5-14, 26-40`, `hash.go:17-23`, `manifest.go:75-86` | `ManifestHash` is optional and can disagree with `Manifest`. Slices are mutable after hashing. | Require+verify `ManifestHash` at creation; deep-copy slices; enforce immutability through constructors. |
| **C-llm-10** | `internal/capability/version.go:35-40`, `inheritance.go:29-36, 46-75`, `capability.go:20-39` | `Version.Parents` documented as capability IDs but resolved as version IDs; no scope/auth enforcement. Cross-workspace artifact inheritance. | Rename field to `ParentVersionIDs`; validate same-workspace; require explicit cross-scope authorisation. |

### `internal/store/`

| ID | File:line | Issue | Recommended fix |
|----|-----------|-------|-----------------|
| **C-store-1** | `internal/store/sqlite.go:673-697` | `BootstrapAdmin` uses `ON CONFLICT (email) DO NOTHING` without checking existing non-system users. Multi-admin bypass. | DB-level singleton bootstrap claim excluding system user; check + insert atomically under write lock; return `ErrConflict`. |
| **C-store-2** | `internal/store/idempotency_sqlite.go:52-71, 88-105` | Idempotency reads and writes are not atomic; expiry deletes can race with concurrent `PutIdempotency`. | Add atomic first-writer reservation/state machine: unique key, pending/completed states, leases, wait/replay. |
| **C-store-3** | `internal/store/sqlite.go:389-412, 414-465` | `VerifyAuditChain` can falsely certify a tampered prefix by relying on a cached checkpoint hash. | Maintain verifiable authenticated checkpoint chain; or periodic full walk under consistent read transaction. |
| **C-store-4** | `internal/store/sqlite.go:40-47`, `sqlite_releases.go:386-390`, `sqlite_capabilities.go:693-695, 818-819` | `mustUnmarshal` logs decode error but returns no error; corrupt manifests served as valid. | Return contextual decode/encode errors from all repository paths; fail closed. |
| **C-store-5** | `internal/store/sqlite_capabilities.go:610-624`, `sqlite_releases.go:46-74, 235-243, 320-328` | `CreateVersion` trusts caller `ManifestHash`; `CreateRelease` does not verify release matches version; `ActivateAtomic` updates legacy fields without denormalised `capability_version_id`. | Recompute and verify hash at storage boundary; derive release content from immutable version; make version/manifest identity immutable. |
| **C-store-6** | `internal/store/sqlite_capabilities.go:505-557` | Reputation queries reference non-existent `eval_results.status`, `executions.status`, `decisions.outcome`. Errors swallowed. | Align queries with persisted schema; use direct release→capability-version join; return first error with context. |
| **C-store-7** | `internal/store/sqlite.go:1105-1117` | `SaveAlert` uses `INSERT OR REPLACE` — destroys existing alert state, resets `acknowledged_at`/`acknowledged_by`. | Explicit UPSERT preserving immutable+acknowledgement fields. |
| **C-store-8** | `internal/store/migrate.go:290-302, 304-341` | Destructive down-migrations inferred solely from filename prefix; no transactional metadata update. | Represent destructive ops explicitly in migration metadata; verify state before rollback; tx-wrap operation + metadata update. |
| **C-store-9** | `internal/store/postgres/adapters.go:283-295` | Postgres `VerifyAuditChain` always returns `Ok: true` regardless of data. | Return explicit not-implemented error until real implementation exists; prevent production wiring. |
| **C-store-10** | `internal/harness/runner.go:91-99, 174-195, 238-243` | Eval run finalisation not atomic with result persistence. Result failure leaves `passed`/`failed` row without results. | Repository unit-of-work finalisation; or durable intermediate state + reconciliation. Propagate `markFailed` errors. |
| **C-store-11** | `internal/harness/runner.go:117-195`, `sqlite_harness.go:348-387` | Runner doesn't implement streaming path — all results buffered in memory, single bulk insert. | Actually call `CreateEvalResult` per case or use bounded batches. |
| **C-store-12** | `internal/harness/runner.go:79-99, 174-183, 199-235` | Eval preconditions: only dataset exists checked; release/dataset ownership not validated; empty dataset marks `RunPassed`; duplicate case seqs collide. | Load+compare release/dataset ownership; reject empty datasets unless explicit skipped; validate unique case IDs. |
| **C-store-13** | `internal/harness/runner.go:101-150, 245-253` | Worker count from CPU; cancellation doesn't stop dispatch; IDs timestamp-based with no idempotency key. | Configure explicit concurrency/semaphore; stop dispatch on cancellation; accept caller-supplied idempotency key. |
| **C-store-14** | `internal/harness/continuous.go:56-70, 97-135` | `ContinuousEval.Stop` can't cancel in-flight `RunOnce`; nil logger dereferenced. | Derive child ctx in `Start`, cancel from `Stop`; default nil logger to `slog.Default()`; ensure `done` closes on every path. |
| **C-store-15** | `internal/harness/continuous.go:29-38, 123-155` | Continuous eval has no jitter (config defines it), no env selector, no distributed claim, errors suppressed. | Implement cancellation-aware jitter; in-flight lease; distinguish storage failure from no-work; env selector. |
| **C-store-16** | `internal/harness/precondition.go:105-165, 173-215` | Preconditions run `sh -c` with full daemon privileges; no rollback; missing env gate silently returns success. | Sandbox: allowlisted argv model, resource limits, separate auth; prefer side-effect-free checks; fail closed when gate missing. |
| **C-store-17** | `internal/harness/precondition.go:216-250` | `CombinedOutput` buffers unbounded before `TruncateOutput`; kill signals not sent to process group; `Kill` errors ignored. | Capture through bounded/ring writer during execution; kill process group immediately on cancellation; surface cleanup failures. |
| **C-store-18** | `internal/store/sqlite_harness.go:84-112`, `harness/repo.go:9-17` | Dataset case replacement destroys concurrent edits; accepts unknown datasets. | Verify dataset exists; validate IDs/seqs; use optimistic versioning; provide atomic dataset-with-cases op. |
| **C-store-19** | `internal/store/sqliteimpl/lineage.go:18-70` | `PutGraph` is destructive whole-graph replacement; edge IDs not persisted; ordering nondeterministic. | Append/merge semantics or optimistic graph-version checks; persist edge identity; deterministic ordering key. |
| **C-store-20** | `internal/store/postgres/postgres.go:94-125`, `postgres/adapters.go:40-70, 482-590` | In-memory Postgres not concurrency-safe, not repository-faithful. | Add synchronisation; honor context; return consistent typed errors; document ordering/tx semantics; keep test-only. |
| **C-store-21** | `internal/store/postgres/adapters.go:248-431` | Many Postgres repository operations are stubs returning private `not implemented` error. | Don't expose as production repo until real implementation exists; export stable not-implemented sentinel if test adapter remains. |
| **C-store-22** | `internal/store/sqlite_capabilities.go:70-86, 147-164, 271-288`, `sqlite_harness.go:58-64, 873-882`, `sqlite.go:896-902` | Mutations ignore `RowsAffected`; stale IDs reported as updated/deleted. | Check affected rows; return `ErrNotFound` where appropriate; document intentionally idempotent ops. |
| **C-store-23** | `internal/store/sqlite.go:167-173` | `SQLite.DB()` leaks storage invariant boundary; bypasses validation. | Keep raw handle private; expose narrowly scoped read-only/transaction callbacks with explicit cache invalidation. |
| **C-store-24** | `internal/store/sqlite.go:795-815, 224-230` | Seeded system audit user can be deleted; subsequent audit FK fails. | Reject deletion/role mutation of system actor; return typed protected-resource error. |
| **C-store-25** | `internal/store/sqlite.go:121-149` | Prepared-statement init hides schema failures; per-`Prepare` error discarded. | Fail `NewSQLite` with contextual errors; close already-prepared statements; only fallback when intentional + observable. |
| **C-store-26** | `internal/store/migrate.go:48-127, 229-245` | Forward migration startup not serialised; no checksum validation; multiple daemons race. | DB-level migration lock; re-read state while holding it; validate strict filenames; persist/verify migration checksums. |
| **C-store-27** | `internal/store/migrate.go:217-250`, `sqlite.go:112-114` | FK PRAGMA on unpinned pooled connections; failed migration leaves pooled conn with FK off. | Pin single `*sql.Conn`; restore prior setting on every path; don't report migration complete until post-steps succeed. |
| **C-store-28** | `internal/store/sqlite_releases.go:218-255, 262-342` | Release activation vulnerable to stale writers; no expected status/version predicates. | Optimistic concurrency/version predicates; validate prior/next in same capability+env. |
| **C-store-29** | `internal/store/sqlite_releases.go:399-440` | Approval votes lost under concurrent approvers — JSON blob overwritten. | Persist votes as independently unique rows; or optimistic version/`updated_at` CAS with retry. |
| **C-store-30** | `internal/store/sqlite.go:1519-1546, 1585-1650` | Settings CRDT bypassed by unconditional writes in `SetSystemConfig`; `MergeSystemConfig` ignores `replicaID` and silently skips empty keys. | Vector-aware merge/CAS at write boundary; reject stale writes; validate keys and replica IDs. |
| **C-store-31** | `internal/store/sqlite_capabilities.go:845-915` | Due schedules read and claimed in separate operations — two schedulers both publish same event. | Atomic claim/lease with conditional update; publish only rows whose claim succeeded. |
| **C-store-32** | `internal/store/sqlite.go:1391-1409` | `SaveVaultState` UPSERT preserves DB `created_at` but in-memory object overwrites with `time.Now()` — disagreement. | Use existing creation timestamp on conflict or read back via `RETURNING`/follow-up query. |
| **C-store-33** | `internal/store/sqliteimpl/banditstore.go:35-58, 96-125` | Bandit validation incomplete — accepts empty arm IDs; `SUM` cast `int64→uint64` swallows negatives/overflow. | Reject empty arm IDs; validate counter ranges; fail on negative/overflowed aggregate values. |
| **C-store-34** | `internal/store/sqlite_alerting_m2m.go:14-23` | M2M linking uses broad `INSERT OR IGNORE` — can suppress intended constraints. | Targeted duplicate-conflict clause; validate rule/group IDs so FK and other constraint errors remain visible. |

### `internal/selfevolve/` + orchestration

| ID | File:line | Issue | Recommended fix |
|----|-----------|-------|-----------------|
| **C-self-1** | `internal/invoke/invoke.go:183-218` | Budget and quota charges lost / cap bypassed under concurrency: read with RLock, release, Charge, re-acquire Lock, overwrite. | Hold write lock for entire read→Charge→write window (or per-workspace sub-mutex); persist charges transactionally. |
| **C-self-2** | `internal/schedule/schedule.go:70-87, 106-116`, `internal/store/sqlite_capabilities.go:849-871` | Webhook/manual schedules fire forever — `NextFireAt` left as epoch, `MarkFired` doesn't update for non-cron, `ListDueSchedules` re-returns. | Filter non-cron kinds out of `ListDueSchedules`; or set `Enabled=false` after first `MarkFired`; document semantics. |
| **C-self-3** | `internal/election/election.go:91-147` | Split-brain window: stale-lease steal `UPDATE leader SET identity=?, expires_at=?` has no `expires_at <= ?` guard. | Add `AND (identity=? OR expires_at <= ?)` predicate; consider monotonic term + fencing tokens. |
| **C-self-4** | `internal/election/election.go:95` | Election uses `BEGIN DEFERRED` for a write transaction; promotes to write on `INSERT`/`UPDATE` and may fail with `SQLITE_BUSY_SNAPSHOT`. | Use SQLite `BEGIN IMMEDIATE` inside `Acquire`; configure busy-timeout PRAGMA + `SetMaxOpenConns`. |
| **C-self-5** | `internal/invoke/persisted_enforcer.go:45-63` | `PersistedEnforcer` constructor doesn't eagerly load persisted state; first request after restart is uncharged. | Add `ListEnforcerBudgets`/`ListEnforcerQuotas` to `EnforcerStore`; eager load on construction. |
| **C-self-6** | `internal/selfevolve/promoter.go:99-117` | Self-evolve promotion not transactional; orphan `Version` row if `CreateRelease`/`SelfActivate` fails. No `DeleteVersion` exists. | Wrap `CreateVersion`+`CreateRelease` in one repository tx; or implement compensating `DeleteVersion`. |

### `internal/eventbus/`, `internal/ratelimit/`, `internal/slo/`, `internal/alerting/`, `internal/webhook/`, `internal/subprocess/`

| ID | File:line | Issue | Recommended fix |
|----|-----------|-------|-----------------|
| **C-evb-1** | `internal/eventbus/publisher.go:228-249` | Sync `Publish` mutates caller's `Event.Timestamp` in place; reused struct sees prior timestamp. | Copy event before stamping. |
| **C-evb-2** | `internal/eventbus/publisher.go:177-188` | `dispatch` recovers subscriber panics but does not log. | Log panic via package logger; consider restarting failed subscribers. |
| **C-rl-1** | `internal/ratelimit/ratelimit.go:319` | `net.ParseIP(remote)` may return nil; `trustedProxies.Contains(nil)` returns true; X-Forwarded-For from attacker used as rate-limit key. | `if ip := net.ParseIP(remote); ip != nil && trustedProxies.Contains(ip)`. |
| **C-slo-1** | `internal/slo/slo.go:97-99` | `Validate` doesn't cross-check `Op` ↔ Signal — `OpGT` on latency means "lower latency is worse". | Reject mismatched Op↔Signal pairs in `Validate`. |
| **C-slo-2** | `internal/slo/slo.go:154-159` | `BurnRate` ignores Op direction — returns `actual/target` regardless. | Use `target/actual` for `OpGT` signals. |
| **C-slo-3** | `internal/slo/evaluator.go:80-96` | `Evaluator.Start` blocks until ctx cancelled; named `Start` but synchronous. No way to wait for return. | Rename to `Run`; provide drain helper. |
| **C-ale-1** | `internal/alerting/manager.go:309-331` | `TriggerAlert` appends to in-memory slice then writes DB; on DB failure in-memory record not removed. Phantom alert. | DB-first then in-memory; rollback on DB error. |
| **C-ale-2** | `internal/alerting/manager.go:289-356` | `m.alerts` slice grows unbounded; no TTL eviction. Memory leak. | Cap with ring buffer or persist + reload. |
| **C-ale-3** | `internal/alerting/manager.go:401-434` | `getNotificationChannels` per-alert DB call with 2s timeout under alert storm. | Cache M2M table with TTL. |
| **C-wh-1** | `internal/webhook/webhook.go:322` | `BypassSSRF` is package-level mutable `bool`; any importer can disable. | Build tag or constructor argument; never unexported package-level var. |
| **C-wh-2** | `internal/webhook/webhook.go:351-465` | `bytes.Reader` returned to pool immediately after `d.client.Do(req)`; body shared with in-flight request. | Add explicit body drain barrier before `Put`; document. |
| **C-sub-1** | `internal/subprocess/subprocess.go:88-151` | `cmd.Wait` not called on success path; `Reaped` always returns false for cleanly-exited plugin. | Call `cmd.Wait` on success path; document. |
| **C-sub-2** | `internal/subprocess/subprocess.go:178-221` | `Stop` does unbounded `c.Call("Plugin.Stop")`; stuck handler hangs supervisor. | Add per-call timeout (e.g. 1s); on timeout kill process group. |
| **C-metrics-1** | `internal/metrics/middleware.go:103-129` | `LLMMiddleware` panics on nil `tracer`; `tracer.Start(context.Background(), ...)` discards caller ctx — breaks distributed-trace propagation. | Guard `if tracer != nil`; thread caller ctx into `LLMMiddlewareFunc`. |

### `internal/rollups/`, `internal/eval/`, `internal/experiment/`, `internal/observation/`, `internal/bandsession/`, `internal/bridge/`, `internal/config/`, `internal/search/`, `internal/settings/`, `internal/pluginsup/`

| ID | File:line | Issue | Recommended fix |
|----|-----------|-------|-----------------|
| **C-roll-1** | `internal/rollups/rollups.go:202-205` | `drainSummaries` returns `nil`; whole `RunSink` loop never writes — rollup pipeline is dead. | Implement `drainSummaries` to actually iterate and call `Sink.Write`. |
| **C-eval-1** | `internal/eval/scorer.go:65-95` | Scorers are package-level globals mutated by `Register`; hot-reload can leave stale entries; no `Reset`. | Per-run registry; or document + add `Reset` helper. |
| **C-eval-2** | `internal/eval/scorer_llm_judge.go:163-203` | `judgeCache` created but never instantiated — dead code. | Wire the cache or remove. |
| **C-exp-1** | `internal/experiment/engine.go:87-100` | `Engine.metrics` inner map not guarded — `Metric` mutated concurrently with read. | Take outer lock during read; or copy `Metric`. |
| **C-obs-1** | `internal/observation/observation.go:121-160` | `Add` holds `a.mu` AND acquires `b.mu` inside eviction loop; lock held for O(N) walk blocking all writers. | Snapshot bucket list under `a.mu`, release, walk with bucket locks. |
| **C-band-1** | `internal/bandsession/session.go:105-136` | `RegisterArms` holds `s.mu` for entire DB round-trip — blocks all `Select`/`Observe`. | Copy inputs, release, do I/O, re-acquire. |
| **C-band-2** | `internal/bandsession/session.go:171-199` | `Observe` holds `s.mu` for store.Observe + Flush + selector.Observe — same. | Same fix as C-band-1. |
| **C-bridge-1** | `internal/bridge/bridge.go:46-71` vs `:105-120` | Two `Evaluate`-style methods on `BreachEvent` — public `Evaluate` returns `compress_prompt` for any latency breach, private `recommendation()` does burn-rate escalation. Public is wrong. | Unify or remove public `Evaluate`. |
| **C-cfg-1** | `internal/config/config.go:322-360` | `Validate` error message references `PROMPTSHEON_BOOTSTRAP_TOKEN` but Config struct has no such field. | Add `BootstrapToken` field; load from env; validate. |
| **C-search-1** | `internal/search/manager.go:20-38` | `Manager` holds mutex then calls `Index` which holds its own mutex — two layers for no purpose. | Pick one. |
| **C-sett-1** | `internal/settings/resolver.go:180-202` | `Set` is read-modify-write without lock; concurrent `Set` on same key silently overwrites. | Either document loudly or guard at resolver with `sync.Mutex`. |
| **C-psup-1** | `internal/pluginsup/supervisor.go:227-241` | `grpc.NewClient("unix://"+g.Addr, ...)` is non-blocking; first RPC fails on bad address. `_ = ctx` ignores dial ctx. | Use `grpc.DialContext` (blocking) or document. |
| **C-psup-2** | `internal/pluginsup/supervisor.go:224-226` | `dial` checks `g.Addr == ""` but doesn't validate path. External caller can set `Addr` to anything. | Validate path against UDS pattern. |

### `internal/approval/`

| ID | File:line | Issue | Recommended fix |
|----|-----------|-------|-----------------|
| **C-app-1** | `internal/approval/approval.go:78-79` | `Evaluate` returns `("", false, ErrCreatorVoted)`. Callers switching on `state` before checking err see zero value. | Add typed return or document loudly. |

### `cmd/` + `sdk/`

| ID | File:line | Issue | Recommended fix |
|----|-----------|-------|-----------------|
| **C-sdk-1** | `sdk/client.go:735-738` | SDK `delete` helper drops caller `context.Context` — calls `c.do(context.Background(), ...)`. Cancellation and trace baggage lost. | `func (c *Client) delete(ctx context.Context, path string) error`; update callers. |
| **C-cmd-1** | `cmd/promptsheond/evolver_cas.go:15-39`, `main.go:866-889`, `evolver_wire.go:101-107` | CAS path used by evolver + default artifact loader is CWD-relative. Production daemon run as system service from `/` or `/opt` has no `.promptsheon/`. | Make CAS root a startup config (`PROMPTSHEON_CAS_ROOT`); pass explicitly; or refuse to start self-evolve if absent. |
| **C-cmd-2** | `cmd/promptsheond/main.go:1026-1045` | Graceful-shutdown force-quit window: `SIGTERM`+`SIGINT` within 200ms → `os.Exit(130)` from drain goroutine, bypassing defers; goroutine dumps to `/tmp` world-writable. | Drop 200ms heuristic; use 2-minute watchdog; configurable diagnostics dir with `0o700`. |
| **C-cmd-3** | `cmd/promptsheond/main.go:1254-1264` | `--backup` flag silently deletes any pre-existing destination via `os.Remove(dst)` before `VACUUM INTO`. | Remove `os.Remove(dst)`; return clear error: "destination %q already exists; append timestamp suffix". |
| **C-cmd-4** | `cmd/promptsheon/http.go:41-104`, `selfevolve.go:170` | CLI HTTP helpers have no client timeout — `http.Get`/`http.Post` use default client (no timeout). Daemon-down hangs indefinitely. | Add `&http.Client{Timeout: 30 * time.Second}` shared var; pass context-aware requests. |

---

## MAJOR Findings (representative)

- `internal/api/handlers_health.go:78, 117` — `/ready` leaks `err.Error()` (unauthenticated).
- `internal/api/handlers_releases.go:64, 137, 173, 186, 221` — raw `err.Error()` echoed.
- `internal/api/handlers_contract.go:33`, `handlers_auth.go:504`, `handlers_providers.go:87`, `handlers_capabilities.go:409, 427, 430, 432`, `handlers_reasoning.go:29, 33, 39, 41, 43`, `http.go:24, 87` — same pattern.
- `internal/api/audit_workers.go:146-173` — no panic recovery in audit worker goroutine.
- `internal/api/audit_workers.go:85-103` — `StartAuditWorkers` not safely re-entrant; `auditWg` not reset.
- `internal/api/audit_workers.go:43-44` — `_ = writeStart` dead; `r.RemoteAddr` stored verbatim, no trusted-proxy header parsing.
- `internal/api/idempotency.go:86-93` — O(N) slice surgery in eviction.
- `internal/api/idempotency.go:206-253` — `hashAndTeeBody` Windows incompatibility + temp-file race.
- `internal/api/handlers_providers.go:47-101` — `handleTestProvider` lets writer-role key spend real money, no per-key budget.
- `internal/api/handlers_releases.go:280` — `handleInvokeRelease` echoes raw `invErr.Error()`.
- `internal/api/handlers_alerting.go:195-205` — `handleResolveAlert` returns 200 on no-op.
- `internal/api/handlers_settings.go:102-133` — PUT does not validate key shape.
- `internal/api/handlers_auth.go:609-626` — `handleOAuthCallback` mints fresh API key on every login.
- `internal/auth/authenticator.go:142-160` — auth-failure audit over-broad.
- `internal/auth/authenticator.go:104-123` — logs API-key DB IDs via global logger.
- `internal/auth/authenticator.go:180-190` — error text inserted into JSON without escaping.
- `internal/auth/authenticator.go:127-137` — `Stop` concurrent-close race.
- `internal/auth/oauth.go:79-92, 104-118` — manual URL encoding via `fmt.Sprintf` (no `url.Values`).
- `internal/auth/oauth.go:130-133, 168-170` — unbounded error response bodies.
- `internal/auth/oauth.go:63-68` — `OAuthProvider` stored as caller-owned pointer (mutable after register).
- `internal/llm/circuitbreaker.go:41-49` — config not validated; auth/cancellation counted as failures.
- `internal/llm/cost.go:71-80` — silent underreport for unknown models; float64 for money; no validation.
- `internal/llm/openai.go:42-59` — arbitrary `BaseURL` receives API key.
- `internal/llm/anthropic.go:21-39, 52-88` — per-call key ignored.
- `internal/llm/openai.go:117-135`, `anthropic.go:54-85` — generic request params silently dropped.
- `internal/llm/openai.go:127-131`, `anthropic.go:80-82` — zero temp/TopP cannot be expressed.
- `internal/llm/openai.go:155-165` — `Status` mapped to `StopReason`.
- `internal/llm/fallback.go:28-67` — failovers for every error incl. cancellation/permanent.
- `internal/llm/fallback.go:43-47` — skips providers by `Name()` not instance identity.
- `internal/llm/openai.go:63-80` — new HTTP client per per-call key.
- `internal/llm/middleware.go:45-63` — instrumentation records fallback composite name not actual provider.
- `internal/llm/middleware.go:83-145` — aggregate metrics without snapshot API.
- `internal/capability/capability.go:62-79` — contract changes mutable; not versioned.
- `internal/capability/contract.go:64-93` — accepts negative latency; NaN bypasses range comparisons.
- `internal/capability/manifest.go:93-135` — `Manifest.Validate` doesn't validate populated `Context`/`Memory`.
- `internal/capability/manifest.go:16-28, 70-86` — knowledge sources no dedicated kind; ignored by inheritance/diff.
- `internal/capability/diff.go:31-83` — nondeterministic pairing.
- `internal/capability/manifest.go:30-57, 138-149` — slice dedup uses only hash.
- `internal/capability/repo.go:5-18, 18-75` — repository segregation violated; one facade.
- `internal/capability/repo.go:18-75, 77-82` — list APIs unbounded.
- `internal/capability/execution.go:11-27` — exec records accept unrestricted raw input/output.
- `internal/llm/anthropic.go:103-107`, `openai.go:105-115` — repeated `+=` on string concat.
- `internal/llm/fallback.go:20-25, 70-77` — `Fallback.Name` formats chain on every call; retains caller slice.
- `internal/llm/mock.go:31-50, 61-69` — mock doesn't honor ctx cancellation; shallow copies.
- `internal/llm/middleware.go:45-80` — collector callbacks can panic and add latency.
- `internal/capability/capability.go:94-101` — `SelfEvolveConfig.IsZero` checks all fields for zero while doc describes effective defaults.
- `internal/capability/event.go:7-25` — event doc mentions two types but three declared.
- `internal/capability/diff.go:6-19, 55-59` — `ChangedPrompts` contains changes for every artifact kind.
- `internal/capability/hash.go:9-23` — no algorithm/schema version or domain-separation.
- `internal/llm/anthropic.go:52-55`, `openai.go:104-105`, etc. — nil-request/-response panics instead of errors.
- `internal/api/middleware.go:121-148`, `http.go:85-94` — recovery JSON envelope timing.
- `internal/api/pagination.go:50-59` — returns `[]T{}` not `nil`.
- `internal/api/audit_workers.go:102` — `_ = ctx` to silence unused-param warning.
- `internal/api/handlers_webhooks.go:47-53` — `webhookEndpointPublic` non-pointer value.
- `internal/api/handlers_auth.go:26` — `fieldAPIKey` reused for settings key.
- `internal/api/handlers_alerting.go:40-94` — `req.Type` not validated.
- `internal/api/validate.go:59-67` — text-match fallback after `errors.As` succeeds.
- `internal/api/handlers_capabilities.go:533-543` — `req.Inputs` size not bounded.
- `internal/api/handlers_audit.go:113-141` — CSV injection (no `'+-@` quote).
- `internal/api/server.go:43-114` — both `db` and `capabilityRepo2` exposed; inconsistent.
- `internal/api/audit_workers.go:43` — `r.RemoteAddr` without trusted-proxy header.
- `internal/api/audit_workers.go:49-57` — `time.NewTimer` per audit call under load.
- `internal/api/audit_workers.go:122-128` — `auditStopOnce.Do` goroutine ordering.
- `internal/api/handlers_auth.go:230-264` — `handleCreateAPIKey` no rate limit on key creation.
- `internal/api/handlers_workflow.go:19-49` — `def.ID` charset/length not bounded.
- `internal/api/handlers_harness.go:159-196` — accepts arbitrary `Command` strings.
- `internal/api/handlers_webhooks.go:55-119` — secret persisted plaintext as well as ciphertext.
- `internal/auth/oauth.go:15-25` — `RedirectURL` not validated against request host.
- `internal/auth/authorizer.go:7-32` — `Authorizer` and `Require` unused.
- `internal/api/handlers_auth.go:52-84` — `oauthStateStore` package-global, races across `NewServer`.
- `internal/api/validate.go:38-40` — 499 status non-standard.
- `internal/api/audit_workers.go:24-36` — fallback `"api"` not `"anonymous"`.
- `internal/auth/auth.go:152-154` — `ValidateAPIKeyFormat` comment misleading.
- `internal/api/handlers/http.go` vs `internal/api/handlers/` — name collision.
- `internal/api/handlers_auth.go:113-118` — package-level `StartOAuthStateJanitor`.
- `internal/api/audit_workers.go:61-63` — `SetAuditDropped` only when collector non-nil.
- `internal/api/server.go:85-99` — `auditWg`/`auditDone` not exposed for tests.
- `internal/api/audit_workers.go:131-133, 139-141` — nil-check `auditCancel` at use sites.
- `internal/api/handlers_capabilities.go:665` — `r.PathValue("workspace_id")` empty for `/versions/{id}/executions`.
- `internal/api/handlers_capabilities.go:465-467` — `versionResolverAdapter.GetVersion` uses `context.Background()`.
- `internal/api/handlers_contract.go:170-187` — `handleCatalogSearch` lacks workspace ACL.
- `internal/api/audit_workers.go:24-68` — `audit` doesn't enforce max `Details` size.
- `internal/api/handlers_auth.go:189-237` — `callerID` lookup `_ = ctx` pattern.
- `internal/api/audit_workers.go:169` — panics on nil `entry.Timestamp`.
- `internal/api/handlers_health.go:14` — `startTime` package-global.
- `internal/api/audit_workers.go:118-144` — nil-queue check incomplete if `auditCancel` non-nil.

Plus analogous items in `internal/store/`, `internal/capability/`, `internal/llm/`, `internal/vault/`, `internal/injection/`, `internal/redactor/`, `internal/guardrail/`, `internal/selfevolve/`, `internal/workflow/`, `internal/release/`, `internal/invoke/`, `internal/executor/`, `internal/eventbus/`, `internal/ratelimit/`, `internal/quota/`, `internal/budget/`, `internal/slo/`, `internal/alerting/`, `internal/webhook/`, `internal/adoption/`, `internal/approval/`, `internal/lineage/`, `internal/policy/`, `internal/rollups/`, `internal/replay/`, `internal/eval/`, `internal/optimizer/`, `internal/experiment/`, `internal/recommendation/`, `internal/reasoning/`, `internal/observation/`, `internal/bandit/`, `internal/banditstore/`, `internal/bandsession/`, `internal/bridge/`, `internal/buildinfo/`, `internal/config/`, `internal/context/`, `internal/mcpsdk/`, `internal/mcplist/`, `internal/pluginmanifest/`, `internal/plugins/builtins/`, `internal/pluginsup/`, `internal/search/`, `internal/settings/`, `internal/subprocess/`, `internal/testutil/`, `internal/models/`, `internal/trace/`, `internal/ws/`, `internal/observation/`, `internal/lineage/`, `internal/injection/`, `internal/guardrail/`, `internal/redactor/`, `internal/vault/`.

---

## MINOR Findings (representative)

Naming, docs, idiomatic Go, allocation patterns. Full set covers all packages; key categories:

- `Get*` accessor prefix discouraged — replace with idiomatic names (`Stats()`, `CurrentCommitHash()`, `CurrentRef()`).
- One-line wrappers (`ObjectHash`, `canonicalSerialize`).
- Bypassing `os.OpenRoot` (e.g. `ObjectExists`/`ObjectFileSize`).
- `HEADRefName` returns whole string on unparseable input.
- Inconsistent sentinel-wrapping between `validateRefName`/`validateHash`.
- `simhash` allocates FNV per shingle.
- `computeTextDiff` materialises every line.
- `_ = path` no-op in `verify.go`.
- `parseFloat64` mutable package-level var.
- `Log` "newest first" misleading.
- `LogEntry.Telemetry` shallow copy.
- `HEAD` rejection magic check outside `validateBranchName`.
- `Checkout` writes `"ref: refs/heads/"+target` without validating concat.
- `shortHash` doc overstates precondition.
- `repo.SetLogger` racy.
- `Init` not truly idempotent.
- `ListRefDetails` returns `[]*RefDetail`; `GetStats` returns `*RepoStats`.
- `ReadObject` doesn't use `validateHash`.
- `TelemetryKV.Value` is `any` but diff is numeric-only.
- Package doc in non-canonical file.
- `Init` doesn't validate `PROMPTSHEON_LOG_LEVEL`.
- Magic numbers, stale TODO comments.
- Missing GoDoc on exported items in many files.
- And dozens more across `internal/`.

---

## Atomic Fix Plan (priority order)

Each item below is one logical change. Apply independently.

1. `pkg/cas/store.go` — atomic write + fsync + cap `ReadObject`.
2. `pkg/cas/store.go` — atomic `WriteRef`/`WriteHEAD`.
3. `pkg/cas/store.go` — fsync on `WriteObject` close.
4. `pkg/cas/commit.go` — add repo lock; refactor to write-then-rename atomically.
5. `pkg/cas/branch.go` — atomic `CreateBranch`/`DeleteBranch`/`Checkout`.
6. `pkg/cas/verify.go` — `walkReachable` follows tree→entries.
7. `pkg/cas/log.go` — linear-history semantics; doc fix.
8. `pkg/cas/helpers.go` — fold `..`/`HEAD` rejection into `validateBranchName`.
9. `pkg/cas/store.go` — `validateHash` on `ReadObject`; `os.OpenRoot` for `ObjectExists`/`ObjectFileSize`.
10. `pkg/cas/store.go` — `HEADRefName` returns `""` on malformed.
11. `pkg/cas/store.go` — `atomic.Pointer[slog.Logger]`.
12. `pkg/cas/repo.go` — `Init` only rewrites HEAD/ref if not already initialised.
13. `pkg/cas/commit.go` — replace `errIs` with `errors.Is`.
14. `pkg/cas/log.go` — deep-copy `Telemetry`.
15. `pkg/cas/store.go` — log cleanup failures.
16. `pkg/plugin/plugin.go` — `validateDescriptor` checks `MinCoreVersion`.
17. `internal/store/sqlite.go` — `BootstrapAdmin` singleton enforcement.
18. `internal/store/idempotency_sqlite.go` — atomic idempotency state machine.
19. `internal/store/sqlite.go` — reliable audit-chain verification (drop checkpoint shortcut).
20. `internal/store/sqlite*.go` — fail closed on corrupt JSON; remove `mustUnmarshal`.
21. `internal/store/sqlite_capabilities.go` + `sqlite_releases.go` — enforce content-addressed identity.
22. `internal/store/sqlite_capabilities.go` — fix reputation queries.
23. `internal/store/sqlite.go` — `SaveAlert` explicit UPSERT.
24. `internal/store/migrate.go` — destructive-migration metadata + transactional rollback.
25. `internal/store/postgres/adapters.go` — return not-implemented for `VerifyAuditChain`.
26. `internal/store/sqlite.go` — protect system audit user from deletion.
27. `internal/store/sqlite.go` — fail `NewSQLite` on prepare errors.
28. `internal/store/sqlite.go` — keep `DB()` private; expose scoped helpers.
29. `internal/store/sqlite_*.go` — `RowsAffected` consistency.
30. `internal/store/migrate.go` + `sqlite.go` — pin `*sql.Conn` for FK PRAGMA.
31. `internal/store/sqlite_releases.go` — optimistic concurrency on activation.
32. `internal/store/sqlite_releases.go` — vote rows vs JSON blob.
33. `internal/store/sqlite.go` — vector-aware `SetSystemConfig`.
34. `internal/store/sqlite_capabilities.go` — atomic schedule claim.
35. `internal/store/postgres/postgres.go` + `postgres/adapters.go` — concurrency-safe stub.
36. `internal/store/sqlite_harness.go` — optimistic dataset case upsert.
37. `internal/store/sqliteimpl/lineage.go` — append/merge semantics + persisted edge IDs.
38. `internal/store/sqlite.go` — `SaveVaultState` reads back on conflict.
39. `internal/store/sqliteimpl/banditstore.go` — validate arm IDs; safe counter cast.
40. `internal/store/sqlite_alerting_m2m.go` — targeted duplicate-conflict clause.
41. `internal/harness/runner.go` — atomic eval finalisation.
42. `internal/harness/runner.go` + `sqlite_harness.go` — stream results via `CreateEvalResult`.
43. `internal/harness/runner.go` — release/dataset ownership validation; reject empty dataset.
44. `internal/harness/runner.go` — concurrency limit + idempotency key + cancellation.
45. `internal/harness/continuous.go` — child ctx cancellation; nil-logger default; jitter; env selector.
46. `internal/harness/precondition.go` — sandbox; bounded output; process-group kill.
47. `internal/invoke/invoke.go` — write-lock across read→Charge→write.
48. `internal/invoke/persisted_enforcer.go` — eager load on construction; pass request ctx to persist.
49. `internal/schedule/schedule.go` — filter non-cron kinds from `ListDueSchedules`; document semantics.
50. `internal/election/election.go` — `BEGIN IMMEDIATE`; add `expires_at <= ?` predicate; monotonic term.
51. `internal/selfevolve/promoter.go` — transactional `CreateVersion`+`CreateRelease`; validate `Activator`.
52. `internal/selfevolve/evolver.go` — validate constructor wiring; panic recovery in `runRevisions`.
53. `internal/api/handlers_auth.go` — remove duplicate user creation.
54. `internal/api/handlers_capabilities.go` — sanitise `Details["error"]`.
55. `internal/api/handlers_auth.go` — require bootstrap token unconditionally (or loopback-only route).
56. `internal/api/handlers_*.go` — add `RequireWorkspaceAccess(caller, wsID)` helper and apply per-handler.
57. `internal/api/handlers_audit.go` — bound `handleExportAudit`.
58. `internal/api/audit_workers.go` — panic recovery in worker; reset `auditWg` on start.
59. `internal/auth/oauth.go` — server-side state generation/storage/validation.
60. `internal/auth/oauth.go` — SSRF-restrict `TokenURL`/`UserInfoURL`; allowlist hosts.
61. `internal/auth/oauth.go` — `url.Values.Encode` for OAuth params; bound error body reads.
62. `internal/auth/oauth.go` — deep-copy provider on registration.
63. `internal/auth/authenticator.go` — `sync.Once` for `Stop`; configured logger for API-key logs; JSON-safe error responses.
64. `internal/vault/kmsbyok/provider.go` — remove nil-ciphertext fallback; zeroize plaintext buffers.
65. `internal/vault/vault.go` — AES-GCM AAD with tenant/secret/provider; enforce size limits.
66. `internal/vault/providers.go` — validate 32-byte key in `NewStaticKeyProvider`; honor ctx.
67. `internal/guardrail/manager.go` — fail closed on unknown envs/policies; validate threshold/weights; UUID violation IDs; json.Valid; concurrent-safe rule access.
68. `internal/redactor/redactor.go` — synchronised `Enable`/`Disable`; `regexp.QuoteReplacement`; Luhn on cc candidates; IPv4 range check.
69. `internal/injection/detector.go` — validate `OverrideThreshold` and `Enable` weights.
70. `internal/llm/anthropic.go` — never promote `Thinking` to content.
71. `internal/llm/openai.go` — structured role/content items (no marker flattening).
72. `internal/llm/provider.go` — `json:"-"` on `APIKey`; allowlist `BaseURL`; per-call key support.
73. `internal/llm/judge.go` — populate `Model`; validate judge-provider env.
74. `internal/llm/circuitbreaker.go` — single probe; config validation; classify errors.
75. `internal/llm/cost.go` — RWMutex; reject unknown models; numeric validation.
76. `internal/llm/fallback.go` — stop on permanent errors; respect `ctx.Err()`; return aggregated error.
77. `internal/llm/middleware.go` — add real provider/model to `Response`; snapshot aggregates.
78. `internal/llm/openai.go` — reuse transport; map `Status` correctly; temperature/TopP zero handling.
79. `internal/capability/inheritance.go` — deterministic artifact-slice ordering.
80. `internal/capability/contract.go` — `CanAutoAdopt` requires SLO.
81. `internal/capability/version.go` — require+verify `ManifestHash`; deep-copy slices.
82. `internal/capability/version.go` + `inheritance.go` — rename `Parents` to `ParentVersionIDs`; scope validation.
83. `internal/capability/hash.go` — algorithm/schema version + domain separation.
84. `internal/capability/manifest.go` — validate optional refs; deep-copy slices.
85. `internal/capability/manifest.go` — add `ArtifactKnowledge` kind; include in inheritance/diff.
86. `internal/capability/diff.go` — deterministic pairing; rename `ChangedPrompts`.
87. `internal/capability/contract.go` — cross-check Op↔Signal; validate numerics.
88. `internal/capability/repo.go` — split into small consumer-side interfaces.
89. `internal/capability/repo.go` + `execution.go` — bound list APIs; restrict exec IO size.
90. `internal/slo/slo.go` — `Validate` cross-checks Op↔Signal; `BurnRate` respects Op.
91. `internal/slo/evaluator.go` — rename `Start`→`Run`; bounded breach callback.
92. `internal/eventbus/publisher.go` — copy event before stamping timestamp; log subscriber panics.
93. `internal/ratelimit/ratelimit.go` — `net.ParseIP` nil check; CIDR list (not merged); drop init-time env.
94. `internal/alerting/manager.go` — DB-first insert; cap `m.alerts`; cache M2M channels; non-blocking delivery.
95. `internal/webhook/webhook.go` — `BypassSSRF` constructor arg with build tag; safe body pool return.
96. `internal/subprocess/subprocess.go` — `cmd.Wait` on success; `c.Call` timeout.
97. `internal/rollups/rollups.go` — implement `drainSummaries`.
98. `internal/eval/scorer.go` — document global registry; remove dead `judgeCache`.
99. `internal/experiment/engine.go` — outer lock during read; `MinSamples > 0`; Welford mean; status constants.
100. `internal/observation/observation.go` — bucket snapshot; use `now` parameter.
101. `internal/bandsession/session.go` — release lock across I/O.
102. `internal/bridge/bridge.go` — unify/remove duplicate `Evaluate`.
103. `internal/config/config.go` — add `BootstrapToken`; cap YAML file size; validate TLS pair.
104. `internal/search/manager.go` + `bm25.go` — single lock layer.
105. `internal/settings/resolver.go` — document or guard `Set` concurrency.
106. `internal/pluginsup/supervisor.go` — `grpc.DialContext`; validate `Addr`.
107. `internal/metrics/middleware.go` — nil-tracer guard; thread caller ctx.
108. `sdk/client.go` — `delete(ctx, path)`; update callers; add `User-Agent`; add timeout/retry options.
109. `cmd/promptsheon/http.go` — `&http.Client{Timeout: 30s}`.
110. `cmd/promptsheon/main.go` — fix `readStrFlag` off-by-one; `--provider` in `cmdRelease invoke`; `cmdRun` prompt precedence; `cmdProvider test` print.
111. `cmd/promptsheond/main.go` — drain audit before `db.Close()`; refuse `--backup` overwrite; reject non-loopback pprof.
112. `cmd/promptsheond/evolver_cas.go` + `main.go` + `evolver_wire.go` — explicit CAS root; injectable interface; env defaults.
113. Tests for each critical fix.
114. `go vet`, `golangci-lint`, `go test -race -count=1`.

---

**End of audit.**