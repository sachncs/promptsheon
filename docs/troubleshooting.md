# Troubleshooting

The "It works on my machine" checklist. Run each in order; the
common case is resolved by step 1 or 2.

## Daemon won't start

- **`PROMPTSHEON_AUTH=false is only valid for loopback binds`** —
  the daemon refuses to start when auth is disabled on a
  non-loopback bind. Either set `PROMPTSHEON_AUTH=true`, bind
  to `127.0.0.1`, or set `PROMPTSHEON_TLS_CERT_FILE` and
  `PROMPTSHEON_TLS_KEY_FILE`.
- **`non-loopback bind requires TLS`** — set
  `PROMPTSHEON_TLS_CERT_FILE` and `PROMPTSHEON_TLS_KEY_FILE` to
  PEM-encoded files.
- **`PROMPTSHEON_BOOTSTRAP_TOKEN required for setup` (and
  `PROMPTSHEON_AUTH=true`)** — either set
  `PROMPTSHEON_BOOTSTRAP_TOKEN` to enable the first-run
  bootstrap endpoint, or set `PROMPTSHEON_AUTH=false` (only on
  a loopback bind).
- **`failed to open promptsheon.db: permission denied`** — the
  daemon user doesn't have write access to the directory. Either
  chown the directory or set `PROMPTSHEON_DB_PATH=/var/lib/promptsheon/db`.

## Authentication issues

- **`401 Unauthorized` on every call** — your `Authorization`
  header is missing or the key was revoked. Run
  `promptsheon audit list --action apikey_revoke` to see recent
  revocations, and `promptsheon apikey create` to mint a new
  one.
- **`403 Forbidden` on a route I should have** — the key's role
  doesn't have the required permission. Run
  `promptsheon user get <user_id>` to see the role, or
  `promptsheon user update <id> '{"role":"admin"}'` to elevate.
- **`503 "setup is disabled when authentication is enabled and
  no PROMPTSHEON_BOOTSTRAP_TOKEN is set"`** — set
  `PROMPTSHEON_BOOTSTRAP_TOKEN` to enable the bootstrap
  endpoint, or use the existing admin key from
  `promptsheon apikey list --user-id <admin-id>`.

## Invoke failures

- **`502 "no LLM provider configured for this invocation"`** —
  the Release's Manifest references a provider name that isn't
  registered. `promptsheon provider list` to see registered
  names; set the corresponding `PROMPTSHEON_<NAME>_API_KEY` env
  var or use `WithProviders` on the server.
- **`502 "no LLM provider configured for this invocation" (after
  `provider test` succeeded)** — the Manifest's `model_policy`
  artifact has a `provider` value the daemon doesn't recognise.
  Inspect the Release with `promptsheon release get <id>` and
  look at the resolved plan.
- **`429 quota exceeded`** — the per-window quota is exhausted.
  Wait for the window to roll, or raise the quota.
- **`402 budget exceeded`** — the per-period USD cap is
  exhausted. The Release stays in `pending` until the next
  period or an admin raises the cap.
- **Invoke hangs and times out** — the upstream LLM provider
  is unreachable. `promptsheon provider test <name> --model
  <model>` to verify the connection; check your
  `PROMPTSHEON_<NAME>_API_KEY` and `*_BASE_URL` env vars.

## Activate failures

- **`409 with "quorum not satisfied"`** — the MakerChecker
  policy requires a non-creator Approve vote. The vote was
  cast by the same identity as the Release's `CreatedBy`. Cast
  a second vote as a different user.
- **`409 with "precondition failed"`** — one of the
  Capability's preconditions exited non-zero or timed out.
  Inspect the error message; the precondition's stdout/stderr
  is in the `details` payload.
- **`409 with "release is not active"`** — the Release's
  status is not `active`. Only Active Releases can be invoked.
  Re-run Activate.

## Audit chain

- **`ok: false, tail_mismatch: true` from `/audit/verify`** —
  rows were modified or removed out-of-order. This is a
  security event: investigate. The retention sweep archives
  old rows to `audit_archive` but does not delete them; if the
  mismatch is from a tooling issue, the operator can
  reconcile from the archive.
- **`promptsheon_audit_dropped_total` increasing** — the
  audit worker pool's bounded queue is full. The
  `PromptsheonAuditChainBroken` alert fires. Check the
  audit worker's DB write latency
  (`promptsheon_audit_queue_latency_seconds`).

## Metrics

- **`/metrics` returns 401** — the endpoint is
  `PermAuditRead`-gated. Either pass an admin API key in
  `Authorization: Bearer ...`, or bind the daemon to a
  loopback listener and rely on network controls.
- **No traces in your OTel collector** — set
  `PROMPTSHEON_OTEL_ENDPOINT=otel-collector:4317` and
  (optionally) `PROMPTSHEON_OTEL_SAMPLE_RATIO=0.1` for
  high-volume production. Check the daemon's startup log for
  "OTel tracer initialised".

## Database

- **`database is locked` errors under load** — increase the
  busy timeout. The daemon currently hardcodes
  `?_pragma=busy_timeout(5000)` in `cmd/promptsheond/main.go`;
  for higher throughput run the daemon on a Postgres backend
  instead (the SQLite-only constraint is v0.1.x).
- **Schema migration fails at boot** — the migration table
  records the highest applied version. If you jumped a
  version, run the missing migrations manually with
  `sqlite3 promptsheon.db < internal/store/migrations/0NN_*.up.sql`.
  If two replicas race on the same `promptsheon.db`, enable
  `PROMPTSHEON_LEADER_ELECTION=true` so only the leader
  applies migrations.

## LLM providers

- **`unknown provider: <name>`** — the LLM `Registry` doesn't
  have a factory for that name. v0.1.x ships with `openai` and
  `anthropic`; custom providers must be registered in
  `cmd/promptsheond/main.go` before the daemon boots.
- **`provider <name> not configured`** — the provider's API
  key env var is missing. The daemon logs a warning at boot
  for every missing key; check the startup log.

## Plugin supervisor

- **`manifest entry has no binary line`** — the manifest
  entry declared a service but no `binary:` line. The
  supervisor fails-closed: `Start/Health` return
  `errRemoteNotConfigured`, and `/metrics` surfaces the gap.
  Add a `binary: /opt/foo` line and reload the manifest.
- **Plugin binary restarts every minute** — check the
  restart budget. The default `RestartPolicy` allows 5
  restarts with 1s→30s exponential backoff.

## More

- [`docs/faq.md`](faq.md) — top-of-mind questions.
- [`docs/development.md`](development.md) — local-dev workflow.
- [`docs/security.md`](security.md) — threat model + auth
  + audit chain.