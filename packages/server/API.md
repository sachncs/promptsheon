# REST API Reference

Complete reference for every HTTP endpoint exposed by `@promptsheon/server`.

All endpoints live under `/api/`. Endpoints with `:id` style parameters expect a path segment as documented per route. Body-validated endpoints use Zod schemas from `@promptsheon/shared`; when validation fails the server returns `422 Unprocessable Entity` with `{ error: { code: 'VALIDATION_ERROR', issues: [...] } }`. Not-found responses use `404 Not Found` with `{ error: { code: 'NOT_FOUND', message: '...' } }`.

---

## Health (`routes/health.ts`)

| Method | Path | Purpose | Body | Response |
|--------|------|---------|------|----------|
| `GET` | `/api/health` | Liveness probe; verifies SQLite connectivity | — | `200 { status, db, timestamp }` / `503 { status, db, error, timestamp }` |

---

## Workspaces (`routes/workspace.ts`)

| Method | Path | Purpose | Body | Response |
|--------|------|---------|------|----------|
| `GET` | `/api/workspaces` | List workspaces (paginated) | — | `200 Workspace[]` |
| `GET` | `/api/workspaces/:id` | Fetch one workspace | — | `200 Workspace` / `404` |
| `POST` | `/api/workspaces` | Create a workspace | `CreateWorkspaceSchema` | `201 Workspace` |
| `PUT` | `/api/workspaces/:id` | Update a workspace | `UpdateWorkspaceSchema` | `200 Workspace` |
| `DELETE` | `/api/workspaces/:id` | Delete a workspace | — | `204` |

---

## Projects (`routes/project.ts`)

| Method | Path | Purpose | Body | Response |
|--------|------|---------|------|----------|
| `GET` | `/api/projects` | List projects (paginated, optionally filtered by `workspaceId`) | — | `200 Project[]` |
| `GET` | `/api/projects/:id` | Fetch one project | — | `200 Project` / `404` |
| `POST` | `/api/projects` | Create a project under a workspace | `CreateProjectSchema` | `201 Project` |
| `PUT` | `/api/projects/:id` | Update a project | `UpdateProjectSchema` | `200 Project` |
| `DELETE` | `/api/projects/:id` | Delete a project | — | `204` |

---

## Capabilities (`routes/capability.ts`)

| Method | Path | Purpose | Body | Response |
|--------|------|---------|------|----------|
| `GET` | `/api/capabilities` | List capabilities (paginated, optionally filtered by `projectId`) | — | `200 Capability[]` |
| `GET` | `/api/capabilities/:id` | Fetch one capability | — | `200 Capability` / `404` |
| `POST` | `/api/capabilities` | Create a capability under a project | `CreateCapabilitySchema` | `201 Capability` |
| `PUT` | `/api/capabilities/:id` | Update a capability | `UpdateCapabilitySchema` | `200 Capability` |
| `DELETE` | `/api/capabilities/:id` | Delete a capability | — | `204` |

---

## Capability Versions (`routes/version.ts`)

| Method | Path | Purpose | Body | Response |
|--------|------|---------|------|----------|
| `GET` | `/api/capability-versions` | List versions (paginated, optionally filtered by `capabilityId`) | — | `200 CapabilityVersion[]` |
| `GET` | `/api/capability-versions/:id` | Fetch one version | — | `200 CapabilityVersion` / `404` |
| `POST` | `/api/capability-versions` | Create a new immutable version (`manifest` is a string, `manifestHash` is the CAS hash) | `CreateVersionSchema` (`capabilityId`, `version`, `manifest`, `manifestHash`, `createdBy?`) | `201 CapabilityVersion` |
| `DELETE` | `/api/capability-versions/:id` | Delete a version | — | `204` |

---

## Manifests (`routes/manifest.ts`)

| Method | Path | Purpose | Body | Response |
|--------|------|---------|------|----------|
| `GET` | `/api/manifests/:versionId` | Fetch the parsed manifest object for a version | — | `200 Manifest` (parsed JSON) / `404` |

---

## Releases (`routes/release.ts`)

