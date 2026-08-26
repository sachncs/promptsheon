<p align="center">
  <h1 align="center">promptsheon</h1>
  <p align="center">A self-hosted prompt-management platform — DAG editor, canary releases, maker-checker approvals, eval suites, and self-evolution — all powered by Strands Agents.</p>
  <p align="center">
    <a href="#installation"><img src="https://img.shields.io/badge/node-%E2%89%A526-green" alt="Node.js"></a>
    <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache_2.0-blue" alt="License"></a>
    <a href="https://github.com/sachncs/promptsheon/releases/latest"><img src="https://img.shields.io/github/v/release/sachncs/promptsheon" alt="Latest release"></a>
    <a href="https://github.com/sachncs/promptsheon/actions"><img src="https://img.shields.io/github/actions/workflow/status/sachncs/promptsheon/ci.yaml?branch=master" alt="CI"></a>
    <a href="https://github.com/sachncs/promptsheon/stargazers"><img src="https://img.shields.io/github/stars/sachncs/promptsheon" alt="Stars"></a>
    <a href="https://github.com/sachncs/promptsheon/blob/master/AGENTS.md"><img src="https://img.shields.io/badge/code%20style-AGENTS-constitution-orange" alt="AGENTS constitution"></a>
  </p>
</p>

---

## What is this?

Promptsheon is a single self-hosted platform that answers one
question:

> *"How do I author a multi-agent prompt, ship it safely to
> production, prove it doesn't regress, and roll it back if it
> does?"*

It is a Fastify backend on `:8080` plus a Next.js web UI on `:3000`,
talking to a local SQLite file and a content-addressed store. No
cloud account, no signup, no telemetry — `pnpm install && pnpm dev`
and you have a fully working prompt-management platform with
multi-provider LLM, an audit chain, webhooks, eval scorers, and a
live DAG editor.

