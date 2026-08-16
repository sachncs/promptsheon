# SDK

Promptsheon ships one SDK: **Go**. The Python and TypeScript
SDK directories (`sdk/python/`, `sdk/typescript/`) were
removed in v1.0.0 — they had contained only a copy of the
OpenAPI spec and no actual client code. A future generator
pass can re-introduce them; when it does, the parity gate
from PR-5 in `docs/research/audit-fixes-plan.md` will
mechanically catch any drift between the spec and the
generated SDK.

The Go SDK lives at `pkg/promptsheon` and is gated by
`//go:build promptsheon` so the internal types stay
package-private. The legacy `github.com/sachncs/promptsheon/sdk`
import path was removed in v1.0.0; consumers must update
to `github.com/sachncs/promptsheon/pkg/promptsheon`.

## Go SDK

```go
import "github.com/sachncs/promptsheon/pkg/promptsheon"

client := promptsheon.New("http://127.0.0.1:8080", "ps_...")
ctx := context.Background()
```

### Lifecycle

| Method | HTTP | Returns |
|--------|------|---------|
| `client.Health(ctx)` | `GET /health` | `*HealthResponse` |
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

The full surface is generated from `promptsheon/spec/spec.yaml`; see
the contract test (`tests/contract/contract_test.go`) for the
mechanical list of `*promptsheon.Client` methods the parity gate
enforces.

## Adding a new SDK method

1. Add the method to `pkg/promptsheon/client.go`.
2. Add the route to the OpenAPI spec via `make openapi` if it
   is a new endpoint.
3. Run `go test -count=1 ./tests/contract/...` to confirm the
   contract test still passes (the parity gate walks the spec and
   asserts every operationId has a corresponding Client method).

## Contract test

`tests/contract/contract_test.go` is the gate that catches drift
between `promptsheon/spec/spec.yaml` and the Go SDK. The test
parses the spec, walks every registered route, and asserts the
documented SDK surface. It is wired into CI as a step on the
default `test` job (see `.github/workflows/ci.yaml`).
