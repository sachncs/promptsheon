# Promptsheon

Promptsheon is an adaptive agent engineering platform for building,
executing, evaluating, and continuously improving AI agents and multi-agent
systems.

It gives teams one controlled path from an agent specification to an
evidence-backed release:

```text
Compose → Execute → Observe → Evaluate → Compare → Improve → Promote
```

Promptsheon is self-hosted, open source, and designed for teams that need
lineage, evaluation, policy, auditability, and rollback around agentic
software—not another prompt notebook or opaque hosted runtime.

[Product site](https://sachncs.github.io/promptsheon/) · [Developer documentation](https://sachncs.github.io/promptsheon/docs/) · [Issues](https://github.com/sachncs/promptsheon/issues)

![CI](https://github.com/sachncs/promptsheon/actions/workflows/ci.yaml/badge.svg?branch=master)
![License](https://img.shields.io/badge/license-Apache--2.0-blue)
![Node](https://img.shields.io/badge/node-%3E%3D26-green)

## What it provides

- **Agent specifications** — compose prompts, models, tools, context,
  guardrails, permissions, budgets, and evaluation intent as one versioned
  artifact.
- **DAG execution** — run dependent work with routing, retries, parallelism,
  tool calls, state, and resource limits.
- **Evaluation and regression gates** — compare quality, latency, token use,
  cost, tool behaviour, and failure modes against repeatable workloads.
- **Controlled releases** — use approvals, canaries, signed commits, explicit
  transitions, and rollback.
- **Content-addressed lineage** — identify the exact manifest that was
  evaluated and promoted.
- **Governance and operations** — Cedar authorization, organization scope,
  hash-linked audit records, webhooks, health/readiness checks, and optional
  OpenTelemetry instrumentation.
- **Programmatic access** — Fastify HTTP API, typed TypeScript SDK, and CI CLI.

## Quickstart

### Requirements

- Node.js 26.8.1 or newer (`.nvmrc` is included).
- pnpm 11.23.0.
- An LLM provider credential for workflows that invoke an agent.

```bash
git clone https://github.com/sachncs/promptsheon.git
cd promptsheon
nvm use
corepack enable
corepack prepare pnpm@11.23.0 --activate
pnpm install
cp .env.example .env
$EDITOR .env
pnpm dev
```

Open `http://localhost:3000`. The Fastify API listens on
`http://localhost:8080`; the Next.js console proxies `/api/*` to it locally.

```bash
curl http://localhost:8080/api/health
curl http://localhost:8080/api/ready
```

The first useful workflow is:

1. Complete onboarding and create a workspace.
2. Create a project and capability.
3. Compose the agent graph and save its manifest.
4. Attach an evaluation suite and run a candidate.
5. Collect independent approvals and activate a release.
6. Compare the active release with the next candidate before promoting it.

Read the [quickstart](https://sachncs.github.io/promptsheon/docs/quickstart/) for the guided path.

## Integration surfaces

### HTTP API

Use the API for automation and custom control planes:

```bash
curl "$PROMPTSHEON_API_URL/api/releases" \
  -H "Authorization: Bearer $PROMPTSHEON_API_KEY" \
  -H "Accept: application/json"
```

The running server exposes OpenAPI at `/api/openapi.json`. See the [API and SDK guide](https://sachncs.github.io/promptsheon/docs/api/).

### TypeScript SDK

The workspace package is currently private and builds from this repository:

```bash
pnpm --filter @promptsheon/sdk build
```

```ts
import { PromptsheonClient } from '@promptsheon/sdk';

const client = new PromptsheonClient({
  baseUrl: process.env.PROMPTSHEON_API_URL,
  apiKey: process.env.PROMPTSHEON_API_KEY,
});

const repositories = await client.listRepos(process.env.PROMPTSHEON_WORKSPACE_ID!);
console.log(repositories);
```

Keep API keys on the server. Never read platform or provider secrets from browser bundles.

### CLI

```bash
pnpm --filter @promptsheon/cli build
export PROMPTSHEON_API_URL=http://127.0.0.1:8080
export PROMPTSHEON_API_KEY=pk_your_key_here
export PROMPTSHEON_WORKSPACE_ID=workspace-uuid

node packages/cli/dist/index.js login
node packages/cli/dist/index.js repos list --json
node packages/cli/dist/index.js release get "$RELEASE_ID" --json
node packages/cli/dist/index.js release approve "$RELEASE_ID" --dry-run --json
```

## Repository map

```text
packages/shared/       contracts, validation, migrations, pure logic
packages/server/       Fastify API, application services, repos, agents
packages/cli/          CI and operator command-line adapter
packages/sdk/          typed client and framework integrations
frontend/              Next.js 16 console and Playwright suite
site/                  Astro product website and developer docs
```

The intended dependency direction is:

```text
HTTP / UI adapters → application use cases → domain contracts
                         ↓                    ↑
                  infrastructure ports ← adapters
```

Routes validate and translate input. Application services own workflow
decisions. Repositories own prepared SQLite access and scoping. Concrete
adapters are assembled in the composition root. Read the [architecture guide](https://sachncs.github.io/promptsheon/docs/architecture/)
and [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md).

## Development commands

```bash
pnpm dev
pnpm dev:server
pnpm dev:frontend
pnpm dev:site
pnpm typecheck
pnpm test
pnpm --dir frontend lint
pnpm --dir frontend test:e2e
pnpm --dir frontend build
pnpm --dir site build
```

CI also builds the public site so documentation regressions cannot merge unnoticed.

## Production

Production requires explicit authentication, CORS, webhook and SCIM secrets,
persistent SQLite/CAS storage, and readiness gating. Configure `.env.example`
through your deployment secret manager; never commit credentials or bake them
into images.

```bash
docker build -t promptsheon:latest .
docker run --rm -p 8080:8080 \
  -e PROMPTSHEON_AUTH=true \
  -e PROMPTSHEON_JWT_SECRET="$PROMPTSHEON_JWT_SECRET" \
  -e PROMPTSHEON_CORS_ORIGIN="http://localhost:3000" \
  -e PROMPTSHEON_WEBHOOK_SECRET="$PROMPTSHEON_WEBHOOK_SECRET" \
  -e PROMPTSHEON_SCIM_TOKEN="$PROMPTSHEON_SCIM_TOKEN" \
  -v "$PWD/.promptsheon:/data" \
  promptsheon:latest
```

Use `/api/health` for liveness and `/api/ready` for traffic admission. Back up
SQLite and the content-addressed store together, test restoration, and keep
the last verified content hash available for rollback. Read the [deployment](https://sachncs.github.io/promptsheon/docs/deployment/), [operations](https://sachncs.github.io/promptsheon/docs/operations/), and [troubleshooting](https://sachncs.github.io/promptsheon/docs/troubleshooting/) guides.

## Security and contributing

Do not open public issues for security vulnerabilities; follow [`SECURITY.md`](SECURITY.md).
For contributions, read [`AGENTS.md`](AGENTS.md), [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md),
and the [contributing guide](https://sachncs.github.io/promptsheon/docs/contributing/).
Changes should be small, atomic, tested, and documented when they alter a public surface.

## License

Promptsheon is available under the [Apache License 2.0](LICENSE).
