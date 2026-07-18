<p align="center">
  <h1 align="center">Promptsheon</h1>
  <p align="center">The Control Plane for AI Capabilities — v0.1.0</p>
  <p align="center">
    <a href="#installation"><img src="https://img.shields.io/badge/go-1.26-00ADD8?logo=go" alt="Go"></a>
    <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache%202.0-blue" alt="License"></a>
    <a href="https://github.com/sachncs/promptsheon/actions"><img src="https://img.shields.io/github/actions/workflow/status/sachncs/promptsheon/ci.yaml?branch=master" alt="CI"></a>
    <a href="https://github.com/sachncs/promptsheon/stargazers"><img src="https://img.shields.io/github/stars/sachncs/promptsheon" alt="Stars"></a>
  </p>
</p>

Promptsheon is the control plane for AI Capabilities. Every
Capability — its Prompt, Model Policy, Runtime Policy, Context
Contract, Memory, Guardrails, Tools, MCP servers, and
Evaluation Suite — is an immutable, content-addressed Manifest
recorded as a Directed Acyclic Graph. Production tenants manage
their Capabilities the way engineers manage code: with versions,
reviews, releases, canary deployments, and rollbacks. v0.1.0 is
the forward-only baseline; the legacy bundle model and the
v0.0.7 prompts/agents tables are gone (see
[CHANGELOG.md](CHANGELOG.md) for the migration path).

---

## Features

- **Content-Addressable Storage (CAS)** — Immutable, SHA-256-based object storage with Merkle DAG structure
- **Capability Versioning** — Every Capability has zero or more immutable Versions; the live Release per Environment points at exactly one Version
- **Manifest** — Content-addressed composition of Prompt, Model Policy, Runtime Policy, Context Contract, Memory, Guardrails, Tools, MCP servers, and Evaluation Suite
- **Recommendation Engine** — The deterministic rules engine (Tier 2.35) plus the bandit Thompson Sampling selector (Tier 2.36) close the loop
- **Approval Workflow** — `MajorityPolicy` and `MakerCheckerPolicy` with separation of duties
- **Harness Engineering** — Preconditions gate Activate; eval runs score a Release against a Dataset. The fast iteration loop the OpenAI [harness engineering article](docs/harness.md) prescribes.
- **LLM Provider Abstraction** — Unified interface for Anthropic and OpenAI via the official SDKs (`anthropics/anthropic-sdk-go`, `openai/openai-go/v3` Responses API)
- **Workflow DAG** — Topological execution with tool integration
- **Observability** — OpenTelemetry tracing, Prometheus metrics, audit logging
- **Built-in Guardrails** — PII redaction (Tier 2.47) and prompt-injection detection (Tier 2.48) ship as in-process plugins through the supervisor (Tier 2.46)
- **Plugin SDK** — gRPC over loopback for remote plugins (Tier 2.32 manifest); KMS-backed KeyProvider (Tier 2.45) for BYOK
- **Webhooks** — Event-driven integrations with HMAC signing and SSRF protection
- **Secrets Management** — Encrypted vault for API keys and sensitive configuration
- **Rate Limiting** — Configurable per-client rate limiting with burst support
- **Per-Workspace Budgets and Quotas** — USD-cap enforcement (Tier 2.37) and rate-cap enforcement
- **REST API** — Full-featured HTTP API with auto-generated OpenAPI specification

---

## Installation

### From source

```bash
git clone https://github.com/sachncs/promptsheon.git
cd promptsheon
go build -o promptsheond ./cmd/promptsheond
go build -o promptsheon  ./cmd/promptsheon
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
go build -o promptsheond ./cmd/promptsheond
go build -o promptsheon  ./cmd/promptsheon

# Start the server
./promptsheond
```

### REST API (curl)

