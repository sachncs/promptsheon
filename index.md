---
layout: page
title: Promptsheon
subtitle: Git-native version control for AI agents.
---

# Promptsheon

Promptsheon is a self-hosted platform for shipping AI agents to production. It answers one question:

> *How do you author a multi-agent prompt, ship it safely, prove it doesn't regress, and roll it back when it does?*

You get a Fastify backend on `:8080`, a Next.js UI on `:3000`, a local SQLite file, and a content-addressed store. No cloud account, no signup, no telemetry. Run `pnpm install && pnpm dev` and you have a fully working prompt-management platform with multi-provider LLM, an audit chain, webhooks, eval scorers, and a live DAG editor.

## Why Promptsheon?

- **Author prompts as DAGs.** Drag-and-drop nodes for *Planner*, *Agent*, *Tool*, and *Guardrail*. Each node carries its own config and a live execution preview.
- **Ship with canary rollout.** Versioned releases, per-environment activation, weighted traffic split, one-click rollback.
- **Make and check approvals.** The release creator can't approve their own release. Approvals persist with the reason and the voter.
- **Prove no regressions.** Declarative eval suites, pluggable scorers (LLM-judge, regex, exact-match), parallel run results.
- **Self-evolve on regression.** A scheduler watches live eval scores and re-plans the manifest when a release drifts.
- **Audit every action.** Append-only, hash-linked audit log with a public verification endpoint.

## Get started in 60 seconds

```bash
git clone https://github.com/sachncs/promptsheon.git
cd promptsheon
pnpm install
cp .env.example .env
pnpm dev
```

The backend listens on `http://localhost:8080` and the UI on `http://localhost:3000`. No external services beyond the LLM provider you choose.

Continue with [Getting started]({{ '/getting-started/' | relative_url }}) for a guided tour.

## Architecture at a glance

```
┌────────────────────────────────────────────────────────────┐
│                        Browser                             │
│                  Next.js 16 + React 19                     │
└──────────────────┬─────────────────────────────────────────┘
                   │ /api/* (rewritten to :8080)
┌──────────────────▼─────────────────────────────────────────┐
│                       Fastify 5                            │
│ ┌────────────┬────────────┬────────────┬────────────────┐  │
│ │  Strands   │  Cedar     │   Audit    │    Repos /     │  │
│ │  Agents    │  Policy    │   Chain    │    CAS /       │  │
│ │  (Swarm,   │  Authorizer│  (hash-    │    Manifests   │  │
│ │   Graph)   │            │   linked)  │                │  │
│ └────────────┴────────────┴────────────┴────────────────┘  │
└──────────────────┬─────────────────────────────────────────┘
                   │
                   ▼
        ┌──────────────────────┐
        │  better-sqlite3 +    │
        │  content-addressed   │
        │  store on disk       │
        └──────────────────────┘
```

Every AI call goes through the [Strands Agents SDK](https://github.com/strands-agents/harness-sdk). Authorization is enforced by [Cedar](https://www.cedarpolicy.com/) policies under `packages/server/policies/`. The audit chain is hash-linked and publicly verifiable at `/api/audit/verify`.

## Where to go next

| You want to… | Read this |
|--------------|-----------|
| Try it locally in 5 minutes | [Getting started]({{ '/getting-started/' | relative_url }}) |
| Install on a server or container | [Installation]({{ '/installation/' | relative_url }}) |
| Tune environment variables | [Configuration]({{ '/configuration/' | relative_url }}) |
| Understand the model | [Core concepts]({{ '/core-concepts/' | relative_url }}) |
| Hit the HTTP API | [API reference]({{ '/api-reference/' | relative_url }}) |
| Read the architecture | [Architecture]({{ '/architecture/' | relative_url }}) |
| Walk through a guided build | [Tutorials]({{ '/tutorials/' | relative_url }}) |
| Copy a working snippet | [Recipes]({{ '/recipes/' | relative_url }}) |
| Solve a specific problem | [FAQ]({{ '/faq/' | relative_url }}) |
| Contribute code | [Contributing]({{ '/contributing/' | relative_url }}) |
| Report a security issue | [Security]({{ '/security/' | relative_url }}) |

## License

Promptsheon is licensed under the [Apache License 2.0](https://github.com/sachncs/promptsheon/blob/master/LICENSE).
