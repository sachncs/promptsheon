<p align="center">
  <h1 align="center">Promptsheon</h1>
  <p align="center">The Control Plane for AI Capabilities — v0.3.0</p>
  <p align="center">
    <a href="#installation"><img src="https://img.shields.io/badge/go-1.26-00ADD8?logo=go" alt="Go"></a>
    <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache%202.0-blue" alt="License"></a>
    <a href="https://github.com/sachncs/promptsheon/actions"><img src="https://img.shields.io/github/actions/workflow/status/sachncs/promptsheon/ci.yaml?branch=master" alt="CI"></a>
    <a href="https://github.com/sachncs/promptsheon/stargazers"><img src="https://img.shields.io/github/stars/sachncs/promptsheon" alt="Stars"></a>
  </p>
</p>

Promptsheon is the **AI Capability Control Plane** — the
operating system for versioning, deploying, governing,
observing, and optimising AI Capabilities at enterprise scale.

Every Capability — its Prompt, Model Policy, Runtime Policy,
Context Contract, Memory, Guardrails, Tools, MCP servers, and
Evaluation Suite — is an immutable, content-addressed Manifest
recorded as a Directed Acyclic Graph. Production tenants manage
their Capabilities the way engineers manage code: with versions,
reviews, releases, canary deployments, and rollbacks.

Beyond the deployment surface, Promptsheon ships:

- **Capability Contract** — input/output schema, SLO target,
  blast radius. The unit of governance; the contract gates
  auto-promotion by the Recommendation engine.
- **Capability Diff** — semantic diff between two Versions;
  reports added, removed, and changed artifact references.
- **Capability Catalog** — search across Workspaces.
- **Capability Reputation** — derived trust score from eval
  history, SLO adherence, and decision adoption.
- **Capability Inheritance** — a Version can declare Parents
  and inherit artifacts from another Version. Cycles and
  depth overflow are detected at create time.
- **Reasoning Compiler** — `POST /api/v1/reasoning/compile`
  turns an Intent into a CapabilityPlan (a DAG of capability
  invocations). Picks the best-fit capability against a
  catalog filtered by reputation, cost, and latency
  constraints.
- **ContinuousEval** — scheduled eval loops that run the
  active release against a dataset on a fixed cadence.
  Configured via `PROMPTSHEON_CONTINUOUS_EVAL`.
- **LLM-judge scorer** — `eval.RegisterLLMJudge(JudgeClient)`
  registers an LLM-backed scorer; the production daemon
  wires it to the live LLM gateway.
- **Partitioned rate limiter** — 16-way FNV-1a sharded
  Allow() — contention scales linearly with the shard count.
- **Postgres backend with RLS** — planned, not implemented
  in this build. The Postgres backend was removed during the
  layout migration; only SQLite ships today. A future pgx
  adapter will satisfy the `store.Repository` interface at
  `backend/store/repo.go` without touching domain packages
  (see [docs/operations/multi-region.md](docs/operations/multi-region.md)
  for the rationale).
- **Recommendation Loop** — production telemetry → Observation
  → Rules/Bandit → Recommendation → Decision → next Version.
  The loop is closed end-to-end and persists across restarts.
- **CRDT-backed Settings** — last-write-wins with version
  vectors + deterministic tie-break; convergence is pinned by
  property tests.
- **Audit Chain** — hash-chained with TLA+ spec for the
  ordering invariant; every state transition is recorded.

v0.1.0 is the forward-only baseline; the legacy bundle model
and the v0.0.7 prompts/agents tables are gone (see
[CHANGELOG.md](CHANGELOG.md) for the migration path).

The v0.3.0 release ships the audit-chain TLA+ spec,
doc-freshness gating in CI, an mdBook documentation site, a
curated benchmark set plus a k6 p99 gate, and a clean release
pipeline (cosign keyless, GHCR, GitHub artifact attestations).
The runtime work (audit archival, bandit/settings CRDT,
release resolver, vault hot-reload, API server facade, the
sqliteimpl repository move, property tests, coverage / domain
/ lint gates, KMS rotate, LLM-judge scorer, net/rpc over UDS
plugin transport) ships in the binary. The native gRPC over
UDS plugin transport, the live pgx Postgres wiring, the
multi-region replication, and the CRDT idempotency / replay-set
caches ship as design docs and follow-on milestones (see
`ROADMAP.md` for the v0.4.0 / v0.5.0 schedule).

---

## Features