```bash
# Create a Capability
curl -X POST http://localhost:8080/api/v1/projects/p1/capabilities \
  -H "Content-Type: application/json" \
  -d '{"name":"greeting","description":"Friendly greeting"}'

# Add an immutable Version with a Manifest of content-addressed artifacts
curl -X POST http://localhost:8080/api/v1/capabilities/c1/versions \
  -H "Content-Type: application/json" \
  -d '{"version":1, "manifest":{"prompt":{"kind":"prompt","hash":"<sha256>"}, ...}}'

# Drive the Release lifecycle end-to-end:
# 1. Create a Pending Release pointing the Version at the prod env.
REL=$(curl -sS -X POST http://localhost:8080/api/v1/versions/v1/releases \
        -H "Content-Type: application/json" \
        -d '{"environment":"prod"}' | jq -r .id)
# 2. A non-creator identity casts an Approve vote.
curl -sS -X POST http://localhost:8080/api/v1/releases/$REL/votes \
     -H "Content-Type: application/json" \
     -d '{"identity":"alice","decision":"approve"}'
# 3. Activate (consults MakerChecker policy; 409 if quorum not satisfied).
curl -sS -X POST http://localhost:8080/api/v1/releases/$REL/activate
# 4. Invoke through the configured LLM provider.
curl -sS -X POST http://localhost:8080/api/v1/releases/$REL/invoke \
     -H "Content-Type: application/json" \
     -d '{"inputs":{"q":"hello"},"model":"claude-opus-4"}'
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
    Model:  "claude-opus-4",
})
```

---

## Configuration

Promptsheon is configured via environment variables or a config file. Key settings:

| Variable | Default | Description |
|----------|---------|-------------|
| `PROMPTSHEON_ADDR` | `:8080` | Listen address |
| `PROMPTSHEON_DB_PATH` | `promptsheon.db` | SQLite database file. v0.1.x is SQLite-only; the Postgres backend was removed. |
| `PROMPTSHEON_LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `PROMPTSHEON_AUTH` | `false` | Enable authentication |
| `PROMPTSHEON_APPROVAL_POLICY` | `maker_checker` | Approval policy: `maker_checker` (creator cannot approve their own release; at least one other identity must) or `majority` (flat count-based). See [docs/release.md](docs/release.md). |
| `PROMPTSHEON_OPENAI_API_KEY` | (none) | OpenAI API key. Required to invoke OpenAI-backed Releases. |
| `PROMPTSHEON_OPENAI_BASE_URL` | (none) | OpenAI base URL override (for proxies). Defaults to `https://api.openai.com`. |
| `PROMPTSHEON_ANTHROPIC_API_KEY` | (none) | Anthropic API key. Required to invoke Anthropic-backed Releases. |
| `PROMPTSHEON_ANTHROPIC_BASE_URL` | (none) | Anthropic base URL override. Defaults to `https://api.anthropic.com`. |
| `PROMPTSHEON_PLUGINS_FILE` | (none) | Path to the plugin manifest. |
| `PROMPTSHEON_VAULT_KEY` | (none) | Master key for AES-256-GCM vault; override with KMS-backed KeyProvider for production. |

See [docs/configuration.md](docs/configuration.md) for the full reference.

---

## Harness engineering

Promptsheon's headline surface is the [harness engineering](docs/harness.md) loop: Datasets (ground-truth `{inputs, expected}` pairs), Preconditions (named command hooks), and Evals (recorded scoring runs of a Release against a Dataset). Activate runs the Capability's preconditions; a failing hook returns 409 and leaves the Release in `pending`. Eval runs return 200 (passed) or 422 (failed) with per-case outcomes persisted.

```bash
# 1. Add a dataset + a precondition to your capability
promptsheon dataset create c1 --name greeting --file cases.json
promptsheon precondition add c1 --name go-test --cmd "go test ./..." --timeout 60

# 2. Drive the iteration loop
promptsheon release create <vid> '{"environment":"prod"}'
promptsheon release vote <rid> bob approve
promptsheon release activate <rid>      # 409 if preconditions fail
promptsheon eval run <rid> --dataset <dataset_id>
```

