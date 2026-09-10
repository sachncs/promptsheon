# Market gaps & moat plan — promptsheon

A survey of the LLM-ops and prompt-management market, what
promptsheon already does, what's missing, and what — built
intentionally — would actually be defensible against the cloud
SaaS incumbents (LangSmith, Helicone, Portkey, Langfuse,
Braintrust, Humanloop, Vellum).

## TL;DR

> The market is dominated by **cloud-only** observability tools.
> promptsheon's real opportunity is the **on-prem,
> governance-first** slice of that market — regulated industries
> where data sovereignty is non-negotiable and a maker-checker
> release workflow is mandatory. To win there, promptsheon needs
> three things that the cloud SaaS tools ship today and
> promptsheon doesn't, then several things that no one ships
> well yet.

The moat is not "match LangSmith feature-for-feature." It's
"be the only LLM-ops platform a CISO at a bank will let you
self-host, that has git-native release artifacts, and that
audits every byte of every prompt that leaves the building."

---

## 1. What's in the market

| Tool | Form factor | Moat claim | Open-source | Self-host |
|---|---|---|---|---|
| **LangSmith** | Cloud (hosted option limited) | LangChain-bundled; best observability UX | Partial | Limited |
| **Helicone** | Cloud | AI gateway + cheap logs; OpenAI drop-in | Yes (light) | Yes |
| **Portkey / PRISMA AIRS** | Cloud (Palo Alto acquisition) | 250+ models, gateway, ISO 27001/SOC2/HIPAA/GDPR | Yes (gateway) | Yes |
| **Langfuse** | Open-source core + cloud | OpenTelemetry-native; broad eval tooling | Yes (full) | Yes |
| **Braintrust** | Cloud | Eval-first; strong online-eval story | No | No |
| **Humanloop** | Cloud | Prompt management UX; integrations | No | No |
| **Comet** | Cloud | ML + LLM ops; broader data-science angle | Partial | No |
| **Vellum** | Cloud | Prompt + eval UX; enterprise | No | No |

**Common feature surface** (everyone has these):
LLM tracing (span tree), prompt playground, prompt versioning,
dataset management, evaluation suite (LLM-as-judge, code, regex),
A/B experiments, dashboards, alert routing, cost tracking.

**What's actually different across vendors:**
- LangSmith: UX + LangChain lock-in.
- Helicone: gateway-first, lightweight, cheap.
- Portkey: gateway + enterprise security posture + Palo Alto distribution.
- Langfuse: open-source + OTel-native + ecosystem integrations.
- Braintrust: online eval automation.

