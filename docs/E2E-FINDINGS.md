# E2E test findings

End-to-end walk of every CRUD surface as a real user would, against
a fresh DB. The flow covered: bootstrap, workspaces, projects,
capabilities, versions, datasets, eval suites, releases, the
maker-checker gate, audit, and the auxiliary admin/user/settings
endpoints.

Tests live in `tests/e2e-helpers/e2e.sh` (driver script). The test
was driven by the user-provided `MINIMAX_API_KEY` against
`https://api.minimax.io/anthropic` with model `MiniMax-M3`. The LLM
probe round-trip worked (851ms latency, model echoed back).

## Issues, by severity

### BUG-1 (CRITICAL) — Maker-checker workflow is dead-ended

**Repro:** create a capability → create a version with a manifest →
create a release → POST `/api/manifests/<hash>/approve` (Bob) →
GET `/api/manifests/<hash>/approvals` → 404 "manifest not found".

**Root cause:** the version-create route inserts into
`capability_versions` but never upserts the manifest into
`manifest_dag`. The approval routes query `manifestRepo.findByHash`
which only looks in `manifest_dag`. The hash has no row there, so
`findByHash` returns null → 404.

`packages/server/src/repos/version.ts:23` inserts to
`capability_versions` only. There is no
`manifestRepo.upsertManifestDag(...)` call. Until that's added (or
the approval route is rewritten to look in `capability_versions`),
no release in the system can ever pass the maker-checker gate.

**Symptom in user flow:** click "Activate" on any release → always
`409 APPROVAL_REQUIRED: insufficient approvers (0/2)` no matter
how many approvals the operator adds via the UI.

**Also:** the activation gate's `createdBy` reads from
`existing.createdBy` (camelCase) but the DB returns snake_case
columns, so it was always `undefined` → the gate was effectively
no-op (already fixed in a previous round via
`/Users/sachin/repo/promptsheon/packages/server/src/routes/release.ts:289-293`).

**Fix:** in `create()` of `VersionRepo`, also call
`manifestRepo.create(manifest, { goal, createdBy })` (or split into a
shared `registerManifest` helper) so `manifest_dag` has the row.

### BUG-2 (HIGH) — `custom` LLM provider rejected by status check

**Repro:** save LLM with `provider: "custom", baseUrl, apiKey, model`
→ returns `200 { ok: true }`. GET `/api/bootstrap/status` →
`needsLlm: true, provider: "custom"`. The bootstrap wizard can never
complete with the custom provider.

**Root cause:** `packages/server/src/routes/bootstrap.ts:202`
`resolveKeyPresence(provider, resolver)` has no `provider === 'custom'`
branch — it falls through and returns `false`. The save endpoint
correctly stores `llm.customApiKey` (line 176), but the status
endpoint never looks at it.

**Fix:** add the custom branch in `resolveKeyPresence`:
```ts
if (provider === 'custom') {
  const v = await resolver.get<string>('llm.customApiKey');
  return Boolean(v) || Boolean(process.env['LLM_CUSTOM_KEY']);
}
```

### BUG-3 (MEDIUM) — Newly-created users aren't added to the active org

**Repro:** POST `/api/users` (with any email) → 201, user row
inserted. Subsequent call to `/api/users` from the new user's
context returns `403 NOT_ORG_MEMBER`.

**Root cause:** `POST /api/users` only inserts a `users` row; it
does not insert a corresponding `org_members` row. Compare to the
bootstrap `/api/bootstrap/admin` route which does both. The admin
endpoints assume the operator is in some org; non-bootstrap user
creation is half-wired.

**Fix:** the user-create route should accept `orgId` and insert
into `org_members`; the `MEMBER_OF_ORG` middleware should be
documented as requiring `org_members` membership.

### BUG-4 (LOW) — GET `/api/audit` returns empty body with 200

**Repro:** after creating 5 entities, GET `/api/audit` →
`status: 200` and an empty string body. Expected: a list of audit
rows. The `/api/audit/verify` endpoint returns 404.