See [docs/eval.md](docs/eval.md) for the eval primitive, [docs/harness.md](docs/harness.md) for the surface rationale, and the [OpenAI article](https://openai.com/index/harness-engineering/) that inspired the design.

---

## API

| Symbol | Type | Description |
|--------|------|-------------|
| `Capability` | struct | A named logical capability with N immutable Versions |
| `Version` | struct | A specific immutable build of a Capability Manifest |
| `Release` | struct | A pointer to a Version inside a tenant Environment |
| `Manifest` | struct | Content-addressed composition of Prompt, ModelPolicy, RuntimePolicy, ContextContract, Memory, Guardrails, Tools, MCP, EvalSuite |
| `CAS` | type | Content-addressable store (Merkle DAG) |
| `Vault` | type | AES-256-GCM vault (or KMS-backed `KeyProvider`) |
| `PluginSupervisor` | type | gRPC-over-UDS supervisor for in-process and remote plugins |
| `Dataset` | struct | Named collection of `(inputs, expected)` test cases. The ground truth for harness eval. |
| `Precondition` | struct | Named command hook on a Capability; Activate runs every enabled precondition. |
| `EvalRun` | struct | Recorded scoring of a Release against a Dataset using a chosen Scorer. |
| `OpenAPI` | resource | Auto-generated OpenAPI spec at `api/openapi.yaml` |

---

## Examples

```bash
# 1. End-to-end capability lifecycle against a local server.
./promptsheond &  # in another shell

curl -X POST http://localhost:8080/api/v1/workspaces \
  -H 'Content-Type: application/json' \
  -d '{"name":"acme"}'

curl -X POST http://localhost:8080/api/v1/workspaces/w1/projects \
  -H 'Content-Type: application/json' \
  -d '{"name":"summariser"}'

curl -X POST http://localhost:8080/api/v1/projects/p1/capabilities \
  -H 'Content-Type: application/json' \
  -d '{"name":"summariser","description":"Summarise long docs"}'

# Add a Version with a content-addressed Manifest. Use the SampleManifestHash
# (or your own sha256) for each leaf artifact. The hash becomes the
# deduplication key in the CAS store.
curl -X POST http://localhost:8080/api/v1/capabilities/c1/versions \
  -H 'Content-Type: application/json' \
  -d '{"version":1,"manifest":{"prompt":{"kind":"prompt","hash":"<sha256>"}, ...}}'

# Drive the Release lifecycle end-to-end:
REL=$(curl -sS -X POST http://localhost:8080/api/v1/versions/v1/releases \
       -H 'Content-Type: application/json' \
       -d '{"environment":"prod"}' | jq -r .id)
curl -sS -X POST http://localhost:8080/api/v1/releases/$REL/votes \
     -H 'Content-Type: application/json' \
     -d '{"identity":"alice","decision":"approve"}'
curl -sS -X POST http://localhost:8080/api/v1/releases/$REL/activate
curl -sS -X POST http://localhost:8080/api/v1/releases/$REL/invoke \
     -H 'Content-Type: application/json' \
     -d '{"inputs":{"text":"long document..."},"model":"claude-opus-4"}'
```

```go
// 2. Programmatic Go SDK lifecycle
client := sdk.New("http://localhost:8080", "ps_...")
ctx := context.Background()

cap, _ := client.CreateCapability(ctx, "p1", sdk.CreateCapabilityRequest{
    Name: "summariser",
})
ver, _ := client.AddVersion(ctx, cap.ID, sdk.AddVersionRequest{
    Version: 1,
    Manifest: sdk.Manifest{Prompt: "<sha256-of-prompt-blob>"},
})
rel, _ := client.CreateRelease(ctx, ver.ID, sdk.CreateReleaseRequest{Environment: "prod"})
client.Vote(ctx, rel.ID, sdk.VoteRequest{Identity: "alice", Decision: "approve"})
activated, _ := client.Activate(ctx, rel.ID)
out, _ := client.Invoke(ctx, activated.ID, sdk.InvokeRequest{
    Inputs: map[string]any{"text": "long document..."},
    Model:  "claude-opus-4",
})
fmt.Println(out.Outputs["content"])
```

The [`examples/`](examples/) directory has more end-to-end recipes,
including a harness-engineering eval example. See
[docs/harness.md](docs/harness.md) for the full surface and
[docs/eval.md](docs/eval.md) for the eval primitive.

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
│  Content-Addressable Store  │  SQLite (only)    │
│  (Merkle DAG)               │                   │
├──────────────────────────────────────────────────┤
│  LLM Providers  │  Observability  │  Webhooks     │
│  OpenAI/Anthro  │  OTel+Tracing   │  Event-Driven │
│  (official SDKs)│  Prometheus     │  HMAC-signed  │
├──────────────────────────────────────────────────┤
│  Plugin Supervisor  │  Vault  │  KeyProvider     │
│  (gRPC over UDS)   │  (KMS)  │  (BYOK)          │
└──────────────────────────────────────────────────┘
```

The server is composed of layered modules:

| Layer | Description |
|-------|-------------|
| **API** | HTTP handlers, middleware (auth, rate-limit, audit, CORS) |
| **Capabilities** | Manifests, Releases, Approvals, Datasets, Preconditions, Evals |
| **Harness** | The harness-engineering loop: datasets, preconditions, eval runs. See [docs/harness.md](docs/harness.md). |
| **Storage** | CAS (Merkle DAG) + SQLite (v0.1.x is SQLite-only) |
| **Providers** | Unified LLM provider abstraction layer (Anthropic + OpenAI) |
| **Observability** | OpenTelemetry tracing, metrics collection, retention |
| **Security** | AuthN/AuthZ, vault, guardrails, SSRF protection |
| **Plugins** | gRPC over loopback; supervisor-managed lifecycle |

---

## Project Structure

```
promptsheon/
├── cmd/
│   ├── promptsheond/   # Server binary
│   └── promptsheon/    # CLI binary (cas + http + harness + per-command files)
├── api/                # OpenAPI spec and codegen
├── internal/           # Server-side implementation modules
│   ├── capabilities/
│   ├── harness/        # Dataset / Precondition / EvalRun types + runner
│   ├── eval/           # Scorer registry (exact_match, contains, regex)
│   ├── release/        # Release aggregate + application service
│   ├── approval/      # MakerChecker + Majority policies
│   ├── cas/            # Content-addressable store (Merkle DAG)
│   ├── vault/          # AES-256-GCM + KMS KeyProvider
│   ├── observability/  # OTel tracing and Prometheus metrics
│   ├── llm/            # Anthropic + OpenAI provider implementations
│   ├── plugins/        # Plugin supervisor + RPC bridge
│   ├── guardrails/     # PII redaction, prompt-injection detection
│   └── ...
├── pkg/                # Stable public packages consumable by other Go projects
├── sdk/                # Go SDK for embedding Promptsheon (also python/, typescript/)
├── deploy/             # systemd, docker, kubernetes manifests
├── docs/               # Architecture, deployment, ADRs, troubleshooting, FAQ
├── examples/           # End-to-end recipes
├── tests/              # Test suite
├── scripts/            # Local-dev helpers
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile
├── .github/workflows/  # CI (ci.yaml), release pipeline
├── LICENSE             # Apache 2.0
├── CONTRIBUTING.md
├── CODE_OF_CONDUCT.md
└── SECURITY.md
```

---

## Development

```bash
# Run all checks (format, vet, lint, test)
make check

# Build binaries
make build

# Run unit + integration tests
make test

# Regenerate the OpenAPI spec
make openapi

# Run the server on the default addr (`:8080`)
make run

# Format, vet, lint individually
gofmt -w .
go vet ./...
golangci-lint run
```

---

## Testing

```bash
go test ./...
# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

---

## Build

```bash
go build -o promptsheond ./cmd/promptsheond
go build -o promptsheon  ./cmd/promptsheon
```

A GoReleaser pipeline (`.goreleaser.yml`) publishes multi-platform binaries
and a Docker image on tagged releases.

---

## Release

Tagged `vX.Y.X` releases are produced by `.goreleaser.yml`. Each release:

- Builds binaries for Linux, macOS, and Windows on amd64 and arm64.
- Generates a Docker image published to the configured registry.
- Produces a `promptsheon_${VERSION}_checksums.txt` SBOM and a `.deb`/`.rpm`
  pair (when enabled).
- Tags the Git repository.

See [docs/release.md](docs/release.md) for the full process.

---

## Tech Stack

| Category | Technology |
|----------|------------|
| Language | Go 1.26 |
| HTTP Routing | stdlib `net/http.ServeMux` (Go 1.22+ pattern matching) |
| CLI | Hand-rolled command dispatcher under `cmd/promptsheon/main.go` |
| Storage | [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) (CGo-free SQLite). v0.1.x is SQLite-only; the Postgres backend was removed. |
| LLM SDKs | [`anthropics/anthropic-sdk-go`](https://github.com/anthropics/anthropic-sdk-go), [`openai/openai-go/v3`](https://github.com/openai/openai-go) (Responses API) |
| RPC | [google.golang.org/grpc](https://grpc.io/docs/languages/go/) (plugin transport; not wired into the request path in v0.1.x) |
| Observability | [OpenTelemetry](https://opentelemetry.io/), Prometheus |
| Auth | OIDC, static API keys |
| Vault | AES-256-GCM via [crypto/aes](https://pkg.go.dev/crypto/aes); KMS via pluggable `KeyProvider` |
| Lint/Format | [golangci-lint](https://golangci-lint.run/) (see `.golangci.yml`) |
| Releases | [GoReleaser](https://goreleaser.com/) (`.goreleaser.yml`) |
| Containerization | Docker (multi-stage) |

---

## Documentation

Full documentation lives in **[docs/](docs/)**:

- [Getting Started](docs/getting-started.md)
- [Configuration](docs/configuration.md)
- [API Reference](docs/api-reference.md) — [OpenAPI spec](api/openapi.yaml)
- [Architecture](docs/architecture.md) — [Modules](docs/modules.md)
- [Harness engineering](docs/harness.md) — why the eval/precondition/dataset surface exists
- [Eval primitive](docs/eval.md) — datasets, preconditions, eval runs in detail
- [Release lifecycle](docs/release.md) — Capability → Release with MakerChecker approval
- [SDKs](docs/sdk.md) — Go / Python / TypeScript clients
- [LLM providers](docs/llm-providers.md) — Anthropic + OpenAI
- [Design Decisions](docs/design-decisions.md) and [ADRs](docs/adr/)
- [Security](docs/security.md)
- [Troubleshooting](docs/troubleshooting.md) — [FAQ](docs/faq.md)

---

## Roadmap

- **v0.1.x** — Current: forward-only Capability / Version / Release
  model, CAS + Merkle DAG, MakerChecker approval, harness
  engineering (datasets / preconditions / evals), Anthropic + OpenAI
  via official SDKs, REST API. SQLite-only.
- **v0.2.0** — Multi-region replication, configurable retention,
  Prometheus exporter, json_schema scorer for evals
- **v0.3.0** — Webhook delivery retries + dead-letter queue,
  LLM-judge scorer for evals
- **v1.0.0** — Stable API, gRPC streaming for real-time updates,
  additional KMS integrations, Postgres parity (currently deleted)

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and [docs/development.md](docs/development.md).

## Security

See [SECURITY.md](SECURITY.md). Report vulnerabilities via the GitHub Security Advisories workflow — do not open a public issue.

## Support

- **Issues**: [GitHub Issues](https://github.com/sachncs/promptsheon/issues)
- **Discussions**: [GitHub Discussions](https://github.com/sachncs/promptsheon/discussions)

## License

Apache License 2.0 — see [LICENSE](LICENSE) © 2026 Sachin
