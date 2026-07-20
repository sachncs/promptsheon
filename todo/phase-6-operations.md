# Phase 6 — Operations

All operations findings. Fast forward: replace, don't shim.

## Replicas and scaling

- [ ] **OPS-1a** Remove the `replicaCount == 1` constraint from `values.schema.json`.
  - **Where**: `deploy/helm/promptsheon/values.schema.json:13`.

- [ ] **OPS-1b** Add a leader-election layer (e.g. SQLite advisory lock via `PRAGMA user_version` swap) so multi-replica is safe.
  - **Where**: new file `internal/election/sqlite.go` and `cmd/promptsheond/main.go`.

- [ ] **OPS-2a** Add an HPA template scaled on `promptsheon_http_requests_total` rate.
  - **Where**: `deploy/helm/promptsheon/templates/hpa.yaml` (new).

- [ ] **OPS-2b** Add a `PodDisruptionBudget` (already present at `pdb.yaml`) — verify `minAvailable` is sane.

- [ ] **OPS-2c** Add `topologySpreadConstraints` and `priorityClassName` for production-grade scheduling.
  - **Where**: `deploy/helm/promptsheon/templates/statefulset.yaml`.

- [ ] **OPS-2d** Add a `startupProbe` with a generous failure threshold for first-boot migrations.
  - **Where**: `deploy/helm/promptsheon/templates/statefulset.yaml:42-57`.

## Health and probes

- [ ] **OPS-12** Add a `startupProbe` to the Helm chart.

- [ ] **OPS-13** Replace `wget` in the Dockerfile's `HEALTHCHECK` with a Go probe binary.

- [ ] **OPS-HEALTH-1** Have `/ready` also check the OTel exporter and the audit worker queue depth.

## Configuration

- [ ] **OPS-CFG-1** Move configuration from env-only to a `promptsheon.yaml` file with env overrides.
  - **Where**: `internal/config/config.go`.

- [ ] **OPS-CFG-2** Add `PROMPTSHEON_BOOTSTRAP_TOKEN` env var that must match for `POST /api/v1/setup` to succeed.
  - **Where**: `internal/config/config.go` and `internal/api/handlers_auth.go:286`.

- [ ] **OPS-CFG-3** Add validation that `PROMPTSHEON_VAULT_KEY` is exactly 64 hex chars (32 bytes) and refuses startup otherwise.

- [ ] **OPS-CFG-4** Add `PROMPTSHEON_DB_POOL_SIZE` for SQLite (default 1) and `PROMPTSHEON_RETENTION_WORKER_COUNT` (default 1).

## Graceful shutdown

- [ ] **OPS-6** Extend shutdown timeout to 60 s. Add a "force-quit after 120 s" deadline.

- [ ] **OPS-SHUTDOWN-1** Have `httpServer.Shutdown` wait on the audit worker drain before returning.
  - **Where**: `cmd/promptsheond/main.go:493-506`.

- [ ] **OPS-SHUTDOWN-2** Add a SIGQUIT handler that prints goroutine dump + heap profile to disk before exit.

## Secrets

- [ ] **OPS-SEC-1** Document the production-recommended secret layout in `docs/security.md`: vault key in KMS-managed secret, TLS cert from cert-manager, OIDC client secret from sealed-secrets.
- [ ] **OPS-SEC-2** Add a `docs/upgrade.md` walkthrough covering: in-place upgrade, destructive migrations, and rollback (which is forward-only — describe how to restore from backup).

## Backups

- [ ] **OPS-BAK-1** Add a `promptsheond backup` subcommand that uses SQLite's `.backup` API to write a consistent snapshot while the daemon is live.
  - **Where**: `cmd/promptsheond/main.go` (new subcommand).

- [ ] **OPS-BAK-2** Document a backup strategy in `docs/operations.md`: hourly snapshot, daily offsite copy, weekly restore drill.

## Rollouts

- [ ] **OPS-ROLLOUT-1** Add a `docs/canary.md` describing canary/blue-green deployment with Promptsheon. Cover: database migration ordering, read-only mode during upgrade, feature flag toggles.
- [ ] **OPS-ROLLOUT-2** Add a `PROMPTSHEON_READ_ONLY=true` mode that returns 503 on every mutation.
  - **Where**: `internal/api/server.go` (new middleware).

## Disaster recovery

- [ ] **OPS-DR-1** Document RPO / RTO targets in `docs/slos.md`.
- [ ] **OPS-DR-2** Add a chaos test that kills the SQLite file mid-request and verifies the daemon surfaces 503 within 5 s.
  - **Where**: `tests/chaos/sqlite_kill_test.go` (new).

## CI / CD

- [ ] **OPS-7** Add Dependabot for Docker, npm, and pip ecosystems.
  - **Where**: `.github/dependabot.yml`.

- [ ] **OPS-8** Add an explicit `permissions:` block on every workflow job (`contents: read`, `packages: write` where needed).
  - **Where**: `.github/workflows/ci.yaml`.

- [ ] **OPS-9** Add a `concurrency:` guard on `ci.yaml` so PRs don't double-build.
  - **Where**: `.github/workflows/ci.yaml:1-9`.

- [ ] **OPS-10** Add Renovate alongside Dependabot for better batching and group updates.
  - **Where**: `renovate.json` (new).

- [ ] **OPS-11** Add OCI image labels to the Dockerfile. (See Phase 1 SEC-CONTAINER-1.)

- [ ] **OPS-CI-1** Add a `golangci-lint --fix` pre-commit hook via `.pre-commit-config.yaml`.
- [ ] **OPS-CI-2** Add a matrix of `go 1.26.5, 1.27` to CI.
  - **Where**: `.github/workflows/ci.yaml:15`.
- [ ] **OPS-CI-3** Add `markdownlint` and `vale` to the docs CI.
  - **Where**: `.github/workflows/ci.yaml`.

## Multi-region

- [ ] **OPS-MR-1** Document the multi-region story (and its non-goal status for v0.1.x) in `docs/multi-region.md`.

## Container hardening

- [ ] **OPS-CON-1** Add `--cap-drop=ALL --no-new-privileges` directly to the Dockerfile (chart already does it).
  - **Where**: `Dockerfile`.
- [ ] **OPS-CON-2** Add a SBOM attestation step that emits a CycloneDX document at build time.
  - **Where**: `.github/workflows/ci.yaml`.

## CODEOWNERS

- [ ] **OPS-CODEOWNERS-1** Add explicit owners for `internal/webhook/`, `internal/release/`, `internal/harness/`, `internal/llm/`, `internal/rollups/`, `internal/scheduler/`, `internal/subprocess/`, `internal/supervisor/`, `deploy/`, `tests/`, `docs/`, `scripts/`, `SECURITY.md`, `CONTRIBUTING.md`.
  - **Where**: `CODEOWNERS`.

- [ ] **OPS-CODEOWNERS-2** Remove the redundant `.github/workflows/` line (already covered by `/.github/`).
  - **Where**: `CODEOWNERS:41`.

## Helm chart

- [ ] **OPS-HELM-1** Verify `helm-docs` regenerates README.md on CI; commit `deploy/helm/promptsheon/README.md` (currently absent).
- [ ] **OPS-HELM-2** Add `kubeval` / `kubeconform` validation step in CI.
  - **Where**: `.github/workflows/ci.yaml:109-159`.
- [ ] **OPS-HELM-3** Pin the chart icon to a real logo, not the maintainer's avatar.
  - **Where**: `deploy/helm/promptsheon/Chart.yaml:22`.