| Method | Path | Purpose | Body | Response |
|--------|------|---------|------|----------|
| `GET` | `/api/releases` | List releases (paginated, optionally filtered by `capabilityId`) | — | `200 Release[]` |
| `GET` | `/api/releases/:id` | Fetch one release | — | `200 Release` / `404` |
| `POST` | `/api/releases` | Create a release (adds `capabilityVersionId`, `manifest`, `createdBy?` to `CreateReleaseSchema`) | `CreateBodySchema` (`CreateReleaseSchema` + `capabilityVersionId`, `manifest`, `createdBy?`) | `201 Release` |
| `PUT` | `/api/releases/:id/activate` | Transition a release to `active` | — | `200 Release` |
| `PUT` | `/api/releases/:id/supersede` | Transition a release to `superseded` | — | `200 Release` |

---

## Executions (`routes/execution.ts`)

| Method | Path | Purpose | Body | Response |
|--------|------|---------|------|----------|
| `GET` | `/api/executions` | List executions (paginated, optionally filtered by `capabilityVersionId`) | — | `200 Execution[]` |
| `GET` | `/api/executions/:id` | Fetch one execution | — | `200 Execution` / `404` |
| `POST` | `/api/invoke` | Invoke a capability version via the Strands `InvocationAgent`; persists the execution | `InvokeExecutionSchema` (`capabilityVersionId`, `inputs`, `environment?`, `traceId?`) | `200 Execution` |

---

## Datasets (`routes/dataset.ts`)

| Method | Path | Purpose | Body | Response |
|--------|------|---------|------|----------|
| `GET` | `/api/datasets` | List datasets (paginated, optionally filtered by `capabilityId`) | — | `200 Dataset[]` |
| `GET` | `/api/datasets/:id` | Fetch one dataset | — | `200 Dataset` / `404` |
| `POST` | `/api/datasets` | Create a dataset | `CreateDatasetSchema` | `201 Dataset` |
| `DELETE` | `/api/datasets/:id` | Delete a dataset | — | `204` |
| `GET` | `/api/datasets/:id/cases` | List the cases belonging to a dataset | — | `200 DatasetCase[]` |
| `POST` | `/api/datasets/:id/cases` | Append a case to a dataset | `CreateCaseSchema` (`inputs`, `expected`, `description?`) | `201 DatasetCase` |
| `DELETE` | `/api/datasets/:datasetId/cases/:caseId` | Delete a single case | — | `204` |

---

## Eval Runs (`routes/eval.ts`)

| Method | Path | Purpose | Body | Response |
|--------|------|---------|------|----------|
| `GET` | `/api/eval-runs` | List eval runs (paginated, optionally filtered by `releaseId`) | — | `200 EvalRun[]` |
| `GET` | `/api/eval-runs/:id` | Fetch one eval run | — | `200 EvalRun` / `404` |
| `POST` | `/api/eval-runs` | Create an eval run record | `CreateEvalRunSchema` (`releaseId`, `datasetId`, `scorer`) | `201 EvalRun` |
| `GET` | `/api/eval-runs/:id/results` | List the per-case results of an eval run | — | `200 EvalResult[]` |
| `POST` | `/api/eval/run` | Execute an eval run via the Strands `EvaluationAgent`; `getActualUrl` is the endpoint the agent calls to obtain the actual output for each case | `RunEvalSchema` (`evalRunId`, `getActualUrl`) | `200 EvalRun` (updated) / `404` |

---

## Preconditions (`routes/precondition.ts`)

| Method | Path | Purpose | Body | Response |
|--------|------|---------|------|----------|
| `GET` | `/api/preconditions` | List preconditions (optionally filtered by `capabilityId`) | — | `200 Precondition[]` |
| `GET` | `/api/preconditions/:id` | Fetch one precondition | — | `200 Precondition` / `404` |
| `POST` | `/api/preconditions` | Create a precondition (named command hook) | `CreatePreconditionSchema` (`capabilityId`, `name`, `command`, `timeoutSec?`, `enabled?`) | `201 Precondition` |
| `DELETE` | `/api/preconditions/:id` | Delete a precondition | — | `204` |

---

## Alert Rules (`routes/alert.ts`)

| Method | Path | Purpose | Body | Response |
|--------|------|---------|------|----------|
| `GET` | `/api/alert-rules` | List all alert rules | — | `200 AlertRule[]` |
| `GET` | `/api/alert-rules/:id` | Fetch one alert rule | — | `200 AlertRule` / `404` |
| `POST` | `/api/alert-rules` | Create an alert rule | `CreateAlertRuleSchema` | `201 AlertRule` |
| `PUT` | `/api/alert-rules/:id` | Update an alert rule | `UpdateAlertRuleSchema` | `200 AlertRule` |
| `DELETE` | `/api/alert-rules/:id` | Delete an alert rule | — | `204` |
| `GET` | `/api/alerts` | List active alerts (optionally filtered by `status`) | — | `200 Alert[]` |
| `PUT` | `/api/alerts/:id/acknowledge` | Acknowledge an alert (sets `acknowledgedAt`) | — | `200 Alert` |