Built on the
[`@strands-agents/sdk`](https://github.com/strands-agents/sdk) for
every AI call: planning is a 5-agent `Swarm`, execution is a `Graph`
of per-node `Agent`s, and standalone `Agent`s handle compilation,
scoring, and self-evolution.

---

## Who is this for?

You, even if:

- You've never written TypeScript before.
- You don't know what a "multi-agent DAG" is.
- You've never shipped a prompt to production.

If you can install Node.js and run `pnpm dev`, you can use
Promptsheon. When the docs use a word you don't know, look it up
in the [Glossary](https://github.com/sachncs/promptsheon#what-can-it-do)
or in [AGENTS.md](AGENTS.md).

If you've used Fastify or Next.js before, you'll be productive in
ten minutes.

---

## What can it do?

- **DAG-based capability editor** — drag-and-drop nodes for
  Planner, Agent, Tool, and Guardrail; per-node config; live
  execution preview.
- **Releases with canary rollout** — versioned releases,
  per-environment activation, weighted traffic split, and
  one-click rollback / supersede.
- **Maker-checker approvals** — release creator cannot approve their
  own release; approvals are persisted with reason and voter.
- **Strands-powered planning** — a `Swarm` of 5 specialised agents
  decomposes a goal into a capability DAG.
- **Strands-powered execution** — each capability node becomes a
  `Graph` node with its own `Agent`, shared scratchpad, and
  observability hooks.
- **Multi-provider LLM** — OpenAI, Anthropic, AWS Bedrock, or a
  custom OpenAI/Anthropic-compatible endpoint. Strands handles
  retries, timeouts, and structured-output validation.
- **Eval suites + scorers** — declarative dataset cases, pluggable
  scorers (LLM-judge, regex, exact-match), and parallel run results.
- **Self-evolution loop** — monitors live eval scores of an active
  release; on regression it triggers a re-plan and re-release with
  cooldown.
- **Audit chain** — append-only, hash-linked audit log with a
  cryptographic verification endpoint.
- **Content-addressed store (CAS)** — every compiled manifest is
  hashed and stored by content, never by name.
- **Maker-checker webhooks** — incoming webhooks with HMAC
  verification and replay protection.

---

## Before you start

You'll need **Node.js 26 or newer** and **pnpm 11** installed on
your computer.

If you don't know what Node.js is or whether you have it:

1. Open a terminal (on macOS: `Cmd + Space`, type "Terminal"; on
   Windows: open "PowerShell"; on Linux: open your usual terminal).
2. Type `node --version` and press Enter.
3. If you see a version number starting with `26`, you're set.
4. If you see "command not found" or an older version, follow the
   [official Node.js installer guide](https://nodejs.org/en/download/package-manager).

You'll also need at least one LLM API key — OpenAI, Anthropic, or
your own OpenAI-compatible endpoint. Promptsheon supports
custom-URL providers so a private gateway works out of the box.

---

## Installation

Pick whichever option fits your setup:

### Option 1 — From source (recommended for development)

```bash
# 1. Download the code
git clone https://github.com/sachncs/promptsheon.git
cd promptsheon

# 2. Install dependencies for every workspace
pnpm install

# 3. Copy the env template and edit it
cp .env.example .env
$EDITOR .env   # fill in OPENAI_API_KEY (or ANTHROPIC_API_KEY)

# 4. Start the backend + the frontend together
pnpm dev
```

By default the backend listens on `http://localhost:8080` and the
frontend on `http://localhost:3000`. No external services beyond
the LLM provider you choose.

> 💡 **The frontend's `next.config.ts` rewrites `/api/*` →
> `http://localhost:8080/api/*` automatically.** You only need both
> servers running.

### Option 2 — From source, hot reload (two terminals)

```bash
# terminal 1
cd packages
pnpm dev:server    # Fastify + tsx watch, :8080

# terminal 2
cd packages
pnpm dev:frontend  # Next.js + Turbopack, :3000
```

### Option 3 — Docker (no Node install needed)

Not shipped yet. Docker packaging is on the roadmap (see
[Roadmap](#roadmap)).

---

## Your first run — the command line

The fastest way to see Promptsheon work:

```bash
# 1. Confirm the backend is up
curl http://localhost:8080/api/health
# {"status":"ok","service":"promptsheon-server", …}

# 2. Open the UI in a browser
open http://localhost:3000      # macOS
xdg-open http://localhost:3000   # Linux

# 3. Walk the wizard:
#    - Welcome
#    - Admin + org
#    - LLM provider (OpenAI / Anthropic / Bedrock / Custom)
#    - Finish
```

You'll land on the control-plane dashboard with a "Create workspace"
CTA. From there:

1. **Workspaces** → create a workspace.
2. **Projects** → create a project inside that workspace.
3. **Capabilities** → open the DAG editor (or click one of the
   templates: *Customer support triage*, *Doc Q&A*, *Blank canvas*).
4. **Save** → the manifest is hashed into the CAS.
5. **Releases** → create a v1 release; cast two non-creator
   approvals; **activate**.

That's the full maker-checker loop. Once activated, every
`POST /api/executions` call routes through that release.

---

## Your first run — TypeScript SDK

If you'd rather drive the API from your own code, the repo ships a
typed fetch client in `frontend/src/lib/api.ts`. From a Next.js page
or any TS project:

```typescript
import { workspaceApi, capabilityApi, releaseApi } from '@/lib/api';

const ws = await workspaceApi.create({ name: 'refund-triage' });
const cap = await capabilityApi.list(projectId);
const release = await releaseApi.activate(releaseId);
// → 409 APPROVAL_REQUIRED until 2 distinct non-creator approvals
//    are on the manifest hash. The gate fires correctly now that
//    BaseRepo.findById returns camelCase rows.
```

The full type definitions and request shapes are documented in
[`packages/server/API.md`](packages/server/API.md).

---

## Configuration

All configuration is via environment variables prefixed with
`PROMPTSHEON_`. Copy `.env.example` to `.env` and edit as needed.
**Never commit `.env`** — it is in `.gitignore`.

| Variable                       | Purpose                                | Default                       |
|--------------------------------|----------------------------------------|-------------------------------|
| `PROMPTSHEON_PORT`             | HTTP listen port                        | `8080`                        |
| `PROMPTSHEON_HOST`             | Bind address                            | `127.0.0.1`                   |
| `PROMPTSHEON_DB_PATH`          | SQLite file path                        | `promptsheon.db`               |
| `PROMPTSHEON_CAS_PATH`         | Content-addressed store directory      | `.promptsheon`                |
| `PROMPTSHEON_FRONTEND_PATH`    | Path to the built frontend              | `./frontend/.next`             |
| `PROMPTSHEON_CORS_ORIGIN`      | Allowed CORS origin                     | `http://localhost:3000`        |
| `PROMPTSHEON_LOG_LEVEL`        | Pino log level                          | `info`                        |
| `PROMPTSHEON_NODE_ENV`         | `production` / `development` / `test`   | `development`                 |
| `PROMPTSHEON_AUTH`             | Enable JWT bearer-token auth            | `false`                        |
| `PROMPTSHEON_JWT_SECRET`       | HMAC secret for token verification     | `""`                           |
| `PROMPTSHEON_LLM_PROVIDER`     | `openai` / `anthropic` / `bedrock` / `custom` | `openai`         |
| `PROMPTSHEON_LLM_MODEL`        | Model id (e.g. `gpt-4`, `claude-3-5-sonnet-20241022`) | `gpt-4`        |
| `PROMPTSHEON_LLM_API_KEY_ENV`   | Env var holding the API key             | `OPENAI_API_KEY`               |
| `PROMPTSHEON_LLM_MAX_RETRIES`   | Strands retry budget                    | `5`                            |
| `PROMPTSHEON_LLM_TIMEOUT_MS`    | Per-request LLM timeout                 | `120000`                       |
| `OPENAI_API_KEY`               | OpenAI provider key                    | —                              |
| `ANTHROPIC_API_KEY`            | Anthropic provider key                 | —                              |
| `AWS_BEDROCK_REGION`           | Bedrock region (e.g. `us-east-1`)      | `us-east-1`                    |
| `PROMPTSHEON_WEBHOOK_SECRET`    | HMAC secret for incoming webhooks — **required in production** | `""`           |
| `PROMPTSHEON_ALLOW_SYSTEM_ACTOR` | Allow `X-User-Id: api` bypass — off in production | `true` (dev)             |
| `PROMPTSHEON_SELF_EVOLVE_ENABLED`     | Enable the self-evolution loop   | `false`                        |
| `PROMPTSHEON_SELF_EVOLVE_COOLDOWN_SEC` | Min seconds between re-evolves | `900`                          |
| `PROMPTSHEON_SELF_EVOLVE_MAX_CONCURRENT` | Cap concurrent evolutions per worker | `3`                       |
| `PROMPTSHEON_OTEL_ENDPOINT`    | OpenTelemetry OTLP collector URL       | `""`                           |

> 💡 For a **Custom** OpenAI/Anthropic-compatible endpoint, set
> `PROMPTSHEON_LLM_PROVIDER=custom` and supply the credentials
> inline during the onboarding wizard (Base URL + API key + Model
> name). The Settings store persists these per-org.

---

## Where to go next

For everyone:

- **[CHANGELOG](CHANGELOG.md)** — what changed and when.
- **[REST API Reference](packages/server/API.md)** — every endpoint
  with request/response shapes.
- **[AGENTS.md](AGENTS.md)** — the engineering constitution: type
  safety, validation, repo layout, testing bar, naming, lifecycle.
  Read it before opening a PR.

For operators / maintainers:

- **[CONTRIBUTING.md](CONTRIBUTING.md)** — how to set up a dev
  environment and submit changes.
- **[SECURITY.md](SECURITY.md)** — the disclosure policy. Please
  don't file security issues as public GitHub issues.
- **[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)** — the standards we
  expect everyone to follow.
- **[packages/server/README.md](packages/server/README.md)** — backend
  architecture, agent subsystems, hardening layers.
- **[packages/shared/README.md](packages/shared/README.md)** — domain
  types, Zod schemas, the SQLite migration runner, the CAS.

---

## Tech stack

| Category       | Technology                                       |
|----------------|--------------------------------------------------|
| Runtime        | Node.js ≥ 26, pnpm 11 workspaces                |
| Language       | TypeScript (strict, exactOptionalPropertyTypes)  |
| HTTP           | [Fastify 5](https://fastify.dev)                 |
| Validation     | [Zod 4](https://zod.dev)                         |
| Database       | SQLite via [better-sqlite3](https://github.com/WiseLibs/better-sqlite3) |
| AI             | [`@strands-agents/sdk`](https://github.com/strands-agents/sdk) — `Agent`, `Swarm`, `Graph` |
| LLM providers  | OpenAI, Anthropic, AWS Bedrock, custom OpenAI/Anthropic-compatible |
| Frontend       | [Next.js 16](https://nextjs.org) (App Router, Turbopack) |
| UI             | [React 19](https://react.dev), [shadcn/ui](https://ui.shadcn.com), [Tailwind v4](https://tailwindcss.com) |
| Data fetching  | [TanStack React Query 5](https://tanstack.com/query), [axios](https://axios-http.com) |
| DAG editor     | [@xyflow/react 12](https://reactflow.dev)        |
| Icons          | [lucide-react](https://lucide.dev)               |
| Observability  | [OpenTelemetry](https://opentelemetry.io), [Pino](https://getpino.io) |
| Hashing        | Node `crypto` (audit chain, manifest CAS, HMAC)  |
| Tests          | [Vitest 4](https://vitest.dev) (server + shared), [Playwright](https://playwright.dev) (frontend) |

---

## Development

```bash
pnpm install                # install all workspace deps
pnpm typecheck              # tsc across shared + server + frontend
pnpm --dir packages/server test   # vitest, 59 files / 377 cases
pnpm --dir packages/shared test   # vitest, 3 files / 29 cases
pnpm --dir frontend build          # next build
```

### Per-package workflow

```bash
# backend
cd packages/server
pnpm dev        # Fastify + tsx watch, :8080
pnpm test       # vitest
pnpm build      # tsc → dist/

# frontend
cd frontend
pnpm dev        # http://localhost:3000 (Turbopack)
pnpm test:e2e   # Playwright tier suite
pnpm build      # next build

# shared
cd packages/shared
pnpm build      # tsc → dist + copies db/migrations
```

### Commit conventions

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add canary-percent setter on release route
fix: clamp audit-chain hash to 32 bytes
docs: document PROMPTSHEON_WEBHOOK_SECRET
refactor: extract capability-version repo
test: add adversarial eval fixtures
chore: bump @strands-agents/sdk to 1.14
```

---

## Roadmap

- **v0.1.0 — v0.3.0** (shipped) — Fastify + Strands backend,
  Next.js frontend, DAG editor, releases + canary, maker-checker
  approvals, audit chain, eval suites, self-evolution loop,
  webhooks + replay protection, chaos hooks, OpenTelemetry.
- **v0.4.2** (current) — admin gates on 14 management routes,
  maker-checker gate now fires correctly for self-approvals,
  `/api/invoke` SDK alias, `/api/goals/:hash` drilldown, DAG
  editor drafts persist, `BaseRepo` camelCase mapper,
  Playwright tier suite rewritten against the new contracts
  (377 server tests + 41-route smoke + 5 new auth/forms/audit/
  manifest-detail/approvals/admin-gating tier specs).
- **v0.5.0** (next) — Docker packaging (`Dockerfile` +
  multi-stage build), production deployment guide, RBAC refinement
  on the maker-checker flow, dataset import/export.
- **Backlog** — gRPC interface alongside HTTP, multi-tenant SSO,
  Postgres adapter behind the better-sqlite3 repo layer, Helm chart
  for Kubernetes, OpenAPI → typed client codegen.

Have an idea? [Open a feature request](https://github.com/sachncs/promptsheon/issues/new).

---

## Contributing

Contributions are welcome. See
[`CONTRIBUTING.md`](CONTRIBUTING.md) for the process, coding
standards, and Conventional Commits workflow. Bug reports and
feature requests use the [issue templates](.github/ISSUE_TEMPLATE/).

The full engineering standards — type safety, validation, repo
layout, testing bar, naming, lifecycle — are codified in
[`AGENTS.md`](AGENTS.md). Read it before opening a PR.

## Code of Conduct

This project follows the
[Contributor Covenant v2.1](CODE_OF_CONDUCT.md). By participating,
you are expected to uphold that standard.

## Security

Please do **not** file security vulnerabilities as public GitHub
issues. See [`SECURITY.md`](SECURITY.md) for the disclosure policy.

### Prompt-security benchmark

The shipped scanner (`packages/server/src/security/prompt-scanner.ts`)
is exercised by a curated dataset at
[`docs/security/benchmark/dataset.json`](docs/security/benchmark/dataset.json)
covering OWASP LLM01..LLM10 plus edge cases. Run it with:

```
pnpm --filter @promptsheon/server bench:security
```

It writes [`docs/security/benchmark/RESULTS.md`](docs/security/benchmark/RESULTS.md)
with a per-case verdict + the rules that fired. CI should gate on a
100% pass rate so a regex tweak never silently regresses coverage.

### On-prem / air-gapped deploys

Government, defense, and regulated customers run on hosts with no
outbound internet. The repo ships an offline installer that bundles
every dependency, the SBOM, and a `systemd` bootstrap:

```
bash scripts/build-offline-installer.sh    # build the tarball
sudo bash bin/bootstrap.sh --fips          # install + FIPS mode
```

The step-by-step runbook —
[`docs/operations/air-gap-rhel.md`](docs/operations/air-gap-rhel.md) —
covers pre-flight, FIPS-mode requirements, upgrades, backups,
DR, and the FIPS gate's `refuse to boot` contract.

### Running the firewall sidecar

The firewall sits in front of *any* LLM application (not just
promptsheon-managed ones) and inspects every prompt + response
against the T2-3 scanner. Block / warn / allow decisions are
written to the audit chain so `/api/audit/verify` covers sidecar
traffic end-to-end.

```bash
# Start the sidecar with an OpenAI-compatible upstream:
PROMPTSHEON_FIREWALL_UPSTREAM_URL=https://api.openai.com \
PROMPTSHEON_FIREWALL_PORT=9090 \
  pnpm --filter @promptsheon/server firewall
```

Point any client at `http://127.0.0.1:9090/v1/chat/completions`
instead of the upstream URL. The firewall transparently forwards
when the scanner verdict is `clean`, attaches an
`X-Promptsheon-Warning` header on `warn`, and rejects with
`422 PROMPT_BLOCKED`. The implementation lives at
`packages/server/src/firewall/`; the policy + scanner extension
shipped with T3-5 carries over unchanged.

### Framework integrations

`packages/sdk/src/integrations/` ships adapters for the three
agent frameworks the doc names. All three route through the
promptsheon OpenAI-compatible gateway so caching + the audit
chain apply transparently.

```ts
// Vercel AI SDK
import { openai } from '@ai-sdk/openai';
import { withPromptsheon } from '@promptsheon/sdk/integrations/vercel-ai-sdk';
const model = withPromptsheon(openai('gpt-4'), {
  gatewayUrl: 'https://promptsheon.example.com',
  apiKey: process.env.PROMPTSHEON_API_KEY!,
});

// LlamaIndex
import { PromptsheonLLM } from '@promptsheon/sdk/integrations/llamaindex';
const llm = new PromptsheonLLM({
  gatewayUrl: 'https://promptsheon.example.com',
  apiKey: process.env.PROMPTSHEON_API_KEY!,
  model: 'gpt-4',
});

// Haystack
import { PromptsheonGenerator } from '@promptsheon/sdk/integrations/haystack';
const generator = new PromptsheonGenerator({
  gatewayUrl: 'https://promptsheon.example.com',
  apiKey: process.env.PROMPTSHEON_API_KEY!,
  model: 'gpt-4',
});
```

The adapters use structural typing (no `@ai-sdk/provider`,
`llama-index-core`, or `@haystack/core` runtime dep) so the SDK
stays framework-optional — install the framework package
yourself and pass a model that satisfies the shape. 9 vitest
cases exercise the wire format against an in-process
OpenAI-shaped stub.

## License

[Apache-2.0](LICENSE) © 2026 Sachin — **sachncs@gmail.com**.
