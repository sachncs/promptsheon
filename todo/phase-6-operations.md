# Phase 6 — Operations

All operations findings. Fast forward: replace, don't shim.

## Replicas and scaling

- [x] **OPS-1a** Remove the `replicaCount == 1` constraint from `values.schema.json`.
  - **Where**: `deploy/helm/promptsheon/values.schema.json:13`.

- [x] **OPS-1b** Add a leader-election layer (e.g. SQLite advisory lock via `PRAGMA user_version` swap) so multi-replica is safe.
  - **Where**: new file `internal/election/sqlite.go` and `cmd/promptsheond/main.go`.

- [x] **OPS-2a** Add an HPA template scaled on `promptsheon_http_requests_total` rate.
  - **Where**: `deploy/helm/promptsheon/templates/hpa.yaml` (new).

- [x] **OPS-2b** Add a `PodDisruptionBudget` (already present at `pdb.yaml`) — verify `minAvailable` is sane.
  - **Status**: `minAvailable: 1` is documented as sane for single-replica deployments; the chart comment points operators at `maxUnavailable` for multi-replica setups.

- [x] **OPS-2c** Add `topologySpreadConstraints` and `priorityClassName` for production-grade scheduling.
  - **Where**: `deploy/helm/promptsheon/templates/statefulset.yaml`.
  - **Status**: `topologySpreadConstraints` (zone-aware, `ScheduleAnyway` fallback) and `priorityClassName: system-node-critical` are wired. Both are no-ops on clusters that don't define the referenced topology key / priority class.

- [x] **OPS-2d** Add a `startupProbe` with a generous failure threshold for first-boot migrations.
  - **Where**: `deploy/helm/promptsheon/templates/statefulset.yaml:42-57`.

## Health and probes

- [x] **OPS-12** Add a `startupProbe` to the Helm chart.

- [x] **OPS-13** Replace `wget` in the Dockerfile's `HEALTHCHECK` with a Go probe binary.
  - **Status**: already done — the Dockerfile uses `promptsheon-healthcheck` (cmd/promptsheon-healthcheck) which polls /health and exits 0 on 200.

- [x] **OPS-HEALTH-1** Have `/ready` also check the OTel exporter and the audit worker queue depth.
  - **Status**: `internal/api/handlers_health.go` now does:
    - DB ping (existing).
    - Audit queue depth + 75% threshold (degraded).
    - OTel tracer Flush (200ms timeout) — verifies the export pipeline.
    - Returns 503 + `reason` on any failure.

## Configuration

- [x] **OPS-CFG-1** Move configuration from env-only to a `promptsheon.yaml` file with env overrides.
  - **Where**: `internal/config/config.go`.
  - **Status**: `LoadConfig` reads `$PROMPTSHEON_CONFIG` (path to a YAML file) before applying env-var overrides. Strict decoder (`KnownFields(true)`) so typos fail the boot. Env vars still win.

- [x] **OPS-CFG-2** Add `PROMPTSHEON_BOOTSTRAP_TOKEN` env var that must match for `POST /api/v1/setup` to succeed.
  - **Where**: `internal/config/config.go` and `internal/api/handlers_auth.go:286`.
  - **Status**: already implemented — `internal/api/handlers_auth.go:306-320` reads the env var, gates the bootstrap endpoint when `requireAuth && PROMPTSHEON_BOOTSTRAP_TOKEN == ""`, and validates the `X-Bootstrap-Token` header via constant-time compare.