- **Content-Addressable Storage (CAS)** — Immutable, SHA-256-based object storage with Merkle DAG structure
- **Capability Versioning** — Every Capability has zero or more immutable Versions; the live Release per Environment points at exactly one Version
- **Manifest** — Content-addressed composition of Prompt, Model Policy, Runtime Policy, Context Contract, Memory, Guardrails, Tools, MCP servers, and Evaluation Suite
- **Recommendation Engine** — The deterministic rules engine plus the bandit Thompson Sampling selector close the loop
- **Approval Workflow** — `MajorityPolicy` and `MakerCheckerPolicy` with fail-closed separation of duties
- **Harness Engineering** — Preconditions gate Activate; eval runs score a Release against a Dataset. The fast iteration loop the OpenAI [harness engineering article](docs/reference/harness.md) prescribes.
- **LLM Provider Abstraction** — Unified interface for Anthropic and OpenAI via the official SDKs (`anthropics/anthropic-sdk-go`, `openai/openai-go/v3` Responses API)
- **Workflow DAG** — Topological execution with tool integration
- **Observability** — OpenTelemetry tracing, Prometheus metrics, audit logging
- **Built-in Guardrails** — PII redaction and prompt-injection detection ship as in-process plugins through the supervisor
- **Plugin SDK** — net/rpc over UDS subprocess transport (in-process supervisor at `backend/plugins/` + `backend/supervisor/`; v0.1.x production transport per `ADR-0024`); a future gRPC-over-UDS transport is on the roadmap (`ADR-0025`, not yet wired)
- **Webhooks** — Event-driven integrations with HMAC signing and SSRF protection
- **Secrets Management** — Encrypted vault for API keys and sensitive configuration
- **Rate Limiting** — Configurable per-client rate limiting with burst support
- **Per-Workspace Budgets and Quotas** — USD-cap and rate-cap enforcement via the `invoke` package
- **REST API** — Full-featured HTTP API with auto-generated OpenAPI specification (`backend/spec/spec.yaml`)

---

## Installation

### From source

```bash
git clone https://github.com/sachncs/promptsheon.git
cd promptsheon
make build                    # produces bin/promptsheond, bin/promptsheon, bin/promptsheon-healthcheck
```

### Run from a release binary

```bash
# Download the release binary for your platform from GitHub Releases.
# Then start the server.
./promptsheond
```

**Requirements**: Go 1.26+ (see `go.mod`).

---

## Quick Start

### CLI

```bash
# Clone and build
git clone https://github.com/sachncs/promptsheon.git
cd promptsheon
go build -o promptsheond ./bin/promptsheond
go build -o promptsheon  ./bin/promptsheon

# Start the server for this unauthenticated local walkthrough
PROMPTSHEON_AUTH=false ./promptsheond
```

### REST API (curl)

```bash
# Set up a Workspace, then a Project under it.
curl -X POST http://localhost:8080/api/v1/workspaces \
  -H 'Content-Type: application/json' \
  -d '{"name":"acme"}'

curl -X POST http://localhost:8080/api/v1/workspaces/w1/projects \
  -H 'Content-Type: application/json' \
  -d '{"name":"summariser"}'

# Create a Capability under the Project, then a Version with a Manifest.
curl -X POST http://localhost:8080/api/v1/projects/p1/capabilities \
  -H 'Content-Type: application/json' \
  -d '{"name":"summariser","description":"Summarise long docs"}'

curl -X POST http://localhost:8080/api/v1/capabilities/c1/versions \
  -H 'Content-Type: application/json' \
  -d '{"version":1, "manifest":{"prompt":{"kind":"prompt","hash":"<sha256>"}, "model_policy":{"kind":"model_policy","hash":"<sha256>"}, "runtime_policy":{"kind":"runtime_policy","hash":"<sha256>"}, "context_contract":{"kind":"context","hash":"<sha256>"}, "memory":{"kind":"memory","hash":"<sha256>"}}}'

# Drive the Release lifecycle end-to-end:
# 1. Create a Pending Release pointing the Version at the prod env.
REL=$(curl -sS -X POST http://localhost:8080/api/v1/versions/v1/releases \
        -H 'Content-Type: application/json' \
        -d '{"environment":"prod"}' | jq -r .id)
# 2. A distinct identity casts an Approve vote.
curl -sS -X POST http://localhost:8080/api/v1/releases/$REL/votes \
     -H 'Content-Type: application/json' \
     -d '{"identity":"bob","decision":"approve"}'
# 3. Activate (consults MakerChecker policy; 409 if quorum not satisfied
#    or if any precondition fails).
curl -sS -X POST http://localhost:8080/api/v1/releases/$REL/activate
# 4. Invoke through the configured LLM provider. The Release
#    decides provider + model; the request carries only the inputs.
curl -sS -X POST http://localhost:8080/api/v1/releases/$REL/invoke \
     -H 'Content-Type: application/json' \
     -d '{"inputs":{"q":"hello"}}'
```

