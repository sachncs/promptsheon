# Design Decisions

This page is a curated, user-facing index of the design
decisions behind Promptsheon. The full set of architectural
decision records (ADRs) lives in [docs/adr/](adr/) and is
authoritative; this page is the summary you read first.

## Capability-as-immutable-Manifest

A Capability is a name. A Version is an immutable Manifest.
A Release points a Version at an Environment. The Manifest is
the single source of truth for the bytes of every artifact
the Capability consumes.

[ADR 0010](adr/0010-version-is-a-manifest-not-a-bundle.md) is
the design record. The legacy "bundle" model (a Version
carried inline JSON for every artifact) was removed in
v0.1.0; today's Version is a Manifest of `(kind, hash)`
references.

[ADR 0023](adr/0023-forward-only-cleanup.md) is the migration
record — what was removed, what replaced it, and the rationale
for not keeping a deprecation shim.

## Content-addressed storage

Every artifact is keyed by its SHA-256. Two Capabilities
that share the same Manifest share the same underlying
artifacts; the CAS is a hash table. [ADR 0001](adr/0001-use-cas-for-prompt-history.md)
is the rationale.

The CAS lives at `pkg/cas/`, a stable public package
intended to be importable by external Go projects.

## Hash-chained audit log

The audit log is a hash chain, not an append-only list.
[ADR 0003](adr/0003-hash-chained-audit-log.md) is the design
record. `GET /api/v1/audit/verify` walks the chain from
rowid 1 forward and asserts the invariant.

The retention manager copies expired rows to `audit_archive`
rather than deleting them; the chain survives because the
source row is preserved. See [docs/security.md](security.md#audit-chain).

## Vault

API keys for upstream LLM providers live in an AES-256-GCM
vault with the master key sourced from
`PROMPTSHEON_VAULT_KEY` (or a `KeyProvider` backed by AWS
KMS, HashiCorp Vault, etc. for production).
[ADR 0004](adr/0004-aes-256-gcm-vault.md) is the rationale.

## Webhook hardening

Webhooks are signed with HMAC + a timestamp (5-minute replay
window). The URL is validated to refuse non-HTTPS to
non-private / non-loopback / non-link-local / non-multicast /
non-unspecified addresses. [ADR 0005](adr/0005-hmac-webhooks-with-ssrf-allowlist.md)
is the design record.

## Modernc SQLite

`modernc.org/sqlite` is the SQLite driver — pure Go, no CGo.
[ADR 0006](adr/0006-modernc-sqlite-no-cgo.md) is the
rationale. v0.1.x is SQLite-only.

## slog as observability foundation

Every log line is JSON via `log/slog`. [ADR 0007](adr/0007-slog-as-observability-foundation.md)
is the design record. The SSE log stream hub (`internal/ws`)
uses the same slog chain so `slog.Default()` flows to both
stderr and `/api/v1/logs/stream`.

## Workflow DAG

Workflows are sequences of `Step`s executed in declaration
order. [ADR 0008](adr/0008-workflow-dag-with-topological-execution.md)
is the design record. Each step is `{ID, Tool, Input, Output?}`;
the Workflow runtime is in `internal/workflow`.

## Capability Service Level Objectives

Three first-class SLOs ship with the project. [ADR 0020](adr/0020-capability-slos.md)
is the design record; [docs/slos.md](slos.md) is the user
guide. The Prometheus alert definitions live in
`deploy/prometheus/promptsheon-alerts.yaml`.

## Approval workflow

`MakerCheckerPolicy` (default) self-enforces separation of
duties: the Release's creator cannot vote to approve their
own release. [ADR 0011](adr/0011-release-and-approval-aggregates.md)
is the design record. The alternative `MajorityPolicy` is a
flat count-based threshold.

## Recommendation engine

A deterministic rules engine (`internal/optimizer/rules`)
plus a Thompson Sampling bandit (`internal/bandit`). Two
phases: v0.1.x ships the rules engine; the bandit selector
is a follow-on. [ADR 0021](adr/0021-bandit-foundation.md) is
the bandit design record.

## Plugin supervisor

In-process plugins (PII redactor, prompt-injection detection)
ship as built-ins; remote plugins are subprocess binaries that
implement the gRPC-over-UDS `PluginServer` contract
([ADR 0025](adr/0025-pluginproto.md)) or the net/rpc-over-UDS
fallback for v0.1.x
([ADR 0024](adr/0024-plugin-transport-uds.md)).
[ADR 0022](adr/0022-plugin-manifest.md) is the manifest
format.

## Deferred items

The architecture review board's deferred work
([ADR 0019](adr/0019-deferred-items.md)) includes the
Postgres backend (removed in v0.1.0), the bandit
recommender (v0.4), and the full DAG workflow runtime with
parallel branches and `depends_on`. Items in this ADR are
informational only — operators wanting them can track the
GitHub issues.

## See also

- [docs/architecture.md](architecture.md) — the system
  diagram.
- [docs/adr/](adr/) — the full ADR index.