---

## Schedules (`routes/schedule.ts`)

| Method | Path | Purpose | Body | Response |
|--------|------|---------|------|----------|
| `GET` | `/api/schedules` | List schedules (paginated) | — | `200 Schedule[]` |
| `GET` | `/api/schedules/:id` | Fetch one schedule | — | `200 Schedule` / `404` |
| `POST` | `/api/schedules` | Create a schedule | `CreateScheduleSchema` (`workspaceId`, `releaseId`, `kind`, `cron`, `enabled?`) | `201 Schedule` |
| `PUT` | `/api/schedules/:id` | Update a schedule (`cron`, `enabled`, `nextFireAt`) | `UpdateScheduleSchema` | `200 Schedule` |
| `DELETE` | `/api/schedules/:id` | Delete a schedule | — | `204` |

---

## Settings (`routes/settings.ts`)

| Method | Path | Purpose | Body | Response |
|--------|------|---------|------|----------|
| `GET` | `/api/settings` | List all settings (CRDT-backed) | — | `200 Setting[]` |
| `GET` | `/api/settings/:key` | Fetch one setting | — | `200 { key, value }` / `404` |
| `PUT` | `/api/settings/:key` | Set a setting value | `SetSettingSchema` (`value: unknown`) | `200 { key, value }` |

---

## Server-Sent Events (`routes/sse.ts`)

| Method | Path | Purpose | Body | Response |
|--------|------|---------|------|----------|
| `GET` | `/api/events/:channel` | Subscribe to an SSE channel (`text/event-stream`); events: `log`, `progress`, `status`, `error`, `complete`, `heartbeat`, `alert` | — | `200 text/event-stream` (long-lived) |

---

## Approvals (`routes/approval.ts`)

| Method | Path | Purpose | Body | Response |
|--------|------|---------|------|----------|
| `GET` | `/api/approvals/:releaseId` | Fetch the approval record for a release | — | `200 Approval` / `404` |
| `POST` | `/api/approvals` | Upsert the approval votes for a release | `UpsertApprovalSchema` (`releaseId`, `votes`) | `201 { releaseId, votes }` |

---

## Self-Evolve (`routes/self-evolve.ts`)

| Method | Path | Purpose | Body | Response |
|--------|------|---------|------|----------|
| `POST` | `/api/self-evolve/run` | Trigger one evolution cycle for a capability via the Strands `EvolutionAgent` | `RunCycleSchema` (`capabilityId`) | `200 { action: 'revised' \| 'no_change', state: SelfEvolveState }` / `404` |
| `GET` | `/api/self-evolve/:capabilityId/state` | Fetch the current evolution state for a capability | — | `200 SelfEvolveState` (or `{ status: 'idle', cycleCount: 0 }`) |

---

## Reasoning Compiler (`routes/compiler.ts`)

| Method | Path | Purpose | Body | Response |
|--------|------|---------|------|----------|
| `POST` | `/api/compiler/compile` | Compile a `Manifest` into a `CapabilityPlan` (DAG of capability invocations) via the `ReasoningCompiler`; optional `capabilityContext` and `constraints` steer best-fit selection | `CompileSchema` (`manifest: ManifestSchema`, `capabilityContext?`, `constraints?`) | `200 CapabilityPlan` |
| `POST` | `/api/compiler/decompile` | Reverse a `CapabilityPlan` back into the originating `Manifest` | `DecompileSchema` (`manifest: ManifestSchema`) | `200 { original: CapabilityPlan }` |

---

## Conventions

- **Pagination** — query parameters `page` (default 1) and `pageSize` (default 20, max 100) via `PaginationSchema`.
- **IDs** — UUIDs, validated as `z.string().uuid()` by the relevant schemas.
- **Error envelope** — every error response is `{ error: { code: ErrorCode, message, issues? } }` (`routes/validate.ts:35`).
- **Auth** — when `PROMPTSHEON_AUTH=true`, requests must carry an API key (`Authorization: Bearer <key>` or `X-API-Key`).
- **Content-Type** — `application/json` for request and response bodies; `text/event-stream` for SSE.
