# Design Decisions

This page is a curated summary of the key design decisions
behind Promptsheon.

## Capability-as-immutable-Manifest

A Capability is a name. A Version is an immutable Manifest.
A Release points a Version at an Environment. The Manifest is
the single source of truth for the bytes of every artifact
the Capability consumes. The legacy "bundle" model (a Version
carried inline JSON for every artifact) was removed in the
forward-only v0.1.0 cleanup; today's Version is a Manifest
of `(kind, hash)` references.

## Content-addressed storage

Every artifact is keyed by its SHA-256. Two Capabilities
that share the same Manifest share the same underlying
artifacts; the CAS is a hash table.

## Hash-chained audit log

The audit log is a hash chain, not an append-only list.
`GET /api/v1/audit/verify` walks the chain from rowid 1
forward and asserts the invariant. The retention manager
copies expired rows to `audit_archive` rather than deleting
them; the chain survives because the source row is preserved.
See [docs/security.md](security.md#audit-chain).

## Vault

API keys for upstream LLM providers live in an AES-256-GCM
vault with the master key sourced from
`PROMPTSHEON_VAULT_KEY` (or a `KeyProvider` backed by AWS
KMS, HashiCorp Vault, etc. for production).

## Webhook hardening

Webhooks are signed with HMAC + a timestamp (5-minute replay
window). The URL is validated to refuse non-HTTPS to
non-private / non-loopback / non-link-local / non-multicast /
non-unspecified addresses.

## Modernc SQLite

`modernc.org/sqlite` is the SQLite driver — pure Go, no CGo.
The shipped configuration is SQLite-only.

## slog as observability foundation

Every log line is JSON via `log/slog`. The SSE log stream hub
uses the same slog chain so `slog.Default()` flows to both
stderr and `/api/v1/logs/stream`.

## Workflow DAG

Workflows are sequences of `Step`s executed in declaration
order. Each step is `{ID, Tool, Input, Output?}`; the
Workflow runtime is in `backend/workflow`.

## Capability Service Level Objectives

Three first-class SLOs ship with the project. See
[docs/slos.md](slos.md) for the user guide. The Prometheus
alert definitions live in
`deploy/prometheus/promptsheon-alerts.yaml`.

## Approval workflow

`MakerCheckerPolicy` (default) self-enforces separation of
duties: the Release's creator cannot vote to approve their
own release. The alternative `MajorityPolicy` is a flat
count-based threshold.

## Recommendation engine

A deterministic rules engine (`backend/optimizer`)
plus a Thompson Sampling bandit (`backend/bandit`). Both
ship today and close the loop: production telemetry →
Observation → rules/bandit → Recommendation → Decision →
next Version.

## Plugin supervisor

In-process plugins (PII redactor, prompt-injection detection)
ship as built-ins; remote plugins are subprocess binaries that
implement the gRPC-over-UDS `PluginServer` contract or the
net/rpc-over-UDS fallback.

## See also

- [docs/architecture.md](architecture.md) — the system
  diagram.
