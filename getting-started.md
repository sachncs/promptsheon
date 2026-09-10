---
layout: page
title: Getting started
subtitle: Run Promptsheon locally and ship your first release in ten minutes.
---

# Getting started

This page walks you through your first end-to-end run of Promptsheon on a developer laptop. By the end, you will have:

1. A working backend on `http://localhost:8080` and a UI on `http://localhost:3000`.
2. An organisation, a workspace, and a capability version you authored.
3. A release with two non-creator approvals and an activated canary.

## Before you start

You need:

- **Node.js 26 or newer.** Check with `node --version`. If you need to install it, follow the [official Node.js guide](https://nodejs.org/en/download/package-manager).
- **pnpm 11.** Enable it with `corepack enable && corepack prepare pnpm@11 --activate`.
- **At least one LLM provider key.** OpenAI, Anthropic, AWS Bedrock, or any OpenAI/Anthropic-compatible endpoint.

If you've never written TypeScript before, that's fine. You don't need to touch code to use Promptsheon. If you've used Fastify or Next.js before, you can skim this page.

## Install

```bash
git clone https://github.com/sachncs/promptsheon.git
cd promptsheon
pnpm install
cp .env.example .env
$EDITOR .env   # fill in OPENAI_API_KEY (or ANTHROPIC_API_KEY)
pnpm dev
```

The `pnpm dev` script launches the backend and the frontend together. You see two log streams side by side. The backend listens on `:8080`; the UI listens on `:3000`.

> Tip — the frontend's `next.config.ts` rewrites every `/api/*` request to `http://localhost:8080/api/*` automatically. You only need both servers running.

## Walk the onboarding wizard

Open `http://localhost:3000` in a browser. The wizard runs you through four steps:

1. **Welcome.** Click *Begin setup*.
2. **Admin and organisation.** Enter your name, your email, and an organisation name.
3. **LLM provider.** Pick *OpenAI*, *Anthropic*, *Bedrock*, or *Custom endpoint*. If you pick a custom endpoint, paste the base URL, the model name, and the API key.
4. **Finish.** Click *Open the control plane*.

You land on the dashboard. From here you build your first release.

## Create your first release

The full maker-checker loop runs in five clicks:

1. **Workspaces** → click *Create workspace* → name it (for example, *Refund triage*).
2. **Projects** → click *Create project* inside the workspace.
3. **Capabilities** → click one of the templates (*Customer support triage*, *Doc Q&A*) or start from *Blank canvas*. The DAG editor opens.
4. **Save.** The manifest is hashed into the content-addressed store. You see a *Versions* panel with `v1`.
5. **Releases** → click *New release* → pick the capability, the version, and an environment. Cast two non-creator approvals. **Activate.**

Once activated, every `POST /api/executions` call routes through that release. The audit chain records every transition.

## Drive the API from code

If you'd rather script the loop, use the typed SDK in `frontend/src/lib/api.ts`. From a Next.js page or any TypeScript project:

```typescript
import { workspaceApi, capabilityApi, releaseApi } from '@/lib/api';

const ws = await workspaceApi.create({ name: 'refund-triage' });
const cap = await capabilityApi.list(projectId);
const release = await releaseApi.activate(releaseId);
// → 409 APPROVAL_REQUIRED until 2 distinct non-creator approvals
//    are on the manifest hash.
```

The full type definitions and request shapes are documented in [API reference]({{ '/api-reference/' | relative_url }}).

## Verify the install

Run this in a second terminal:

```bash
curl http://localhost:8080/api/health
# {"status":"ok","service":"promptsheon-server", …}

curl http://localhost:8080/api/audit/verify
# {"valid": true, "frameCount": 1, "lastHash": "…"}
```

The `/api/health` response tells you the backend booted cleanly. The `/api/audit/verify` response tells you the audit chain was seeded and the hash link holds.

## Where to go next

| You want to… | Read this |
|--------------|-----------|
| Install on a server or container | [Installation]({{ '/installation/' | relative_url }}) |
| Tune environment variables | [Configuration]({{ '/configuration/' | relative_url }}) |
| Understand the model | [Core concepts]({{ '/core-concepts/' | relative_url }}) |
| Hit the HTTP API | [API reference]({{ '/api-reference/' | relative_url }}) |
| Read the architecture | [Architecture]({{ '/architecture/' | relative_url }}) |
