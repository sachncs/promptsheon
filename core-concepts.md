---
layout: page
title: Core concepts
subtitle: Workspaces, capabilities, releases, approvals, and the audit chain.
---

# Core concepts

Promptsheon models your prompt-management workflow as a small set of well-typed entities. This page defines each one and explains how they relate.

## Organisation

An **organisation** is the top-level tenant. It owns workspaces, users, API keys, webhooks, and audit-chain state. Every other entity in Promptsheon lives under exactly one organisation.

## Workspace

A **workspace** groups related projects under a shared setting (for example, a team or a business unit). Workspaces scope capability executions, eval results, and cost rollups.

## Project

A **project** lives inside a workspace. Projects are containers for capabilities and their versions. You typically have one project per use case or per release train.

## Capability

A **capability** is a *named* prompt or agent workflow. It owns a manifest that describes the DAG (directed acyclic graph) of nodes that handle a request.

## Manifest

A **manifest** is the JSON document that describes the capability's DAG. It lists nodes (`Planner`, `Agent`, `Tool`, `Guardrail`), edges, prompt text, and per-node configuration. The manifest is hashed with SHA-256 and stored in the content-addressed store (CAS) under that hash. The hash is the canonical identifier; the manifest is never looked up by name.

A manifest looks like this:

```json
{
  "name": "refund-triage",
  "version": 1,
  "nodes": [
    { "id": "plan",  "type": "Planner",  "config": { "max_steps": 5 } },
    { "id": "agent", "type": "Agent",    "config": { "model": "gpt-4", "tools": ["refund_lookup"] } },
    { "id": "guard", "type": "Guardrail","config": { "block": ["pii", "jailbreak"] } }
  ],
  "edges": [
    { "from": "plan",  "to": "agent" },
    { "from": "agent", "to": "guard" }
  ]
}
```

## Version

A **version** is an immutable snapshot of a manifest. Each save creates a new version. Versions are append-only — you cannot edit them in place. To change a capability, save a new version.

## Release

A **release** packages a version with an environment and a canary split. A release has six states:

```
draft → review → approved → canary → active → rolled_back
```

A release is the unit you approve, deploy, and roll back. The same version can have multiple releases across different environments (for example, `staging` and `production`).

## Maker-checker approval

A release cannot activate on its own. It needs **two non-creator approvals**. The release creator is recorded and any attempt to self-approve returns `409 APPROVAL_REQUIRED` until two distinct, eligible approvals exist.

Approvals persist with the voter's user id, the manifest hash, the reason, and a timestamp.

## Audit chain

Every state transition, every approval, every execution, and every cost rollup is appended to a hash-linked log. Each frame stores:

- A monotonic index.
- The previous frame's hash.
- A timestamp.
- The actor (user id, system, or agent identity).
- The action, the resource kind, the resource id, and a structured `details` payload.

The chain is publicly verifiable at `GET /api/audit/verify`. Tampering with any frame breaks the chain and the endpoint returns `{"valid": false, "brokenAt": …}`.

## Content-addressed store

Every compiled manifest is hashed and stored by content, never by name. The CAS lives on disk under `PROMPTSHEON_CAS_PATH` (default `.promptsheon/`) with a sharded layout that keeps any single directory bounded. Garbage collection runs alongside the retention sweeper.

## Identity

Promptsheon supports two authentication mechanisms:

- **Bearer API keys** for human users and SDK callers. The key is sha256-hashed in the `api_keys` table.
- **SPIFFE Verifiable Identity Documents (SVIDs)** for agents. The SVID is an ed25519-signed JWT carrying the agent's id, organisation, and classification. Verification is enforced at the auth middleware against the operator's signing key.

See [Security]({{ '/security/' | relative_url }}) for the full identity contract.

## Where to go next

| You want to… | Read this |
|--------------|-----------|
| Hit the HTTP API | [API reference]({{ '/api-reference/' | relative_url }}) |
| Read the architecture | [Architecture]({{ '/architecture/' | relative_url }}) |
| Walk through a guided build | [Tutorials]({{ '/tutorials/' | relative_url }}) |