### Go SDK

```go
import "github.com/sachncs/promptsheon/sdk"

client := sdk.New("http://localhost:8080", "ps_...")
ctx := context.Background()

rel, err := client.CreateRelease(ctx, "v1", sdk.CreateReleaseRequest{
    Environment: "prod",
})
if err != nil { return err }

if _, err := client.Vote(ctx, rel.ID, sdk.VoteRequest{
    Identity: "alice",
    Decision: "approve",
}); err != nil { return err }

if _, err := client.Activate(ctx, rel.ID); err != nil { return err }

out, err := client.Invoke(ctx, rel.ID, sdk.InvokeRequest{
    Inputs: map[string]any{"q": "hello"},
})
```

---

## Configuration

Promptsheon is configured via environment variables. Key settings:

| Variable | Default | Description |
|----------|---------|-------------|
| `PROMPTSHEON_ADDR` | `:8080` | Listen address |
| `PROMPTSHEON_DB_PATH` | `promptsheon.db` | SQLite database file. The daemon ships with SQLite; the Postgres backend (init + RLS bundles, in-memory fixture) is wired through `backend/store/postgres` and awaits the pgx follow-on. A shared backend (the prerequisite for multi-region replication) is tracked in [docs/multi-region.md](docs/operations/multi-region.md). |
| `PROMPTSHEON_AUTH` | `true` | Enable authentication. Set `false` only for local dev (and never on a non-loopback bind — the daemon refuses to start). |
| `PROMPTSHEON_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `PROMPTSHEON_APPROVAL_POLICY` | `maker_checker` | Approval policy: `maker_checker` (creator cannot approve their own release) or `majority`. See [docs/release.md](docs/reference/release.md). |
| `PROMPTSHEON_OPENAI_API_KEY` | (none) | OpenAI API key. Required to invoke OpenAI-backed Releases. |
| `PROMPTSHEON_OPENAI_BASE_URL` | (none) | OpenAI base URL override (for proxies). Defaults to `https://api.openai.com`. |
| `PROMPTSHEON_ANTHROPIC_API_KEY` | (none) | Anthropic API key. Required to invoke Anthropic-backed Releases. |
| `PROMPTSHEON_ANTHROPIC_BASE_URL` | (none) | Anthropic base URL override. Defaults to `https://api.anthropic.com`. |
| `PROMPTSHEON_PLUGINS_FILE` | (none) | Path to the plugin manifest. |
| `PROMPTSHEON_VAULT_KEY` | (none) | Master key for AES-256-GCM vault; override with KMS-backed `KeyProvider` for production. |
| `PROMPTSHEON_TLS_CERT_FILE` / `PROMPTSHEON_TLS_KEY_FILE` | (none) | TLS cert/key. Required for non-loopback binds. |
| `PROMPTSHEON_BOOTSTRAP_TOKEN` | (none) | Optional gate for `POST /api/v1/setup` when auth is enabled. |
| `PROMPTSHEON_LEADER_ELECTION` | `false` | Enable SQLite-backed leader election (`true` for multi-replica deployments). |
| `PROMPTSHEON_OTEL_SAMPLE_RATIO` | `1.0` | OTel trace sampling ratio (0.0–1.0). |
| `PROMPTSHEON_OTEL_ENDPOINT` | (none) | OTLP gRPC endpoint for trace export. |

See [docs/configuration.md](docs/operations/configuration.md) for the full reference.

---

## Harness engineering

Promptsheon's headline surface is the [harness engineering](docs/reference/harness.md) loop: Datasets (ground-truth `{inputs, expected}` pairs), Preconditions (named command hooks), and Evals (recorded scoring runs of a Release against a Dataset). Activate runs the Capability's preconditions; a failing hook returns 409 and leaves the Release in `pending`. Eval runs return 200 (passed) or 422 (failed) with per-case outcomes persisted.