What **no one** does well yet:
- A real **maker-checker git-native release pipeline** (promptsheon has this).
- **Self-hosted with full feature parity** (Langfuse gets closest but its eval story is weaker than Braintrust's; no one ships governance).
- A **first-class governance model** that an enterprise CISO will sign off on.

---

## 2. What promptsheon has today

Sourced from `frontend/src/app/`, `packages/server/src/routes/`, and `packages/shared/src/`.

### Capability authoring
- DAG editor (Planner / Agent / Tool / Guardrail nodes; @xyflow/react).
- Three starter templates; live per-node config panel.
- Manifest content-addressed via raw-string SHA-256.
- Validation against `ManifestSchema` (Zod) + DAG-cycle check.
- `mergeDraftManifest()` so partial editor payloads persist.

### Release governance
- 6-state machine (`draft → review → approved → canary → active → rolled_back`).
- **Maker-checker gate**: ≥2 distinct non-creator approvers required; creator cannot self-approve.
- Canary routing (`canaryPercent` weighted split across active releases for the same manifest).
- One-click rollback / supersede.
- Git-native: `merge_requests` with author ≠ reviewer enforcement.
- Per-org **operator signing keys** (ed25519).

### Eval + scoring
- Eval suites, 4 deterministic graders (regex, schema, tool-call, transcript).
- `passAtK` / `pass^k`, Cohen's κ / Krippendorff's α calibration.
- Human-review queue.
- Self-evolution loop that re-plans on live eval regression.
- Online evaluation API surface (`/api/eval/run`, `/api/eval/score`).

### Observability + audit
- OpenTelemetry + Pino structured logs.
- SSE event stream at `/api/events/:channel`.
- **Append-only hash-linked audit chain** with `/api/audit/verify`.
- Cost rollups by capability / by day.

### Security + ops
- Webhook receiver with HMAC + replay protection.
- AES-256-GCM vault with KMS abstraction (`AwsSecretsManagerKms`, etc.).
- Per-org residency (`local | us | eu | ap | sa | me | af`).
- Chaos engineering hooks.
- Admin gates on 14 management routes; role-escalation cap.
- Production refuses to boot without `PROMPTSHEON_WEBHOOK_SECRET`.

### Stack
- Fastify 5 + better-sqlite3 + Zod + 41+ SQLite migrations.
- Strands Agents SDK (Agent / Swarm / Graph) for every AI call.
- Next.js 16 + TanStack Query + shadcn/ui (no Vite, no React Router v7).

### Test posture
- 377 server vitest cases + Playwright tier suite rewritten to use the new admin gates.
- typecheck clean (shared + server + frontend, strict mode).

---

## 3. What's missing, ranked by moat impact

Tier numbering = priority for shipping, not severity.
Tier 1 features make promptsheon a credible alternative to the
cloud SaaS tools. Tier 2 features defend the on-prem / regulated
segment. Tier 3 features would create a category.

---

### Tier 1 — without these, promptsheon is a curiosity

#### T1-1. **Span-level LLM tracing**
- **Why it matters**: every competitor's primary UI is the trace viewer. promptsheon has audit-chain entries but not the rich tree of `LLM call → retrieval → tool use → response` spans that engineers use to debug.
- **Where it lives today**: `packages/server/src/agents/executor/` emits execution traces but they aren't persisted to a queryable store. OpenTelemetry is set up but no span exporter fires for agent runs.
- **What's missing**: a `trace_runs` table + a `/app/traces` page with span tree, token counts, latency histograms, and the canonical Langfuse/LangSmith feature set: filter by userId, session, tag, latency, cost, model. Span-level exporter so existing OTel collectors consume it.
- **Moat impact**: low alone, but **table stakes**. Without this, an engineer evaluating promptsheon vs. LangSmith deletes promptsheon in the first 5 minutes of demo.

#### T1-2. **Prompt playground + parameter sweep**
- **Why it matters**: every competitor has a chat-style playground where you iterate prompt + model + temperature side-by-side. promptsheon's editor is for DAGs — there's no surface for "I have one prompt, give me the curl/sdk/UI to try it against 3 models with different temperatures."
- **Where it lives today**: `frontend/src/app/app/editor/` is graph-only. There's no `/app/playground` route.
- **What's missing**: a chat interface that lets a developer paste a prompt, pick the model, stream the response, and diff two runs side by side. Parameter sweep UI that runs N variants and ranks by latency/cost/quality.
- **Moat impact**: low alone. But **the missing tool that every solo prompt-engineer needs**.

#### T1-3. **LLM gateway — caching, fallback, routing**
- **Why it matters**: Helicone + Portkey's entire value prop is "drop our OpenAI SDK in front of your code and you get caching, fallbacks, rate limiting, cost tracking." promptsheon has provider support but not a runtime gateway with caching.
- **Where it lives today**: `packages/server/src/llm/router.ts` routes calls but doesn't cache responses or implement fallback chains.
- **What's missing**: a content-hash-keyed response cache (so identical prompts are free), provider fallback chains (`openai → anthropic` if one fails), per-user rate limits, and prompt-template caching (treat `{hash}` placeholders as cache keys, not literal strings).
- **Moat impact**: **high**. promptsheon becomes the cheapest-to-run LLM ops platform, which is sticky once teams adopt it. Caching at the gateway level is the single biggest cost lever most teams need.

#### T1-4. **Online evaluation on production traces**
- **Why it matters**: Braintrust + Langfuse ship LLM-as-judge that runs on every production trace. promptsheon has eval suites for offline runs but not online trace-attached scoring.
- **Where it lives today**: the audit chain records actions but no evaluation results are attached to execution traces.
- **What's missing**: a `trace_scores` table; an `/api/traces/:id/scores` endpoint; built-in evaluators (hallucination, toxicity, prompt-injection, answer-relevance, custom); auto-evaluation on every execution.
- **Moat impact**: medium. Differentiation comes from **shipping the eval library + the trace store** as one product. Both already have traces; the eval library is where Langfuse/Braintrust add value.

#### T1-5. **Customer-facing analytics (per-user, per-tenant)**
- **Why it matters**: every SaaS product can tell you "user X used 4,200 tokens today on this prompt." promptsheon has system-level cost rollups but not per-end-user analytics.
- **Where it lives today**: `packages/server/src/repos/vault-extras.ts` CostRollupRepo aggregates by capability/day, not by user.
- **What's missing**: a `user_id` field on every execution row (currently nullable), a per-user dashboard, per-tenant quota management, and a "show me which prompt is being abused by which user" view.
- **Moat impact**: high for SaaS-style customers; medium for self-hosted (most self-hosted customers don't bill per-user internally yet).

---

### Tier 2 — these defend the regulated / on-prem segment

#### T2-1. **Compliance certifications (SOC 2, HIPAA, GDPR, ISO 27001)**
- **Why it matters**: every enterprise procurement team asks for these. promptsheon has zero today. Even a self-hosted product is held to SOC 2 for the **company** that ships it. LangSmith, Portkey, Helicone all have at least SOC 2 + ISO 27001.
- **What's missing**: external audit (Wyndham / Vanta / Drata), controls documentation, pen-test report, SBOM, vendor risk questionnaire responses. This is a process + paperwork job, not engineering, but it's blocking enterprise sales.
- **Moat impact**: **gigantic**. Without it, no Fortune 500 deal closes. With it, promptsheon becomes the **only** self-hosted LLM-ops platform a CISO signs off on.

#### T2-2. **SSO / SCIM / RBAC / teams**
- **Why it matters**: every serious product supports OIDC + SAML. promptsheon has X-User-Id / X-Org-Id with no auth flow.
- **Where it lives today**: `packages/server/src/middleware/auth.ts` is a stub with bearer-token support but no IdP integration.
- **What's missing**: SAML 2.0 + OIDC connectors; SCIM 2.0 user provisioning; per-team RBAC (currently role is a flat per-user field); team-based audit chain partitioning.
- **Moat impact**: high. Even with self-host, an enterprise customer needs Okta / Azure AD / Google Workspace to log their engineers in. This is also a prerequisite for multi-tenant SaaS.

#### T2-3. **Prompt security: PII detection + injection / jailbreak scoring**
- **Why it matters**: every prompt leaving the building is a data exfiltration vector. Today promptsheon has runtime guardrails but no static analysis of prompt bodies.
- **Where it lives today**: `packages/server/src/agents/guardrails/` has runtime evaluators. No static scan.
- **What's missing**: scanner that classifies every saved manifest as containing: emails, SSNs, credit-card patterns, customer PII, internal URLs. Block saves with findings, require override. Plus automated red-team scan suite (promptfoo / garak integration).
- **Moat impact**: high. Compliance teams need this as evidence for SOC 2 / HIPAA / ISO 27001 controls.

#### T2-4. **Compliance reporting (audit reports, evidence packs)**
- **Why it matters**: quarterly SOC 2 audits require evidence of who changed what, when, and why. promptsheon has the data (audit chain) but no report generator.
- **Where it lives today**: `audit_entries` table is append-only; `/api/audit/verify` is the only consumer.
- **What's missing**: `GET /api/audit/report?from=&to=&actor=&resource=` returning a signed JSON document; PDF export; per-quarter evidence packs.
- **Moat impact**: medium. Builds on T2-1; without certifications it's unused.

#### T2-5. **Air-gap / FIPS-mode support**
- **Why it matters**: government and defense customers run on air-gapped networks. The npm install + GitHub-Chromium-fetch path doesn't work for them.
- **What's missing**: an offline-installer tarball that bundles every dep; verified mirror packages; FIPS-validated crypto modules (Node's `crypto` is not FIPS-validated by default).
- **Moat impact**: high — no competitor opens this segment at all because the engineering work is unprofitable without committed volume. With 2-3 anchor customers, promptsheon could own it.

---

### Tier 3 — category-defining, no competitor ships this well

#### T3-1. **Time-travel debugging**
- Replay any past execution with the **same model, same tools, same context** and watch it again. Determinism for free because the executor is recorded.
- **Why it matters**: when a prompt regressed in production, the on-call engineer wants to rewind, replay, and A/B-compare what changed.
- **Where it lives today**: `execution_results` exists but no replay endpoint. The CAS stores compiled manifests; the trace store would store inputs.
- **Moat impact**: unique to promptsheon. No competitor has this today.

#### T3-2. **Prompt firewall as a service**
- A reverse-proxy sidecar that sits in front of *any* LLM application (not just the promptsheon-managed ones), inspects every prompt + response, applies the policy engine, and logs to promptsheon's audit chain.
- **Why it matters**: CISOs buy "the thing that watches everything leaving our network." promptsheon has the audit-chain substrate; building the firewall on top is natural.
- **Moat impact**: **category-defining**. No one owns "the OpenTelemetry of LLM egress."

#### T3-3. **Managed governance tier**
- Not the multi-tenant SaaS — a separate offering where promptsheon SREs run the instance for the customer. Removes the "self-host is too hard" objection without losing the data-sovereignty positioning.
- **Why it matters**: most regulated buyers want the security posture of self-host without the operational burden.
- **Moat impact**: very high, takes time to build.

#### T3-4. **Operator-tier certification ("promptsheon certified operator")**
- A paid training + certification program for SREs to deploy, upgrade, and operate promptsheon. Creates a services partner ecosystem.
- **Why it matters**: every CISO asks "if you go out of business, who runs this?" The certification creates a bench of operators independent of promptsheon-the-company.

#### T3-5. **Prompt-security benchmarks / dataset publishing**
- Curated and published benchmark dataset of known prompt-injection / PII-exfil / jailbreak attacks that promptsheon blocks.
- **Why it matters**: "see how the firewall handles OWASP LLM01–L10" becomes a sales asset. The benchmark gets cited by analysts.

---

## 4. Tier 4 — catch-up features (no moat, just parity)

These keep promptsheon credible but won't differentiate.

| Feature | Notes |
|---|---|
| **Streamed completions over SSE** | promptsheon has SSE infrastructure; need a streaming response path on `/api/executions`. |
| **Multi-region replication** | Read replicas across regions; today single SQLite file. Important for global SaaS, less so for self-host. |
| **Per-prompt A/B experiments with statistical significance** | The `experiment` repo already exists; missing the chi-squared / Bayesian band-it output. |
| **Cost forecast / budget alerts** | CostRollupRepo already aggregates; missing the forecast + alert layer. |
| **Multi-region active-active failover** | Future infrastructure work. |
| **Embedding / vector store / RAG connectors** | A separate product category; partner with Pinecone / Qdrant / Weaviate instead. |
| **Fine-tuning pipelines** | Open-weights fine-tuning only; partner with Hugging Face or provider-native. |
| **Prompt templates marketplace / community** | Network-effect play. Don't build until T1-T2 are done. |
| **VS Code extension for prompt authoring** | Nice-to-have. Low ROI until the playground (T1-2) lands. |
| **CLI improvements** | Basic CLI exists; needs better scripting + CI integration. |
| **More framework integrations** | Vercel AI SDK, LlamaIndex, Haystack. Reach-based; low moat. |

---

## 5. Recommended sequencing — three horizons

### Horizon 1 — "credible alternative" (3–6 months)

Make promptsheon a credible alternative to LangSmith cloud for
non-regulated use cases. Goal: stop losing "we already have
LangSmith" deals.

| Order | Feature | Why this order |
|---|---|---|
| 1 | T1-1 trace store + viewer | table stakes for any engineer evaluating the platform |
| 2 | T1-2 prompt playground | the missing daily-driver surface |
| 3 | T1-3 LLM gateway with cache + fallback | the cost lever that wins procurement |
| 4 | T1-4 online eval on traces | completes the eval story |

By end of horizon 1, promptsheon is in the same conversation as
LangSmith + Langfuse for any team that doesn't require
self-hosting.

### Horizon 2 — "defensible for regulated" (6–12 months)

Pivot hard into the on-prem / regulated segment. Stop competing
for cloud-only buyers.

| Order | Feature | Why this order |
|---|---|---|
| 1 | T2-1 SOC 2 + ISO 27001 certification | unblocks every enterprise deal |
| 2 | T2-2 SSO + SCIM + RBAC | required for the same deals |
| 3 | T2-3 prompt security scanner | closes the CISO objection |
| 4 | T2-4 audit report exports | evidence packs for auditors |
| 5 | T2-5 air-gap installer + FIPS mode | opens defense / gov segment |

By end of horizon 2, promptsheon is the **only** LLM-ops
platform a CISO will sign off on for self-host.

### Horizon 3 — "category-defining" (12–24 months)

Open new categories no one else has.

| Order | Feature | Why this order |
|---|---|---|
| 1 | T3-1 time-travel debugging | builds on T1-1 trace store; naturally fits after |
| 2 | T3-2 prompt firewall sidecar | builds on T2-3 security scanner + T1-1 traces |
| 3 | T3-5 published prompt-security benchmark | marketing asset for T3-2 |
| 4 | T3-3 managed governance tier | service revenue + reduces self-host friction |
| 5 | T3-4 operator certification program | ecosystem play; sustainable moat |

---

## 6. What to NOT build (and why)

Things that look like moats but aren't:

- **A multi-tenant SaaS** to compete with LangSmith cloud head-on.
  promptsheon has $0 of the infrastructure cost advantages that
  cloud-native competitors have. Competing on cloud pricing
  against Langfuse and Helicone is a losing move. Sell to the
  segments the cloud SaaS can't reach.

- **A general-purpose LLM framework.** Strands already is one.
  Building another is waste.

- **A vector database / RAG platform.** That's Pinecone's /
  Qdrant's / Weaviate's budget. Partner.

- **Custom-evaluation marketplace.** Network-effect plays fail
  without scale. Build after T1-T2.

- **More LLM providers.** Each provider adds maintenance debt
  for tiny incremental reach. Be deliberate; pick the providers
  that match the customer.

---

## 7. Suggested next steps

1. **Validate the regulated-segment thesis with 5 customer interviews**
   before any Tier 2 spend. If the answer is "we'd use LangSmith
   cloud" the whole strategy shifts. If the answer is "we can't use
   cloud because data sovereignty" — proceed.
2. **Ship Tier 1 in 3 months.** Even a stub playground + working
   trace store moves promptsheon from "interesting OSS" to
   "credible product."
3. **Pick a beachhead industry** (banking? healthcare? defense?)
   and over-invest in their specific needs. Don't try to be all
   things to all regulated buyers — that's Portkey's path and
   they've already lost to the cloud incumbents there.
4. **Write the on-prem deployment story.** Most OSS tools
   under-document this. A good "deploy to air-gapped RHEL"
   guide is itself a moat.

---

## 8. Source notes

- [LangSmith docs](https://docs.smith.langchain.com/) —
  feature surface, observability framing.
- [Helicone quickstart](https://docs.helicone.ai/) — gateway-first
  positioning.
- [Langfuse overview](https://docs.langfuse.com/) — open-source
  benchmark, OTel-native, eval tooling breadth.
- [Portkey docs](https://portkey.ai/docs) — gateway + enterprise
  security posture (ISO 27001, SOC 2, HIPAA, GDPR).
- promptsheon's own `CHANGELOG.md` (v0.4.2) for the current feature
  surface and what shipped in the v0.4 audit cycle.

This document is a strategy brief, not a sprint plan. Convert the
recommended horizons into a quarterly roadmap once the beachhead
is picked.
