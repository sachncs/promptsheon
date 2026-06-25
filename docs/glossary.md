# Glossary

Canonical terms used throughout the Promptsheon documentation. Every doc in `docs/` must use the terms defined here verbatim — no synonyms.

## Core concepts

**Prompt**
A versioned template with variables that can be rendered, run against an LLM, evaluated, and deployed. The fundamental unit of authoring in Promptsheon.

**Agent**
A named DAG of steps. Each step references a prompt and declares which other steps it depends on. Agents are the unit of orchestration.

**Step**
A single unit of work in an agent workflow. May execute a prompt, invoke a tool, or both. Has an `id`, an optional `depends_on` list, and an `output_key` for the result.

**Tool**
A registered, named function that a step can invoke. Built-in types: `http`, `shell`, `json_transform`, `prompt_call`. See [Workflows](workflows.md).

**Context**
A persisted multi-message history that an agent can use across runs. Composed, trimmed, and token-budgeted by the [context manager](../internal/context/manager.go).

**Snapshot**
A captured, immutable record of an agent or prompt execution output. Used for replay, comparison, and regression detection.

## Version control and storage

**CAS (Content-Addressable Storage)**
The storage model used for prompt history. Every object is identified by its SHA-256 hash of its content, deduplicated, and write-once. See [Architecture](architecture.md).

**Merkle DAG**
A directed acyclic graph in which every node is addressed by the hash of its content. The four object types (`blob`, `tree`, `commit`, `ref`) form a Merkle DAG.

**Object**
A `blob`, `tree`, `commit`, or `ref` in the CAS. All objects are content-addressed, immutable (except `ref`), and gzip-compressed on disk.

**Blob**
Raw content: a prompt text, a tool spec, a YAML config. Addressed by its hash.

**Tree**
A named mapping of blobs (and other trees) that forms a configuration snapshot.

**Commit**
An immutable node that references a tree, zero or more parent commits, and metadata (author, message, telemetry). Forms the nodes of the version history.

**Ref**
A mutable pointer (branch) to a commit. Refs are the only mutable objects in the system.

**Branch**
A named `ref` (lives under `refs/heads/`). Branches are how you develop new versions in parallel.

**Tag**
A named `ref` that points to a fixed commit and is not moved by commits. Used for releases and rollbacks.

**Hash**
A 64-character lowercase hex SHA-256 string that uniquely identifies an object.

## Resilience and reliability

**Retry**
The act of re-issuing a transiently-failed LLM request. Uses exponential backoff and a typed-error classifier. See [Algorithms — Retry](algorithms.md#retry).

**Circuit breaker**
A state machine (`closed` → `open` → `half-open` → `closed`) that prevents calls to a failing provider. Opens after a threshold of failures and probes periodically. See [Algorithms — Circuit breaker](algorithms.md#circuit-breaker).

**Fallback chain**
A primary provider plus an ordered list of alternatives. If the primary fails, the dispatcher tries each in order. See [Algorithms — Fallback chain](algorithms.md#fallback-chain).

**Typed error**
A Go error type that signals its semantics to the retry classifier: `*llm.ErrTransient` (worth retrying) and `*llm.ErrPermanent` (do not retry). See `internal/llm/retry.go`.

## Security and observability

**Audit chain**
A hash-linked sequence of `audit_entries` rows. Each entry stores `previous_hash` and `entry_hash` over the canonical representation of itself and its predecessor. Verifiable with `GET /api/v1/audit/verify`. See [Algorithms — Audit chain](algorithms.md#audit-chain).

**Vault**
AES-256-GCM encryption used to store LLM provider API keys at rest. The 32-byte key is read from `PROMPTSHEON_VAULT_KEY`. The all-zero key is rejected. See [Algorithms — Vault](algorithms.md#vault).

**HMAC webhook**
A webhook delivery whose body is signed with `HMAC-SHA256(secret, body)`. Receivers verify the signature in the `X-Promptsheon-Signature` header. See [Security](security.md).

**SSRF (Server-Side Request Forgery)**
An attack in which a server makes an attacker-controlled request to an internal address. Promptsheon's webhook dispatcher refuses loopback and RFC1918 destinations by default; `PROMPTSHEON_WEBHOOK_ALLOW_PRIVATE=true` opts in for local development. See [Security](security.md).

**Slowloris**
A denial-of-service attack that opens many connections and trickles bytes slowly. Defended by `ReadHeaderTimeout` (default 10s). See [Security](security.md).

**OTel (OpenTelemetry)**
The cross-vendor observability standard. If `PROMPTSHEON_OTEL_ENDPOINT` is set, Promptsheon exports traces via OTLP gRPC. See [Observability](observability.md).

**Slog**
Go's `log/slog` structured logging package. The single logger used server-wide. JSON output to stderr.

**Retention**
A periodic sweep that deletes trace spans, audit entries, and snapshots older than their configured TTL. Configurable via env vars. See [Observability](observability.md).

## Evaluation and search

**Test case**
A single (input, expected) pair in a test dataset. Inputs are substituted into the prompt template; expectations are matched against the LLM output.

**Scorer**
A registered function that scores an LLM output for a test case on a 0.0–1.0 scale. Built-in: exact-match, contains, hallucination. See [Evaluations](evaluations.md).

**Hallucination score**
A 0.0–1.0 measure of how much of the LLM output is unsupported by the prompt and inputs. Higher means more hallucinated. See [Evaluations](evaluations.md).

**BM25**
The ranking function used by `internal/search`. BM25 is the standard Okapi BM25 with `k1=1.2`, `b=0.75`, unigrams and bigrams, light suffix stripping, and per-document length normalisation. See [Algorithms — BM25](algorithms.md#bm25).

## Project and process

**ADR (Architecture Decision Record)**
A short, immutable document that captures one significant design decision: context, options, choice, and consequences. ADRs live under `docs/adr/`.

**Server (promptsheond)**
The HTTP daemon binary. Built from `cmd/promptsheond`.

**CLI (promptsheon)**
The client binary that operates on a `.promptsheon` repository. Built from `cmd/promptsheon`.

**SDK**
The Go client library at `github.com/sachn-cs/promptsheon/sdk`. Wraps the HTTP API.

**Provider**
An implementation of the `llm.Provider` interface for a single LLM backend (OpenAI, Anthropic, Azure OpenAI, Ollama, NVIDIA). See [LLM Providers](llm-providers.md).

**Handler**
A function in `internal/api/handlers_*.go` that handles a single route. The OpenAPI generator parses the handler AST to extract request schemas.

**Route**
A `(method, path)` pair registered on the HTTP mux in `internal/api/server.go`. The OpenAPI generator collects these.

## Compliance and policy

**Shell tool policy**
The combination of `PROMPTSHEON_SHELL_ENABLED` and `PROMPTSHEON_SHELL_ALLOWLIST`. When the allowlist is empty the shell tool is disabled regardless of the enabled flag. See [Security](security.md).

**Severity level**
The classification of a guardrail violation: `low` (info), `medium` (warn), `high` (block), `critical` (immediate block + security event). See [Guardrails](guardrails.md).