```bash
# 1. Add a dataset + a precondition to your capability
promptsheon dataset create c1 --name greeting --file cases.json
promptsheon precondition add c1 --name go-test --cmd "go test ./..." --timeout 60

# 2. Drive the iteration loop
promptsheon release create <vid> prod
promptsheon release vote <rid> bob approve
promptsheon release activate <rid>      # 409 if preconditions fail
promptsheon eval run <rid> --dataset <dataset_id>
```

See [docs/eval.md](docs/reference/eval.md) for the eval primitive, [docs/harness.md](docs/reference/harness.md) for the surface rationale, and the [OpenAI article](https://openai.com/index/harness-engineering/) that inspired the design.

---

## API

| Symbol | Type | Description |
|--------|------|-------------|
| `Capability` | struct | A named logical capability with N immutable Versions |
| `Version` | struct | A specific immutable build of a Capability Manifest |
| `Release` | struct | A pointer to a Version inside a tenant Environment |
| `Manifest` | struct | Content-addressed composition of Prompt, ModelPolicy, RuntimePolicy, ContextContract, Memory, Guardrails, Tools, MCP, EvalSuite |
| `CAS` | type | Content-addressable store (Merkle DAG), lives at `backend/cas/` |
| `Vault` | type | AES-256-GCM vault (or KMS-backed `KeyProvider`) |
| `PluginSupervisor` | type | Supervisor for in-process plugins and remote (net/rpc over UDS) subprocess plugins |
| `Dataset` | struct | Named collection of `(inputs, expected)` test cases. The ground truth for harness eval. |
| `Precondition` | struct | Named command hook on a Capability; Activate runs every enabled precondition. |
| `EvalRun` | struct | Recorded scoring of a Release against a Dataset using a chosen Scorer. |
| `OpenAPI` | resource | Auto-generated OpenAPI spec at `backend/spec/spec.yaml` |

---

## Architecture

```
┌──────────────────────────────────────────────────┐
│                   REST API                        │
│         (autogenerated OpenAPI spec)              │
├──────────────────────────────────────────────────┤
│  Auth      │  Rate Limit  │  Audit Log  │  CORS   │
│  Middleware │  Middleware  │  Middleware │         │
├─────────────┴──────────────┴─────────────┴───────┤
│  Capability Mgr  │  Harness   │  Recommendation │  │
│  Manifests       │  Datasets, │  Engine       │  │
│  Releases        │  Precond,  │  (rules +     │  │
│  Approvals       │  Eval Runs │  bandit)      │  │
├──────────────────────────────────────────────────┤
│  Content-Addressable Store  │  SQLite runtime   │
│  (Merkle DAG)               │                   │
├──────────────────────────────────────────────────┤
│  LLM Providers  │  Observability  │  Webhooks     │
│  OpenAI/Anthro  │  OTel+Tracing   │  Event-Driven │
│  (official SDKs)│  Prometheus     │  HMAC-signed  │
├──────────────────────────────────────────────────┤
│  Plugin Supervisor  │  Vault  │  KeyProvider     │
│  (net/rpc over UDS)│  (KMS)  │  (BYOK)          │
└──────────────────────────────────────────────────┘
```

The server is composed of layered modules:

| Layer | Description |
|-------|-------------|
| **API** | HTTP handlers, middleware (auth, rate-limit, audit, CORS) |
| **Capabilities** | Manifests, Releases, Approvals, Datasets, Preconditions, Evals |
| **Harness** | The harness-engineering loop: datasets, preconditions, eval runs. See [docs/harness.md](docs/reference/harness.md). |
| **Storage** | CAS (Merkle DAG, `backend/cas/`) + SQLite. The Postgres backend is not implemented in this build. |
| **Providers** | Unified LLM provider abstraction layer (Anthropic + OpenAI) |
| **Observability** | OpenTelemetry tracing, metrics collection, retention |
| **Security** | AuthN/AuthZ, vault, guardrails, SSRF protection |
| **Plugins** | net/rpc over loopback (UDS); supervisor-managed lifecycle |

---

## Project Structure

```
promptsheon/
├── cmd/                     # One package main per binary
│   ├── promptsheond/        # Server daemon (daemon.go + adapters + embed + tests)
│   ├── promptsheon/         # CLI (cli.go + cli_cas + cli_harness + cli_http + cli_selfevolve)
│   └── promptsheon-healthcheck/  # Container probe (healthcheck.go)
├── backend/                 # Server-side implementation (all sub-packages live here)
│   ├── capability/          # Workspace / Project / Capability / Version / Release / Approval types
│   ├── harness/             # Dataset / Precondition / EvalRun types + runner
│   ├── eval/                # Scorer registry (exact_match, contains, regex, json_schema, llm_judge)
│   ├── release/             # Release aggregate + application service
│   ├── approval/            # MakerChecker + Majority policies
│   ├── vault/               # AES-256-GCM + KMS KeyProvider
│   ├── observability/       # OTel tracing and Prometheus metrics
│   ├── llm/                 # Anthropic + OpenAI provider implementations
│   ├── plugins/             # Plugin supervisor + transport
│   ├── guardrail/           # PII redaction, prompt-injection detection
│   ├── store/               # SQLite init + sqliteimpl repository layer
│   ├── cas/                 # Content-addressable store (Merkle DAG)
│   ├── handlers_*.go        # HTTP handlers (auth, capability, releases, harness, ...)
│   └── routes.go            # Mux registration; main entry point for the route table
├── backend/spec/spec.yaml   # OpenAPI 3.0 spec (source of truth; regenerated by `make openapi`)
├── sdk/                     # Go SDK for embedding Promptsheon
│   ├── python/              # Python client (codegen from backend/spec/spec.yaml)
│   └── typescript/          # TypeScript client (codegen from backend/spec/spec.yaml)
├── deploy/                  # Helm chart, Grafana dashboard, Prometheus alerts
├── docs/                    # Architecture, deployment, ADRs, troubleshooting, FAQ
│   ├── architecture/        # architecture.md, modules.md, design-decisions.md, glossary.md, algorithms.md, README.md (index)
│   ├── operations/          # deployment, configuration, operations, troubleshooting, upgrade, observability, slos, multi-region
│   ├── development/         # development, cli, testing, getting-started, faq
│   ├── reference/           # api-reference, sdk, harness, eval, llm-providers, guardrails, release, canary, workflows
│   └── security/            # security.md, audit-2026-07-26.md (frozen snapshot)
├── tests/                   # contract/ + e2e/ + smoke/ + chaos/ + load/
├── scripts/                 # genopenapi/, sync-version.sh, docs-check.sh, etc.
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile.goreleaser
├── .github/workflows/       # CI (ci.yaml), release pipeline
├── LICENSE                  # Apache 2.0
├── CONTRIBUTING.md
├── CODE_OF_CONDUCT.md
└── SECURITY.md
```

---

## Development

```bash
# Format, vet, lint, and test
make fmt
make vet
make lint
make test

# Build binaries
make build

# Regenerate the OpenAPI spec
make openapi

# Run the server on the default addr (`:8080`)
make run

# Run the lint-domain and lint-deps purity gates
make lint-domain
make lint-deps
```

---

## Testing

```bash
go test ./...
# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Contract test (every OpenAPI route is reachable + SDK surface in sync)
go test ./tests/contract/...

# End-to-end tests (in-process daemon, real HTTP)
go test ./tests/e2e/...

# Smoke test (boots a real daemon against an ephemeral DB)
bash tests/smoke/run.sh

# Chaos test (SQLite file-delete mid-query)
go test ./tests/chaos/...
```

---

## Build

```bash
make build                   # builds all three binaries into ./bin/
```

A GoReleaser pipeline (`.goreleaser.yml`) publishes multi-platform binaries

A GoReleaser pipeline (`.goreleaser.yml`) publishes multi-platform binaries
and a Docker image on tagged releases.

---

## Release

Tagged `vX.Y.Z` releases are produced by `.goreleaser.yml`. Each release:

- Builds binaries for Linux, macOS, and Windows on amd64 and arm64
  (`promptsheond` and `promptsheon`).
- Generates a Docker image (`ghcr.io/sachncs/promptsheon/promptsheond`)
  for `linux/amd64` and `linux/arm64`, tagged `v<version>` always
  and `latest` only on stable releases.
- Writes a single `checksums.txt` over every archive and signs it
  with `cosign sign-blob` (keyless; the bundle ships next to the
  checksum).
- Bundles the CycloneDX and SPDX SBOMs (produced upstream by the
  `sbom` CI job) as `extra_files` next to the archives.
- Creates a GitHub Release at `v<version>` (prerelease flag is
  inferred from the tag).

See [docs/release.md](docs/reference/release.md) for the full process.

---

## Tech Stack

| Category | Technology |
|----------|------------|
| Language | Go 1.26 |
| HTTP Routing | stdlib `net/http.ServeMux` (Go 1.22+ pattern matching) |
| CLI | Hand-rolled command dispatcher under `cmd/promptsheon/` |
| Storage | [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) (CGo-free SQLite). The Postgres backend is not implemented in this build. |
| LLM SDKs | [`anthropics/anthropic-sdk-go`](https://github.com/anthropics/anthropic-sdk-go), [`openai/openai-go/v3`](https://github.com/openai/openai-go) (Responses API) |
| RPC | `net/rpc` over UDS for plugin transport (`backend/plugins/` + `backend/supervisor/`; `ADR-0024`); a future gRPC-over-UDS transport is on the roadmap (`ADR-0025`, not yet wired) |
| Observability | [OpenTelemetry](https://opentelemetry.io/) (OTLP gRPC), Prometheus |
| Auth | OAuth 2.0 (`backend/auth/oauth.go`), static API keys (`backend/auth/auth.go`) |
| Vault | AES-256-GCM via [crypto/aes](https://pkg.go.dev/crypto/aes); KMS via pluggable `KeyProvider` |
| Lint/Format | [golangci-lint](https://golangci-lint.run/) (see `.golangci.yml`) |
| Releases | [GoReleaser](https://goreleaser.com/) (`.goreleaser.yml`) |
| Containerization | Docker (multi-stage) |

---

## Documentation

Full documentation lives in **[docs/](docs/)**:

- [Getting Started](docs/development/getting-started.md)
- [Configuration](docs/operations/configuration.md)
- [API Reference](docs/reference/api-reference.md) — [OpenAPI spec](backend/spec/spec.yaml)
- [Architecture](docs/architecture/architecture.md) — [Modules](docs/architecture/modules.md)
- [Harness engineering](docs/reference/harness.md) — why the eval/precondition/dataset surface exists
- [Eval primitive](docs/reference/eval.md) — datasets, preconditions, eval runs in detail
- [Release lifecycle](docs/reference/release.md) — Capability → Release with MakerChecker approval
- [SDKs](docs/reference/sdk.md) — Go / Python / TypeScript clients
- [LLM providers](docs/reference/llm-providers.md) — Anthropic + OpenAI
- [SLOs](docs/operations/slos.md) — three first-class SLOs with Prometheus alerts in `deploy/prometheus/`
- [Design Decisions](docs/architecture/design-decisions.md)
- [Security](docs/security/security.md)
- [Troubleshooting](docs/operations/troubleshooting.md) — [FAQ](docs/development/faq.md)

---

## Roadmap

- **v0.3.0** — Current release: forward-only Capability / Version /
  Release model, CAS + Merkle DAG (`backend/cas/`), MakerChecker
  approval, harness engineering (datasets / preconditions /
  evals), Anthropic + OpenAI via official SDKs, REST API,
  OTLP-only tracing, SQLite. Adds audit archival + chain-state
  tail cache, bandit + settings CRDT persistence, release
  resolver, vault hot-reload, API server facade, sqliteimpl
  repository move, property tests, coverage / domain / lint
  gates, KMS key rotation, the mdBook site, curated Go
  benchmark set + k6 p99 gate, the cosign-keyless +
  GitHub-attestation release pipeline, LLM-judge scorer,
  net/rpc over UDS plugin transport, configurable retention,
  Prometheus exporter.
- **v0.4.0** — Multi-region replication (the v0.3.0 release
  stays single-region by design; a shared backend lands
  first), live pgx Postgres wiring (v0.3.0 ships init +
  RLS bundles + an in-memory adapter only), the gRPC over
  UDS plugin transport swap-in (proto + stubs committed in
  v0.3.0), CRDT idempotency cache + replay-set CRDT (design
  docs in `docs/research/`), Canary Release primitive,
  webhook delivery retries + dead-letter queue, LLM-judge
  at scale (caching, batching, SLO observability), additional
  KMS integrations.
- **v1.0.0** — Stable API, gRPC streaming for real-time
  updates.

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and [docs/development.md](docs/development/development.md).

## Security

See [SECURITY.md](SECURITY.md). Report vulnerabilities via the GitHub Security Advisories workflow — do not open a public issue.

## Support

- **Issues**: [GitHub Issues](https://github.com/sachncs/promptsheon/issues)
- **Discussions**: [GitHub Discussions](https://github.com/sachncs/promptsheon/discussions)

## License

Apache License 2.0 — see [LICENSE](LICENSE) © 2026 Sachin