**Root cause:** unknown. Possibly the audit chain doesn't flush
rows to the table the GET queries, or the route is misnamed.
README documents `GET /api/audit` and `POST /api/audit/verify`;
neither matches what's in `src/routes/audit.ts` (likely).

**Fix:** check the audit route file; reconcile route paths with the
README.

### BUG-5 (LOW) — Workspace not linked to org on create

**Repro:** POST `/api/workspaces` → 201, body returns
`{ id, name, organization, created_at, ... }`. GET
`/api/workspaces` shows the row with `org_id: ""` (empty string).

**Root cause:** `WorkspaceRepo.create` does not insert into
`org_workspaces` (or whatever the join table is). The workspace
exists but isn't associated with the requesting org.

**Fix:** add the org↔workspace join on create.

### BUG-6 (LOW) — Release's `createdBy` is empty

**Repro:** POST `/api/releases` → 201, body returns
`{ createdBy: "" }` even though the requesting user's
`X-User-Id` is set. The audit chain still attributes the action to
the actor via `actorOf(request)`, so this is an inconsistency in
the release record, not in the audit log.

**Root cause:** `ReleaseRepo.create` accepts `createdBy?` as
optional; the route passes nothing, and the SQL `INSERT` falls
through to `''`.

**Fix:** in the release-create route, set
`data.createdBy = actorOf(request)`.

### BUG-7 (LOW) — Routes documented in README that don't exist

| README | Reality |
|---|---|
| `GET /api/webhooks` | returns 404. Only the create/list endpoint shape exists. |
| `GET /api/admin/cost` | returns 404. The frontend route at `/app/admin/cost` calls it. |
| `POST /api/audit/verify` | returns 404. The README cites it as the chain integrity check. |
| `GET /api/approvals?releaseId=…` | returns 404. The frontend uses this; the backend exposes only `/api/manifests/:hash/approvals`. |
| `POST /api/releases/:id/approvals` | returns 404. The frontend posts here. |
| `GET /api/approvals/:releaseId` | returns 404. Same mismatch. |

**Fix:** the backend route file uses one vocabulary
(`/api/manifests/:hash/…`); the frontend uses another
(`/api/approvals/:releaseId`, `/api/releases/:id/approvals`). Reconcile
both. Easiest path: add a small wrapper in `approval.ts` that
translates `releaseId` → `hash` (via the release row) and forwards to
the existing `manifest-approval.ts` routes.

### BUG-8 (DOC) — Test script bug, not a product bug

`tests/e2e-helpers/e2e.sh` uses `UID=` which is a bash readonly
reserved variable (the shell UID). It silently fails to assign
inside the heredoc. Rename to `ADMIN_ID` to avoid the conflict.

## What worked correctly (smoke)

- LLM probe round-trip against `https://api.minimax.io/anthropic`
  with model `MiniMax-M3` returned `latencyMs: 851` and the model
  name echoed back correctly.
- Onboarding flow: bootstrap status, create admin, validate LLM,
  save LLM all returned 200/201.
- Workspace create / list
- Project create / list under a workspace
- Capability create / list under a project
- Capability-version create with manifest + manifestHash
- Dataset create + add case
- Eval-suite create
- Release create (in draft state)
- Maker-checker gate fires (returns 409) when no approvals
- Alert rules list, alerts list (empty arrays)
- API keys list (empty array)
- Goals list (empty array)
- Settings list (returns defaults)

## Suggested commit order for fixes

1. **BUG-1** (version create → also insert manifest into
   `manifest_dag`). Without this, no release can ever activate.
2. **BUG-2** (custom provider in `resolveKeyPresence`). Without
   this, the custom provider path in onboarding can never complete.
3. **BUG-7** (route reconciliation for approvals / audit / webhooks
   / cost). Without these, the frontend hits 404s on the relevant
   pages.
4. **BUG-3** (user→org membership on create). Without this, you
   can't add a second user.
5. **BUG-5** (workspace→org on create). Without this, workspaces
   float unassociated.
6. **BUG-6** (release createdBy).
7. **BUG-4** (audit/verify) — investigate the actual audit route
   before fixing.
8. **BUG-8** — test script rename. Trivial.
