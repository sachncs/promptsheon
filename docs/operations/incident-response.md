# Incident response

This runbook covers the first response for a self-hosted Promptsheon node. It
assumes an operator has shell access to the installation and can inspect
structured logs. Never paste API keys, vault material, authorization headers,
or raw prompts into an incident ticket.

## First five minutes

1. Record the UTC start time, deployment SHA, migration version, host, and
   affected organization(s).
2. Check process and dependency state:

   ```bash
   curl -fsS http://127.0.0.1:8080/api/health
   curl -fsS http://127.0.0.1:8080/api/ready
   ```

3. Check queue depth/age in the Operations page or
   `GET /api/execution-jobs/metrics` using an administrator session.
4. Check structured logs for `server.started`, `execution.failed`, evidence
   writer failures, circuit-breaker transitions, and shutdown/drain events.
5. Declare the incident if the issue affects availability, tenant isolation,
   evidence integrity, release routing, or the configured error budget.

## Symptom playbooks

### Readiness is failing

- If health is failing, inspect SQLite availability and filesystem capacity.
- If health is healthy but readiness is failing, the process is draining or the
  database probe is unavailable; do not send new traffic to the node.
- Restore service only after `/api/ready` returns `200` and the deployment
  supervisor reports the process as accepting traffic.

### Queue age or pending jobs are growing

- Compare queued, running, and oldest queued time.
- Check provider/tool circuit breakers, worker count, SQLite contention, and
  budget-exhaustion rates together.
- Do not raise the queue limit as the first response. Cancel only explicitly
  approved work, and preserve idempotency keys for retries.
- After remediation, verify queue age falls and no jobs remain stranded in
  `running` beyond the lease window.

### Evidence lag or drops are reported

- Inspect evidence sink queue, dropped, accepted, and writer-failure metrics.
- Critical policy/error/release events must be present after a flush; compare
  the execution timeline with `GET /api/evidence`.
- Do not delete evidence during the incident. Use the retention workflow only
  after the incident owner approves the scope.

### A release is unhealthy

- Stop promotion/canary changes and identify the exact immutable manifest hash.
- Use the release rollback endpoint or console action; do not edit lifecycle
  state directly in SQLite.
- Verify the active release resolves to the expected hash and confirm the
  rollback transition appears in the audit chain.

### Suspected secret or tenant-isolation incident

- Treat it as a security incident: restrict access, preserve logs, and rotate
  the affected credential through the vault/provider control plane.
- Do not reproduce with a real secret. Use the simulator and redaction tests.
- Verify organization-scoped queries, API-key revocation, and audit records
  before restoring access.

## Recovery and verification

1. Capture a verified SQLite backup before destructive repair:

   ```bash
   PROMPTSHEON_DB_PATH=/var/lib/promptsheon/promptsheon.db \
   PROMPTSHEON_BACKUP_PATH=/var/backups/promptsheon/latest.sqlite \
   pnpm --dir packages/server db:backup
   ```

2. Verify CAS objects before restoring or promoting a node:

   ```bash
   PROMPTSHEON_CAS_PATH=/var/lib/promptsheon/.promptsheon \
   pnpm --dir packages/server cas:verify
   ```

3. Run the relevant server, evidence, queue, backup, and CAS tests. Repeat
   `/api/health` and `/api/ready` checks, then execute the simulator journey.
4. Compare the recovered release hash, audit-chain head, evidence counts, and
   queue metrics with the incident record.

## Closure

Close only when impact, timeline, root cause, detection gap, remediation, and
follow-up owner are recorded. Attach benchmark or recovery evidence and note
any accepted SLO or data-retention deviation.
