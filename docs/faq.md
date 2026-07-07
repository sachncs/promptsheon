# FAQ

Short answers to the questions that come up most often. If your question is not here, check [Troubleshooting](troubleshooting.md), the [Glossary](glossary.md), or open an issue.

## General

### What is Promptsheon?

A version-control system and runtime for AI agent configurations. It applies Git-style immutable, content-addressed storage to prompts, agents, evaluations, and tool specs, and exposes them through a REST API and a Go SDK.

### How is this different from LangChain / LlamaIndex / etc.?

Promptsheon is not a framework. It is a **server** for the assets those frameworks consume (prompts, agent definitions, evaluation datasets, audit logs). The idea is that you should be able to put your prompts and agent definitions under version control, run them through CI, evaluate them, and audit every change — the same way you do with application code.

### Is it production-ready?

Use the supported-versions table in `SECURITY.md` to see what is currently maintained. We aim to ship patch releases within a week of a confirmed vulnerability and feature releases on a roughly monthly cadence.

### What license is it under?

Apache 2.0. See `LICENSE`.

## Storage and version control

### Where is the database?

A single SQLite file at `PROMPTSHEON_DB_PATH` (default `promptsheon.db`). The CAS lives in `.promptsheon/objects/` for the CLI; the HTTP server does not currently expose the CAS directly.

### Why a separate CAS layer instead of just SQL?

Three reasons:

1. **Dedupe.** Identical content hashes to the same address and is stored once.
2. **Tamper evidence.** Any change to a blob breaks the chain of hashes.
3. **Offline operation.** The CLI can operate on a `.promptsheon` repository without the server.

See ADR [0001](adr/0001-use-cas-for-prompt-history.md).

### Can I import my existing prompts?

Yes. The `POST /api/v1/prompts` endpoint accepts a JSON body with the full prompt shape. A simple migration script that reads your existing prompts and POSTs them works for most cases.

### Can I export a prompt's full history?

Use `GET /api/v1/prompts/{id}/versions` to get the list of versions. The `cas_hash` field on each version is the SHA-256 of the canonical content. You can use `promptsheon show <hash>` to inspect it.

## LLM providers

### Which providers are supported?

OpenAI, Anthropic, Azure OpenAI, Ollama, NVIDIA NIM. See [LLM Providers](llm-providers.md) and `internal/llm/registry.go`.

### How is the provider selected for a prompt?

The `binding` block on the prompt:

```json
{
  "binding": {
    "provider": "openai",
    "model": "gpt-4o",
    "parameters": {"temperature": 0.7},
    "api_key_ref": "openai-production"
  }
}
```

The `api_key_ref` is a label for a key stored in the vault. If it is omitted, the provider's default credentials (from the environment) are used. See ADR [0009](adr/0009-prompt-binding-version-0-3-0.md).

### What happens if my primary provider is down?

The fallback chain is tried. See [Algorithms — Fallback chain](algorithms.md#fallback-chain).

### How is cost calculated?

Per-model, per-token pricing in `internal/llm/cost.go`. The cost is recorded on every call and exposed as a metric. Unknown models return a cost of `0` — we never invent a price.

## Security

### Where are my API keys stored?

In the vault, encrypted with AES-256-GCM. The encryption key is `PROMPTSHEON_VAULT_KEY`. The full key is never logged. See ADR [0004](adr/0004-aes-256-gcm-vault.md) and [Algorithms — Vault](algorithms.md#vault-aes-256-gcm).

### Is the audit log tamper-proof?

Tamper-evident, not tamper-proof. An attacker with write access to the database can rewrite the chain. The threat model assumes the database is inside the trust boundary. See ADR [0003](adr/0003-hash-chained-audit-log.md) and [Algorithms — Audit chain](algorithms.md#audit-chain).

### How do I report a vulnerability?

See [Security — Vulnerability reporting](security.md#vulnerability-reporting). Do **not** open a public issue.

## Operations

### How do I scale the server?

The server is stateless. You can run multiple instances behind a load balancer. Caveats:

- The audit chain is process-local. Concurrent writers to the same database use a SQLite `BEGIN IMMEDIATE` transaction, which serialises them.
- The rate limiter is per-process. With `N` instances, the effective per-key limit is `N * limit`.
- The CAS index (BM25) is in-process and is rebuilt on every start. For very large prompt corpora, consider sharding.

### How do I back up the database?

```bash
# Hot backup (safe while the server is running)
sqlite3 promptsheon.db ".backup backup.db"
```

Or stop the server and copy the file. The audit chain is hash-linked; it survives a backup/restore cycle as long as the file is copied byte-for-byte.

### How do I upgrade?

1. Read the `CHANGELOG.md` entry for the new version. Pay attention to the `BREAKING` markers.
2. Stop the server.
3. Replace the binary.
4. Start the server. Migrations apply automatically.
5. Verify with `GET /api/v1/audit/verify`.

### Can I run it on Windows?

Yes. The build is `CGO_ENABLED=0`, so the binary runs on any platform that Go supports. The CAS on disk uses forward slashes; the server uses `filepath.Join` for any path manipulation, so it works on Windows file paths.

## SDK and CLI

### Where is the SDK?

`github.com/sachncs/promptsheon/sdk`. See [SDK](sdk.md).

### Where is the CLI?

`cmd/promptsheon`, built as the `promptsheon` binary. See [CLI](cli.md).

### Can I script the CLI?

Yes. Output is plain text on stdout, errors on stderr, exit code `0` on success. `promptsheon log` and `promptsheon graph` are designed to be piped.

## Documentation

### Where is the canonical spec?

[`api/openapi.yaml`](../api/openapi.yaml). Generated by `make openapi` from the server's route table.

### How do I add a doc?

Add a Markdown file to `docs/`. Add a row to the index in `docs/README.md`. Follow the conventions in the "Authoring guide" section of `docs/README.md`.

### How do I write an ADR?

Copy `docs/adr/template.md` to `docs/adr/NNNN-short-slug.md`. Fill every section. Add a row to the index in `docs/adr/README.md`. See [Design Decisions](design-decisions.md) for the criteria.
