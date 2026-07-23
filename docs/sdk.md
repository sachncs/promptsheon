# SDKs

Three SDKs ship with Promptsheon: **Go**, **Python**, and
**TypeScript**. Each is generated from `api/openapi.yaml` via
the `sdk/{lang}/scripts/codegen.sh` helper, so the surface
stays in lockstep with the daemon.

The Go SDK lives at `sdk/` and is hand-extended for ergonomic
features (e.g. `ApproveAndInvoke`); the Python and
TypeScript SDKs are pure codegen.

## Go SDK

```go
import "github.com/sachncs/promptsheon/sdk"

client := sdk.New("http://localhost:8080", "ps_...")
ctx := context.Background()
```

### Lifecycle

| Method | HTTP | Returns |
|--------|------|---------|
| `client.Health(ctx)` | `GET /health` | `*HealthResponse` |
| `client.ListProviders(ctx)` | `GET /api/v1/providers` | `[]string` |
| `client.CreateWorkspace(ctx, name)` | `POST /api/v1/workspaces` | `*Workspace` |
| `client.CreateCapability(ctx, projectID, req)` | `POST /api/v1/projects/{id}/capabilities` | `*Capability` |
| `client.AddVersion(ctx, capabilityID, req)` | `POST /api/v1/capabilities/{id}/versions` | `*Version` |
| `client.CreateRelease(ctx, versionID, req)` | `POST /api/v1/versions/{id}/releases` | `*Release` |
| `client.GetRelease(ctx, id)` | `GET /api/v1/releases/{id}` | `*Release` |
| `client.ListReleases(ctx, capabilityID)` | `GET /api/v1/capabilities/{id}/releases` | `[]*Release` |
| `client.Vote(ctx, releaseID, req)` | `POST /api/v1/releases/{id}/votes` | `*Approval` |
| `client.Activate(ctx, releaseID)` | `POST /api/v1/releases/{id}/activate` | `*Release` |
| `client.Rollback(ctx, releaseID)` | `POST /api/v1/releases/{id}/rollback` | `*Release` |
| `client.Invoke(ctx, releaseID, req)` | `POST /api/v1/releases/{id}/invoke` | `*Execution` |
| `client.Approval(ctx, releaseID)` | `GET /api/v1/releases/{id}/approval` | `*Approval` |
| `client.ApproveAndInvoke(ctx, releaseID, voterIdentity, req)` | (combo) | `*Execution` |

### API keys

| Method | HTTP | Returns |
|--------|------|---------|
| `client.CreateAPIKey(ctx, req)` | `POST /api/v1/apikeys` | `*APIKey` (with `Key` field) |
| `client.ListAPIKeys(ctx, userID)` | `GET /api/v1/apikeys?user_id=...` | `[]*APIKey` |
| `client.RevokeAPIKey(ctx, id)` | `DELETE /api/v1/apikeys/{id}` | error |
| `client.OAuthLoginURL(provider)` | (URL builder) | `string` |

### Harness

| Method | HTTP | Returns |
|--------|------|---------|
| `client.CreateDataset(ctx, capabilityID, req)` | `POST /api/v1/capabilities/{id}/datasets` | `*Dataset` |
| `client.ListDatasets(ctx, capabilityID)` | `GET /api/v1/capabilities/{id}/datasets` | `[]*Dataset` |
| `client.GetDataset(ctx, id)` | `GET /api/v1/datasets/{id}` | `*DatasetWithCases` |
| `client.PutCases(ctx, id, cases)` | `PUT /api/v1/datasets/{id}/cases` | error |
| `client.DeleteDataset(ctx, id)` | `DELETE /api/v1/datasets/{id}` | error |
| `client.CreatePrecondition(ctx, capabilityID, req)` | `POST /api/v1/capabilities/{id}/preconditions` | `*Precondition` |
| `client.ListPreconditions(ctx, capabilityID)` | `GET /api/v1/capabilities/{id}/preconditions` | `[]*Precondition` |
| `client.UpdatePrecondition(ctx, id, req)` | `PUT /api/v1/preconditions/{id}` | `*Precondition` |
| `client.DeletePrecondition(ctx, id)` | `DELETE /api/v1/preconditions/{id}` | error |
| `client.RunEval(ctx, releaseID, req)` | `POST /api/v1/releases/{id}/evals` | `*EvalRun` |
| `client.ListEvals(ctx, releaseID)` | `GET /api/v1/releases/{id}/evals` | `[]*EvalRun` |
| `client.GetEval(ctx, id)` | `GET /api/v1/evals/{id}` | `*EvalRunWithResults` |

## Python SDK

```python
from promptsheon import Client

client = Client(base_url="http://localhost:8080", api_key="ps_...")
```

Same method surface as the Go SDK. Generated via
[`openapi-python-client`](https://github.com/openapi-generators/openapi-python-client)
from `api/openapi.yaml`. Regenerate via:

```bash
bash sdk/python/scripts/codegen.sh
```

## TypeScript SDK

```typescript
import { PromptsheonClient } from "@sachncs/promptsheon-sdk";

const client = new PromptsheonClient({
  baseUrl: "http://localhost:8080",
  apiKey: "ps_...",
});
```

Generated via
[`openapi-typescript`](https://openapi-ts.dev) from
`api/openapi.yaml`. Regenerate via:

```bash
bash sdk/typescript/scripts/codegen.sh
```

## Adding a new SDK method

1. Add the method to `sdk/client.go`.
2. Add the method name to `sdkMandatoryMethods` in
   `tests/contract/contract_test.go` so the contract test
   catches accidental deletions.
3. If the new method hits an endpoint that wasn't previously
   covered by the SDK, add the endpoint to the OpenAPI spec
   via `make openapi` first.
4. Run `bash sdk/python/scripts/codegen.sh` and
   `bash sdk/typescript/scripts/codegen.sh` to regenerate
   the Python and TypeScript SDKs.

## Contract test

`tests/contract/contract_test.go` is the gate that catches
drift between `api/openapi.yaml` and the Go SDK. The test
parses the spec, walks every registered route, and asserts
the documented SDK surface. It's wired into CI as a step
on the default `test` job.