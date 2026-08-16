# Limitations

This document lists the things `promptsheon` deliberately does not
do, the things it cannot do, and the things it might do in a future
release. Operators planning a deployment should read this before
choosing a topology, sizing hardware, or integrating with an
existing observability stack.

## Storage

- **SQLite only.** The daemon is wired to a single SQLite database
  file. There is no MySQL or PostgreSQL backend today; the only
  multi-process path is to mount the same database file on a shared
  volume, which the project does not recommend for production.
- **Single-writer.** SQLite's writer serialises every mutation. The
  audit chain, recommendation ledger, and webhook outbox all go
  through the same writer goroutine. Heavy multi-tenant loads with
  concurrent release activity will queue on this writer.
- **WAL mode by default.** The daemon opens the database in
  `journal_mode=WAL` so readers do not block writers. The WAL file
  is co-located with the database and is not separately
  backed up; operators who copy the database file while the
  daemon is running must include the WAL file to capture the
  in-flight transactions.
- **No online migration rollback.** Schema migrations are
  forward-only. A failed migration halts the daemon at boot. The
  recovery path is to restore from the most recent backup that
  predates the failed migration.

## Scalability

- **Single-node only.** The project does not support clustering,
  leader election across nodes, or geo-replicated writes. The
  embedded `election` package models a single-leader state machine
  for a single host, not a multi-host consensus protocol.
- **No horizontal scale-out.** A single daemon is the unit of
  scale. Operators can run multiple daemons behind a load balancer
  only if each daemon has its own database file (the dashboards,
  release history, and audit chains will diverge).
- **API throughput is bounded by SQLite.** A modern developer
  laptop handles ~3–5k QPS on the read path and ~500 QPS on the
  write path before the SQLite writer saturates. Sustained
  write-heavy traffic is the first thing to break.
- **Recommendation engine is in-process.** Every recommendation
  request runs a Thompson-sampling bandit against the in-memory
  state. Multi-replica deployments would not see consistent
  recommendation results across replicas.

## Single-region only

- **No multi-region deployment.** The audit chain is anchored to
  a single database file; replicating it across regions is out of
  scope. A future design note in
  `docs/operations/multi-region.md` sketches the read-replica shape
  the project would take, but no implementation work has started.
- **No geo-failover.** If the host running the daemon goes down,
  the database file is unavailable until the host is restored or
  the file is mounted elsewhere. There is no automated failover
  story.
- **Clock-skew sensitivity.** The audit chain and event-bus
  ordering depend on the host's wall clock. A host with a clock
  that jumps backward (e.g. NTP correction) may produce
  out-of-order audit rows; the chain is still verifiable because
  it includes a sequence number, but the timestamps will not
  match the wall clock at the moment of the event.

## Single language

- **Go SDK only.** The repository ships a Go client under
  `pkg/promptsheon/`. There is no Python, TypeScript, or any
  other language SDK. The previous `sdk/python/` and
  `sdk/typescript/` directories were removed in v1.0.0 because
  they contained only a copy of the OpenAPI spec and no client
  code; consumers who want a client in another language should
  generate one from `promptsheon/spec/spec.yaml` themselves
  (the spec is committed and the CI gate keeps it in sync).
- **No C or C++ API surface.** The daemon does not expose a
  CGo interface; embedding the daemon in a non-Go process is
  not supported.

## Network and transport

- **No TLS terminator.** The daemon speaks plain HTTP by default.
  Operators front the daemon with a TLS terminator (nginx, Envoy,
  Caddy, a cloud load balancer) and configure
  `PROMPTSHEON_TRUSTED_PROXIES` to forward `X-Forwarded-For`.
- **No gRPC server.** The OpenAPI document covers the entire
  public surface; the daemon does not implement the spec in
  gRPC. A gRPC adapter is on the v0.4.0 roadmap but is not
  shipped today.
- **WebSocket is read-only.** The `/api/v1/ws` endpoint streams
  events from the bus to subscribed clients; it does not accept
  client-originated messages. Operators who want a WebSocket
  command surface must run a separate gateway.
- **Webhook delivery is at-least-once with no global dedup.**
  A webhook receiver must be idempotent. The dispatcher
  retries with exponential backoff up to the configured max
  attempts and then marks the delivery as `failed`; the audit
  chain records every attempt.

## Observability

- **OpenTelemetry traces only.** The daemon emits OTLP traces via
  gRPC. There is no native Prometheus exposition endpoint for the
  internal metrics bus; the `/api/v1/metrics` route returns a
  JSON summary, not a Prometheus text format. Operators who
  want Prometheus should run an OTLP collector that converts
  traces to metrics, or use the JSON summary and scrape it
  with a JSON-exporter.
- **Structured logs only.** Logs are emitted as
  `log/slog` JSON to stdout. There is no syslog, journald, or
  file-rotation support; operators run the daemon under a
  log-rotation tool (fluentbit, vector, promtail) that consumes
  stdout.
- **No remote write for the audit chain.** The audit chain is
  persisted to the same SQLite database as the rest of the
  state. Operators who want an external witness (a hash
  published to a transparency log) must run a periodic export
  job; the daemon does not ship one.

## Security

- **No built-in rate limiting per user.** The daemon has a
  global per-IP token-bucket rate limiter; per-user or
  per-API-key limits are not implemented. Operators who need
  fine-grained limits must front the daemon with an API
  gateway.
- **OAuth scopes are coarse.** The current OAuth model grants a
  single role per identity (`admin`, `editor`, `viewer`); the
  scope-based ACL system is on the v0.4.0 roadmap.
- **Vault key is operator-controlled.** The on-disk encryption
  key (`PROMPTSHEON_VAULT_KEY`) is read from the environment at
  boot. Operators who want automatic key rotation must run a
  sidecar that rewrites the key and restarts the daemon; the
  daemon does not rotate the key in place.
- **Self-evolve is off by default.** The LLM-driven self-evolve
  loop is gated behind `PROMPTSHEON_SELF_EVOLVE=true` and is
  documented as a v0.3.0+ feature. Operators who enable it
  accept that the daemon will issue LLM calls against the
  configured provider on a schedule.

## Testing and certification

- **No SOC 2 / ISO 27001 attestation.** The project is
  open-source software without a formal certification; the
  `SECURITY.md` document describes the threat model and
  disclosure process but not a compliance regime.
- **No FIPS-140 mode.** The daemon does not pin to a FIPS
  cryptographic module. The vault uses AES-GCM via
  `crypto/aes` and `crypto/cipher`; on a host with the
  BoringCrypto Go runtime the cipher is FIPS-validated, but
  the daemon does not enforce that runtime.
- **No load test SLA.** The k6 scenarios under
  `tests/load/scenarios/` are starting points, not
  performance contracts. The nightly load run is informational;
  a release is not gated on a specific p99 number.

## Future work

The above list is the current limitation set. Items that are
likely to change are tracked in `ROADMAP.md` under the
relevant version (v0.4.0 for the persistence and cluster work,
v0.5.0 for the marketplace and reputation work). Items that
are NOT on the roadmap today (TLS in the daemon, multi-region
writes, language SDKs) are documented here so operators do not
infer them from the README.
