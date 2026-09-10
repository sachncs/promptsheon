---
layout: page
title: Tutorials
subtitle: Step-by-step builds that end with a working capability.
---

# Tutorials

Each tutorial starts from a fresh `pnpm install` and ends with a working capability or workflow. Tutorials assume you've completed [Getting started]({{ '/getting-started/' | relative_url }}).

## Tutorial 1 — Build and activate a capability release

**Goal.** Author a *Doc Q&A* capability, create a v1 release, cast two non-creator approvals, and activate it.

**Time.** 8 minutes.

### Steps

1. In the UI, click **Workspaces** → *Create workspace* → name it `Tutorials`.
2. Click **Projects** → *Create project* → name it `Doc QA`.
3. Click **Capabilities** → *Create capability* → pick the *Doc Q&A* template → *Save*. The DAG editor populates with one `Planner`, one `Agent`, and one `Guardrail` node.
4. Click **Versions**. You see `v1` with a manifest hash. Click the hash to view the manifest.
5. Click **Releases** → *New release* → fill in:
   - Capability: `Doc QA`
   - Version: `v1`
   - Environment: `production`
   - Canary percent: `10`
6. The release is in `draft`. Cast one approval as user A and one as user B (use two browser sessions, or the API). The release transitions to `approved`.
7. Click *Activate*. The release transitions to `canary`, then to `active` once the canary window closes.
8. Call `POST /api/executions` with the capability id. The audit chain records every transition.

### Verify

```bash
curl http://localhost:8080/api/audit/verify
# {"valid": true, "frameCount": N, "lastHash": "…"}
```

## Tutorial 2 — Run an eval suite

**Goal.** Author a 10-case eval suite, run it against the v1 release, and read the per-case scores.

### Steps

1. Click **Suites** → *Create suite* → name it `Doc QA · smoke`. Add 10 cases (input + expected output).
2. Click **Run** on the suite. Pick the `Doc QA` v1 release as the target.
3. The runner streams per-case verdicts over SSE. Each case returns `pass` or `fail` with a structured explanation.
4. After the run, click **Suites** → `Doc QA · smoke` → *History*. You see the run with its overall pass-rate and the per-case breakdown.

### Verify with the API

```bash
curl -X POST http://localhost:8080/api/eval/run \
  -H "Content-Type: application/json" \
  -d '{"suiteId": "...", "releaseId": "..."}'
```

The response carries the run id. Poll `/api/eval/runs/:id` for results.

## Tutorial 3 — Self-evolve on regression

**Goal.** Watch the live eval score of an active release; when it regresses, the self-evolution loop re-plans and re-releases.

### Steps

1. Click **Settings** → *Self-evolve* → enable the loop. Set the cooldown to `60` seconds for the tutorial.
2. Activate the `Doc QA · smoke` suite to run every 60 seconds.
3. Edit the capability manifest so the `Agent` node points at a deliberately bad model (for example, `gpt-3.5-turbo`). Save as `v2`. Create a release for `v2` and activate it.
4. After a few cycles, the live eval score drops below the configured threshold. The self-evolution loop fires:
   - It re-plans the manifest with a `Compiler` agent.
   - It saves a `v3` and creates a release for it.
   - The audit chain records every step.
5. Open the **Goals** page to see the evolution history.

## Tutorial 4 — Use the firewall sidecar

**Goal.** Stand up the prompt firewall in front of any OpenAI-shaped client and observe its decisions in the audit chain.

### Steps

1. Start the sidecar:

   ```bash
   PROMPTSHEON_FIREWALL_UPSTREAM_URL=https://api.openai.com \
   PROMPTSHEON_FIREWALL_PORT=9090 \
     pnpm --filter @promptsheon/server firewall
   ```

2. Point any client at `http://127.0.0.1:9090/v1/chat/completions` instead of `https://api.openai.com/v1/chat/completions`.
3. Send a clean prompt. The sidecar forwards it and returns the upstream response. The audit chain records `firewall.allow`.
4. Send a prompt that the scanner flags (for example, one with an obvious PII pattern or a jailbreak payload). The sidecar attaches an `X-Promptsheon-Warning` header on `warn`, or rejects with `422 PROMPT_BLOCKED` on `block`. The audit chain records the verdict.

## Tutorial 5 — Replay an execution

**Goal.** Re-run a past execution with the same manifest, model, environment, and inputs, and read the per-node diff.

### Steps

1. Find an execution id from `GET /api/executions?limit=10`.
2. Call:

   ```bash
   curl -X POST http://localhost:8080/api/executions/<id>/replay
   ```

3. The response is a new execution id linked to the original via `replay_of`. The original's `replay_count` is incremented.
4. Open `/app/executions/<new-id>`. The page header reads *Replay of &lt;original-id&gt;* and links back to the original.
5. The per-node diff shows what changed across the two runs (LLMs are not deterministic).

## Where to go next

| You want to… | Read this |
|--------------|-----------|
| Copy a working snippet | [Recipes]({{ '/recipes/' | relative_url }}) |
| Solve a specific problem | [FAQ]({{ '/faq/' | relative_url }}) |
| Hit the HTTP API | [API reference]({{ '/api-reference/' | relative_url }}) |
