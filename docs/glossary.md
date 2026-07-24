# Glossary

## Workspace

The top-level tenant boundary. Every Capability, Project,
Release, audit entry, and Budget/Quota is scoped to a
Workspace. A single daemon can host many Workspaces; the
auth layer is workspace-scoped so a non-admin user can only
see their own Workspace's data.

## Project

A logical grouping of Capabilities under a Workspace. Useful
for "summariser team owns these 5 capabilities" or "billing
unit A is the production stack". Projects are mutable; a
Capability can move between Projects.

## Capability

A named logical capability. A Capability has zero or more
**Versions** (immutable builds) and a manifest of content-
addressed artifacts. A Capability by itself is just a
container — to do work, you point a Release at one of its
Versions.

## Version

An immutable build of a Capability's **Manifest**. Versions
are content-addressed: a new Version is just a new Manifest
hash, so two Capabilities that share the same Manifest hash
share the same underlying artifacts.

## Manifest

The content-addressed composition that defines a Version.
A Manifest references five required artifacts by `(kind, hash)`:

- `prompt` — the prompt text.
- `model_policy` — provider + model + revision + defaults.
- `runtime_policy` — max_output_tokens, temperature, top_p.
- `context_contract` — the per-workspace Context assembly.
- `memory` — the per-workspace Memory shape.

Optional artifacts: `guardrails` (Guardrail references),
`tools` (Tool references), `knowledge_sources` (knowledge
graph), `mcp_servers` (MCP server allowlist).

## Release

A pointer from a Version to a tenant Environment. A
Capability's live Release per Environment is the *current*
Release; Activate transitions Pending → Active and supersedes
the prior Active Release in the same Environment.

## Manifest Hash

The SHA-256 of the canonical encoding of a Manifest.
Computed at Version creation; stored alongside the Version
and used as a deduplication key.

## Model Policy

The artifact at the Manifest's `model_policy` reference.
Resolves provider + model + revision at release time. The
Release's runtime is the *resolved* plan; the request body
must NOT override the release's runtime.

## CAS (Content-Addressable Storage)

The pkg/cas/ package. Every artifact is a content-addressed
blob keyed by its SHA-256 hash. The CAS is the deduplication
primitive: two Manifests that reference the same hash share
the same blob.

## Precondition

A named command hook on a Capability. `Activate` runs every
enabled Precondition before transitioning the Release. A
failing Precondition returns 409 and leaves the Release in
`pending`.

## Dataset

A named collection of `(inputs, expected)` test cases. The
ground truth for harness eval. A Dataset belongs to a
Capability; a Release evaluates against a Dataset via an
EvalRun.

## EvalRun

A recorded scoring of a Release against a Dataset using a
chosen Scorer. The status is `passed`, `failed`, `running`,
or `error`. Per-case results are persisted alongside the
aggregate.

## Scorer

The scoring strategy. v0.2.0 ships:
- `exact_match`
- `contains`
- `regex`
- `json_schema`

## Approval Policy

`MajorityPolicy` (flat count-based) or `MakerCheckerPolicy`
(creator cannot approve their own release; configurable
`RequiredApprovers`). Set via `PROMPTSHEON_APPROVAL_POLICY`.

## Audit Chain

The hash-chained audit log. Each row records
`previous_hash` and `entry_hash` (sha256 of the row's
canonical JSON). `GET /api/v1/audit/verify` walks the chain
from rowid 1 forward and asserts the invariant.

## Recommendation

A suggestion produced by the rules engine
(`internal/optimizer/rules`) or the bandit selector
(`internal/bandit`). Recommendations are stored with a
type (raise_max_tokens, drop_guardrail, change_temperature)
and a target Release.

## SLO (Service Level Objective)

A target on a metric (latency p95, error rate, eval
pass-rate) over a window. Three first-class SLOs ship with
the project. See [docs/slos.md](slos.md).

## Plugin

A `supervisor.Plugin` registered through the
`PROMPTSHEON_PLUGINS_FILE` manifest. Plugins can be remote
binaries (gRPC over UDS) or in-process built-ins (PII
redaction, prompt-injection detection).