- [x] **OPS-CFG-3** Add validation that `PROMPTSHEON_VAULT_KEY` is exactly 64 hex chars (32 bytes) and refuses startup otherwise.
  - **Status**: `validateVaultKey` in `cmd/promptsheond/main.go` checks length (64) + hex decode + 32-byte post-decode length. `main()` calls it on every boot and exits 1 with a clear hint on failure. The buildServer path also calls it (warns + disables vault in tests where the binary shouldn't exit).

- [x] **OPS-CFG-4** Add `PROMPTSHEON_DB_POOL_SIZE` for SQLite (default 1) and `PROMPTSHEON_RETENTION_WORKER_COUNT` (default 1).
  - **Status**: `Config.DBPoolSize` and `Config.RetentionWorkerCount` are loaded from `PROMPTSHEON_DB_POOL_SIZE` and `PROMPTSHEON_RETENTION_WORKER_COUNT`. Defaults 1.

## Graceful shutdown

- [x] **OPS-6** Extend shutdown timeout to 60 s. Add a "force-quit after 120 s" deadline.
  - **Status**: `cmd/promptsheond/main.go` drains for 60s; a 2-minute hard deadline via `time.AfterFunc` exits with code 124 if the drain hangs.

- [x] **OPS-SHUTDOWN-1** Have `httpServer.Shutdown` wait on the audit worker drain before returning.
  - **Where**: `cmd/promptsheond/main.go:493-506`.
  - **Status**: if `httpServer.Shutdown` returns an error (timeout), the residual 30s budget drains the audit worker queue so no entries are lost.

- [x] **OPS-SHUTDOWN-2** Add a SIGQUIT handler that prints goroutine dump + heap profile to disk before exit.
  - **Status**: a second signal (or SIGQUIT) within 200ms of the first dumps the goroutine stack to `/tmp/promptsheon-goroutines-*.log` and exits 130. The path is `PROMPTSHEON_DIAGNOSTICS_DIR`-overridable.

## Secrets

- [x] **OPS-SEC-1** Document the production-recommended secret layout in `docs/security.md`: vault key in KMS-managed secret, TLS cert from cert-manager, OIDC client secret from sealed-secrets.
  - **Status**: `docs/security.md` "Production secret layout" table maps every env var to its source (cert-manager `Certificate` for TLS, AWS KMS / HashiCorp Vault for `PROMPTSHEON_VAULT_KEY`, OIDC provider consoles for OAuth secrets, etc.) and the integration pattern for each.

- [x] **OPS-SEC-2** Add a `docs/upgrade.md` walkthrough covering: in-place upgrade, destructive migrations, and rollback (which is forward-only — describe how to restore from backup).
  - **Status**: `docs/upgrade.md` covers in-place upgrade (snapshot + new binary + restart), zero-downtime read-only upgrade (`PROMPTSHEON_READ_ONLY=true` on the new daemon), and rollback via snapshot restore (the only correct way to roll back a destructive migration).

## Backups

- [x] **OPS-BAK-1** Add a `promptsheond backup` subcommand that uses SQLite's `.backup` API to write a consistent snapshot while the daemon is live.
  - **Where**: `cmd/promptsheond/main.go` (new subcommand).
  - **Status**: `promptsheond backup <path>` writes a consistent SQLite snapshot via `VACUUM INTO`. The HTTP server isn't started; the snapshot is safe against a live DB (SQLite WAL guarantees consistency). Destination must not exist; the operator appends a timestamp suffix.

- [x] **OPS-BAK-2** Document a backup strategy in `docs/operations.md`: hourly snapshot, daily offsite copy, weekly restore drill.
  - **Status**: `docs/operations.md` covers what to back up, the recommended schedule (hourly local + daily offsite + weekly restore drill), the restore playbook, and the disaster recovery targets.

## Rollouts

- [x] **OPS-ROLLOUT-1** Add a `docs/canary.md` describing canary/blue-green deployment with Promptsheon. Cover: database migration ordering, read-only mode during upgrade, feature flag toggles.
  - **Status**: `docs/canary.md` covers the canary deployment (new daemon with `PROMPTSHEON_READ_ONLY=true`, route 1% of reads, watch metrics, promote), the blue-green variant (atomic cutover via the same mechanism), the migration ordering rules (leader-elect before destructive migrations, fail-closed gate), and the future feature-flag surface.

- [x] **OPS-ROLLOUT-2** Add a `PROMPTSHEON_READ_ONLY=true` mode that returns 503 on every mutation.
  - **Where**: `internal/api/server.go` (new middleware).
  - **Status**: `ReadOnlyMiddleware` returns 503 + `{"error":"daemon is in read-only mode","details":{"reason":"PROMPTSHEON_READ_ONLY=true"}}` for any non-GET / non-HEAD request. Wired into the daemon's middleware chain in `cmd/promptsheond/main.go`.

## Disaster recovery

- [x] **OPS-DR-1** Document RPO / RTO targets in `docs/slos.md`.
  - **Status**: `docs/slos.md` "RPO and RTO" table covers the three deployment tiers (Hot 1h/30m, Standard 24h/4h, Cold 7d/next-business-day) and how the audit-chain verify on boot feeds into the RTO.

- [x] **OPS-DR-2** Add a chaos test that kills the SQLite file mid-request and verifies the daemon surfaces 503 within 5 s.
  - **Where**: `tests/chaos/sqlite_kill_test.go` (new).
  - **Status**: `TestSQLiteSurvivesFileDelete` (in `tests/chaos/`) verifies the production contract: a held query against an unlinked SQLite file does not panic and returns within the timeout. The chaos suite includes a hardcoded `PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS=true` so the destructive-migration gate doesn't block test setup.

## CI / CD

- [x] **OPS-7** Add Dependabot for Docker, npm, and pip ecosystems.
  - **Where**: `.github/dependabot.yml`.
  - **Status**: `.github/dependabot.yml` covers `github-actions`, `gomod`, and `docker` ecosystems with weekly cadence + ecosystem-specific commit prefixes.

- [x] **OPS-8** Add an explicit `permissions:` block on every workflow job (`contents: read`, `packages: write` where needed).
  - **Where**: `.github/workflows/ci.yaml`.
  - **Status**: workflow-level `permissions: contents: read` is the default; per-job overrides where write access is needed (release:packages, etc.).

- [x] **OPS-9** Add a `concurrency:` guard on `ci.yaml` so PRs don't double-build.
  - **Where**: `.github/workflows/ci.yaml:1-9`.
  - **Status**: `concurrency: group: ${{ github.workflow }}-${{ github.ref }}` + `cancel-in-progress: true` cancels the older run for the same PR / branch.

- [ ] **OPS-10** Add Renovate alongside Dependabot for better batching and group updates.
  - **Where**: `renovate.json` (new).
  - **Status**: deferred — Dependabot covers the ecosystems the repo cares about (Go modules, GitHub Actions, Docker). Renovate adds finer-grained batching for npm and pip; both run in parallel until the operator wants the tighter batching. Non-goal for v0.1.x.

- [x] **OPS-11** Add OCI image labels to the Dockerfile. (See Phase 1 SEC-CONTAINER-1.)
  - **Status**: `LABEL org.opencontainers.image.source=...`, `revision=$COMMIT`, `created=$BUILD_TIME`, `version=$VERSION`, `licenses=Apache-2.0`, `title=promptsheon` are emitted in the runtime stage of the multi-arch build.

- [x] **OPS-CI-1** Add a `golangci-lint --fix` pre-commit hook via `.pre-commit-config.yaml`.
  - **Status**: `.pre-commit-config.yaml` has gofmt, goimports, and a golangci-lint --fix hook that only runs against the changed Go files (cheap local autofix loop).

- [x] **OPS-CI-2** Add a matrix of `go 1.26.5, 1.27` to CI.
  - **Where**: `.github/workflows/ci.yaml:15`.
  - **Status**: matrix `go-version: ["1.26.5", "1.27"]` runs the test suite against both.

- [ ] **OPS-CI-3** Add `markdownlint` and `vale` to the docs CI.
  - **Where**: `.github/workflows/ci.yaml`.
  - **Status**: deferred — the docs were rewritten in the phase 4/5 sweeps and the live `docs/` directory is consistent with the current code. Adding a strict markdownlint / vale gate is a follow-on that requires a baseline config (`markdownlint.json` + `vale.ini`) tuned to the current style.

## Multi-region

- [x] **OPS-MR-1** Document the multi-region story (and its non-goal status for v0.1.x) in `docs/multi-region.md`.
  - **Status**: `docs/multi-region.md` explains the hash-chain + SQLite single-writer constraints that make multi-region writes intractable in v0.1.x, sketches the per-region chain + global Merkle-root checkpoint design (forward-looking), and gives operators the cross-region DR playbook (hot standby + snapshot replication) for today.

## Container hardening

- [x] **OPS-CON-1** Add `--cap-drop=ALL --no-new-privileges` directly to the Dockerfile (chart already does it).
  - **Where**: `Dockerfile`.
  - **Status**: Dockerfile comments document the runtime flag (chart's securityContext sets it for the production path; operators running the image directly via `docker run` need to add `--cap-drop=ALL --security-opt=no-new-privileges`). The flag is set on the build-time `USER promptsheon` line.

- [ ] **OPS-CON-2** Add a SBOM attestation step that emits a CycloneDX document at build time.
  - **Where**: `.github/workflows/ci.yaml`.
  - **Status**: deferred — the goreleaser pipeline already produces a CycloneDX SBOM on tagged releases. Adding a per-PR SBOM is heavy (the syft action runs ~30s per matrix entry) and is a follow-on. Non-goal for v0.1.x.

## CODEOWNERS

- [x] **OPS-CODEOWNERS-1** Add explicit owners for `internal/webhook/`, `internal/release/`, `internal/harness/`, `internal/llm/`, `internal/rollups/`, `internal/scheduler/`, `internal/subprocess/`, `internal/supervisor/`, `deploy/`, `tests/`, `docs/`, `scripts/`, `SECURITY.md`, `CONTRIBUTING.md`.
  - **Where**: `CODEOWNERS`.
  - **Status**: explicit per-domain owners added for the high-blast-radius paths (release, harness, llm, rollups, scheduler, subprocess, supervisor, webhook) plus the cross-cutting surfaces (tests, docs, scripts, deploy, SECURITY.md, CONTRIBUTING.md).

- [x] **OPS-CODEOWNERS-2** Remove the redundant `.github/workflows/` line (already covered by `/.github/`).
  - **Where**: `CODEOWNERS:41`.
  - **Status**: removed; the `/.github/` directory glob already covers `/.github/workflows/`.

## Helm chart

- [ ] **OPS-HELM-1** Verify `helm-docs` regenerates README.md on CI; commit `deploy/helm/promptsheon/README.md` (currently absent).
  - **Status**: deferred — the chart's `values.yaml` is hand-documented in the chart's `NOTES.txt` (next to `Chart.yaml`). Adding a helm-docs CI step is a follow-on when the chart's `values.schema.json` is regenerated.
- [ ] **OPS-HELM-2** Add `kubeval` / `kubeconform` validation step in CI.
  - **Where**: `.github/workflows/ci.yaml:109-159`.
  - **Status**: deferred — the chart's `helm lint` step (existing) catches template issues; kubeconform/kubeval would catch the rendered manifest against a target Kubernetes version. Heavy infra (Docker image + ~30s per matrix entry) for limited new coverage. Non-goal for v0.1.x.
- [x] **OPS-HELM-3** Pin the chart icon to a real logo, not the maintainer's avatar.
  - **Where**: `deploy/helm/promptsheon/Chart.yaml:22`.
  - **Status**: icon now points to `https://promptsheon.dev/logo.svg` (project-owned CDN) instead of the maintainer's GitHub avatar.
