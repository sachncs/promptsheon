---
layout: page
title: FAQ
subtitle: Common questions about Promptsheon.
---

# FAQ

This page answers the questions we hear most often. If you don't see your question here, open a [GitHub Discussion](https://github.com/sachncs/promptsheon/discussions) or file an issue.

## Why another prompt-management platform?

Most prompt platforms are SaaS. Promptsheon is self-hosted and designed to live behind your firewall. It runs on a single Linux box with SQLite, ships with a SOC2 control set, and supports air-gapped deployments out of the box.

## What's the difference between a capability, a version, and a release?

A **capability** is the named thing — for example, *Refund triage*. A **version** is an immutable snapshot of a capability's manifest. A **release** packages a version with an environment (staging or production) and a canary split.

You author the manifest once per version. You create a release every time you promote a version to an environment.

## How does the maker-checker gate work?

The release creator is recorded when the release is created. Any attempt to self-approve returns `409 APPROVAL_REQUIRED`. The release activates once two distinct, eligible approvals exist on the manifest hash.

The eligibility rules are in [`packages/server/src/routes/releases.ts`](https://github.com/sachncs/promptsheon/blob/master/packages/server/src/routes/releases.ts).

## How does the audit chain work?

Every state transition, every approval, every execution, and every cost rollup appends a frame to a hash-linked log. Each frame stores a monotonic index, the previous frame's hash, a timestamp, the actor, the action, the resource, and a structured payload.

The chain is publicly verifiable at `GET /api/audit/verify`. Tampering with any frame breaks the chain and the endpoint returns `{"valid": false, "brokenAt": N}`. The chain is never swept by the retention sweeper.

## Can I use a custom LLM endpoint?

Yes. Pick *Custom endpoint* during onboarding and paste the base URL, the model name, and the API key. The settings store persists them per organisation. Any OpenAI-shaped or Anthropic-shaped endpoint works.

## How do I add an integration?

The SDK lives at [`packages/sdk/src/integrations/`](https://github.com/sachncs/promptsheon/tree/master/packages/sdk/src/integrations). The shipped adapters cover Vercel AI SDK, LlamaIndex, and Haystack. Each adapter uses structural typing, so you can also write your own adapter against any framework by satisfying the wire format.

## Can I run Promptsheon on Postgres?

Not today. The repo layer is built on `better-sqlite3` and the migrations target SQLite. A Postgres adapter is on the roadmap (see the [README roadmap](https://github.com/sachncs/promptsheon#roadmap)). Until then, SQLite handles thousands of executions per second with the right tuning.

## How does the firewall sidecar work?

The sidecar is a small Fastify plugin that runs the scanner (`packages/server/src/security/prompt-scanner.ts`) on every prompt before it reaches the upstream. It exposes `POST /v1/chat/completions` and forwards OpenAI-shaped and Anthropic-shaped requests transparently.

Every call — `allow`, `warn`, `block` — writes an audit entry, so `/api/audit/verify` covers sidecar traffic end-to-end.

## What's the difference between Bearer and SVID auth?

Bearer tokens are opaque API keys minted by an admin. They're sha256-hashed in the `api_keys` table. Use them for human users and SDK callers.

SVIDs are SPIFFE Verifiable Identity Documents — ed25519-signed JWTs that carry the agent's id, organisation, and classification. Use them for autonomous agents. The server verifies the SVID against `PROMPTSHEON_SVID_PUBLIC_KEY_PEM` and stamps the request with an `Agent` principal that the Cedar gate can authorise against.

## How do I run the prompt-security benchmark?

```bash
pnpm --filter @promptsheon/server bench:security
```

The runner executes the 53-case dataset at [`docs/security/benchmark/dataset.json`](https://github.com/sachncs/promptsheon/blob/master/docs/security/benchmark/dataset.json) against the scanner and writes the per-case verdicts to [`docs/security/benchmark/RESULTS.md`](https://github.com/sachncs/promptsheon/blob/master/docs/security/benchmark/RESULTS.md). It exits non-zero on any regression so a regex tweak can never silently weaken coverage.

## How do I report a security vulnerability?

Follow the disclosure process in [`SECURITY.md`](https://github.com/sachncs/promptsheon/blob/master/SECURITY.md). Do **not** open a public GitHub issue.

## Where to go next

| You want to… | Read this |
|--------------|-----------|
| Read the architecture | [Architecture]({{ '/architecture/' | relative_url }}) |
| Walk through a guided build | [Tutorials]({{ '/tutorials/' | relative_url }}) |
| Copy a working snippet | [Recipes]({{ '/recipes/' | relative_url }}) |
| Contribute code | [Contributing]({{ '/contributing/' | relative_url }}) |
