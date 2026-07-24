# Canary and Blue-Green Deployment

Promptsheon's `PROMPTSHEON_READ_ONLY` mode makes the daemon
safe to run against live read traffic before writes are
enabled. Combined with the versioned release lifecycle, this
gives operators a clean canary / blue-green path.

## Canary

1. **Deploy the new version as a sidecar** with
   `PROMPTSHEON_READ_ONLY=true` on the same data directory as
   the current version. The new daemon answers reads; the
   old daemon continues to handle writes.

2. **Route a fraction of read traffic to the new daemon**
   (e.g. 1% via a weighted ingress). Watch the metrics:

   - `promptsheon_http_request_duration_seconds` — is the new
     daemon p99 in line with the old?
   - `promptsheon_audit_dropped_total` — non-zero means the
     new daemon's audit worker is saturated.
   - `promptsheon_llm_calls_total` — does the new daemon
     actually drive the LLM path, or is it failing before
     reaching the provider?

3. **Run the eval suite against the new daemon.** The
   harness-engineering primitive (`promptsheon eval run
   <release> <dataset>`) gives a deterministic pass/fail
   signal — if the new version regresses on the existing
   dataset, the canary fails before traffic shifts.

4. **Promote the new daemon.** Set its
   `PROMPTSHEON_READ_ONLY=false`, drain the old daemon's
   in-flight writes, and shift ingress weight to 100%.

5. **Retire the old daemon.** With the new version
   validated and the old drained, the old binary is a
   rollback target if the new version regresses within the
   next hour.

## Blue-green

The canary pattern is a probabilistic version of blue-green.
For deployments that need an atomic cutover, the same
mechanisms work in blue-green mode:

- Run two parallel deployments (old + new) with
  `PROMPTSHEON_READ_ONLY=true` on the new.
- Verify both daemons serve identical read responses for
  a sample of requests.
- Atomic cutover: flip the ingress to the new daemon, drain
  the old.

The `PROMPTSHEON_LEADER_ELECTION=true` flag (ADR-0030) keeps
the audit chain and migration application single-writer across
replicas. Production tenants that need blue-green typically
run the old + new as a single leader-eligible pair during the
cutover, then drop the old replica after the new one is
elected.

## Feature flags

In v0.2.0 the only feature flag is `PROMPTSHEON_READ_ONLY`.
A future flag surface — `PROMPTSHEON_FEATURE_<name>` — is a
follow-on. Today, operators who need to disable a specific
route while upgrading (e.g. to avoid the rate limiter
behaviour) configure the route via the existing Options in
`internal/api/server.go`.

## Database migration ordering

Migrations are versioned. The daemon applies every
migration greater than the highest version in `migrations`
table, in order. Two daemons racing on the same `promptsheon.db`
will race on the migrations table; enable
`PROMPTSHEON_LEADER_ELECTION=true` so only the leader applies
migrations. The leader-election layer is documented in
ADR-0030.

Destructive migrations (filename `*_destructive_*.sql`) are
gated on `PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS=true`. The
gate is fail-closed: the daemon refuses to start with a
pending destructive migration unless the env var is set.

## See also

- [docs/upgrade.md](upgrade.md) — in-place upgrade + restore
  from snapshot.
- [docs/multi-region.md](multi-region.md) — multi-region
  replication story.
- [docs/operations.md](operations.md) — backup / restore.