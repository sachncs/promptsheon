---
layout: page
title: Recipes
subtitle: Copy-pasteable snippets for common tasks.
---

# Recipes

Each recipe is a single, self-contained snippet you can paste into your code or shell. Every recipe assumes you've already run `pnpm install` and have a Bearer token from `POST /api/bootstrap/admin` (or via the UI's onboarding wizard).

## Mint a Bearer API key

```bash
curl -X POST http://localhost:8080/api/api-keys \
  -H "Content-Type: application/json" \
  -H "X-User-Id: <admin-user-id>" \
  -H "X-Org-Id: <admin-org-id>" \
  -d '{"role": "admin"}'
# {"id": "...", "token": "<bearer-token>", "role": "admin"}
```

Store the returned `token` in your secrets manager. The server never stores the cleartext — only the sha256 hash.

## Use a custom OpenAI-compatible endpoint

Set the provider during onboarding:

```bash
curl -X POST http://localhost:8080/api/bootstrap/admin \
  -H "Content-Type: application/json" \
  -d '{
    "userName": "Alice",
    "userEmail": "alice@example.com",
    "orgName": "Acme",
    "llm": {
      "provider": "custom",
      "baseUrl": "https://llm.example.com",
      "apiKey": "<your-key>",
      "model": "gpt-4"
    }
  }'
```

The settings store persists the credentials per organisation.

## Execute a capability release

```bash
curl -X POST http://localhost:8080/api/executions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "releaseId": "rel_abc123",
    "inputs": { "question": "How do I reset my password?" }
  }'
# {"executionId": "e_123", "output": "…", "tokens": 1234}
```

## Stream an execution over SSE

```bash
curl -N -X POST http://localhost:8080/api/executions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -H "Accept: text/event-stream" \
  -d '{
    "releaseId": "rel_abc123",
    "inputs": { "question": "Summarise this…" }
  }'
```

The server emits `execution_start`, `node_start`, `node_complete`, `execution_complete`, and a terminal `done` frame.

## Approve a release

```bash
curl -X POST http://localhost:8080/api/releases/<id>/approvals \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"reason": "Reviewed by SRE"}'
# {"id": "...", "releaseId": "...", "userId": "...", "reason": "...", "at": "..."}
```

A release cannot self-approve. The maker-checker gate enforces two distinct non-creator approvals before activation.

## Roll back a release

```bash
curl -X POST http://localhost:8080/api/releases/<id>/rollback \
  -H "Authorization: Bearer <token>"
# {"id": "...", "state": "rolled_back", "rolledBackFrom": "active"}
```

The release transitions to `rolled_back`. Every subsequent `POST /api/executions` call for that capability routes through the previous active release.

## Verify the audit chain

```bash
curl http://localhost:8080/api/audit/verify
# {"valid": true, "frameCount": 1024, "lastHash": "..."}
```

If the chain has been tampered with, the response includes `"brokenAt": <index>` and `valid` is `false`. The verification endpoint is publicly callable.

## Use the TypeScript SDK

```typescript
import { PromptsheonClient } from '@promptsheon/sdk';

const client = new PromptsheonClient({
  baseUrl: 'http://localhost:8080',
  apiKey: process.env.PROMPTSHEON_API_KEY!,
});

const ws = await client.workspaces.create({ name: 'refund-triage' });
const cap = await client.capabilities.list(ws.id);
const release = await client.releases.activate(cap.releases[0].id);
// → 409 APPROVAL_REQUIRED until 2 distinct non-creator approvals
```

## Use the CLI

```bash
PROMPTSHEON_API_KEY=<token> pnpm --filter @promptsheon/cli login --url http://localhost:8080
pnpm --filter @promptsheon/cli repos list
pnpm --filter @promptsheon/cli release approve <release-id>
pnpm --filter @promptsheon/cli manifest scan <hash>
```

Every command accepts `--json` for raw output and mutating commands accept `--dry-run` to print the would-be request without firing it.

## Mint and verify an SVID

```bash
# Mint (operator-side)
pnpm --filter @promptsheon/server identity:svid:mint \
  --sub agent-abc \
  --org <org-id> \
  --cls internal \
  --ttl 3600

# Verify (server-side, on the request path)
curl -H "Authorization: SVID <token>" http://localhost:8080/api/executions/<id>
```

SVIDs are ed25519-signed JWTs. The server verifies against `PROMPTSHEON_SVID_PUBLIC_KEY_PEM`. A successful verification stamps the request with an `Agent` principal for the Cedar gate.

## Where to go next

| You want to… | Read this |
|--------------|-----------|
| Solve a specific problem | [FAQ]({{ '/faq/' | relative_url }}) |
| Hit the HTTP API | [API reference]({{ '/api-reference/' | relative_url }}) |
| Read the architecture | [Architecture]({{ '/architecture/' | relative_url }}) |
