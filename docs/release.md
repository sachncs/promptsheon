# Releases: the Capability → Release lifecycle

A **Release** is the approved pointer from a Capability Version to
a target Environment. The lifecycle is:

```
Capability Version (immutable)
    │
    │  POST /api/v1/versions/{version_id}/releases
    ▼
Pending Release
    │
    │  POST /api/v1/releases/{release_id}/votes  (one or more)
    ▼
Approval (votes recorded)
    │
    │  POST /api/v1/releases/{release_id}/activate  (Policy consulted)
    ▼
Active Release
    │
    │  POST /api/v1/releases/{release_id}/invoke
    ▼
Execution row recorded with Outputs, CostUSD, LatencyMs
```

## Environment model

There is exactly **one Active Release per (Capability, Environment)**
at any time. Activating a new Release in the same Environment
supersedes the prior Active Release atomically: both writes
commit in one transaction (see
`internal/store/sqlite_releases.go::ActivateAtomic`), so a
mid-flight failure cannot leave two Active Releases.

Environments are a closed set: `dev`, `staging`, `prod`. Adding
environments requires a code change.

## Approval policy

The daemon defaults to **MakerCheckerPolicy** (the creator cannot
approve their own release; at least one other identity must).
Override via `PROMPTSHEON_APPROVAL_POLICY=majority` for a flat
count-based policy.

Both policies honour **separation of duties**: a vote from
`release.CreatedBy` is rejected at activate time regardless of
the configured policy.

## Walkthrough (curl)

```bash
# 1. Create a workspace, project, capability, version (omitted; see README).

# 2. Create a Pending Release pointing the version at the prod env.
REL=$(curl -sS -X POST http://localhost:8080/api/v1/versions/$VID/releases \
        -H "Content-Type: application/json" \
        -d '{"environment":"prod"}' | jq -r .id)

# 3. A non-creator identity casts an Approve vote.
curl -sS -X POST http://localhost:8080/api/v1/releases/$REL/votes \
     -H "Content-Type: application/json" \
     -d '{"identity":"alice","decision":"approve"}'

# 4. Activate (consults the policy; 409 if quorum not satisfied or creator voted).
curl -sS -X POST http://localhost:8080/api/v1/releases/$REL/activate

# 5. Invoke; the Execution row records outputs, tokens, cost, latency.
curl -sS -X POST http://localhost:8080/api/v1/releases/$REL/invoke \
     -H "Content-Type: application/json" \
     -d '{"inputs":{"q":"hello"},"model":"claude-opus-4"}'
```

## Walkthrough (Go SDK)

```go
rel, _ := client.CreateRelease(ctx, "v1", sdk.CreateReleaseRequest{Environment: "prod"})
client.Vote(ctx, rel.ID, sdk.VoteRequest{Identity: "alice", Decision: "approve"})
client.Activate(ctx, rel.ID)
out, _ := client.Invoke(ctx, rel.ID, sdk.InvokeRequest{
    Inputs: map[string]any{"q": "hello"},
    Model:  "claude-opus-4",
})
fmt.Println(out.Outputs["content"])
```

The `ApproveAndInvoke(ctx, rel.ID, voter, req)` helper folds steps
3-5 into one call. The voter identity must differ from
`rel.CreatedBy` for MakerChecker.

## Rollback

`POST /api/v1/releases/{id}/rollback` transitions an Active (or
Approved) Release to `rolled_back`. The supersede chain is preserved;
subsequent activations in the same Environment continue from a
clean slate.

## Audit trail

Every state change writes an audit row keyed `release:{id}`. Vote,
activate, rollback, and invoke each get a separate action with the
relevant identity and outcome details. Use
`GET /api/v1/audit?resource=release:{id}` to reconstruct the history.
