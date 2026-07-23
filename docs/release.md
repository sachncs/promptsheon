# Release Lifecycle

A **Release** is the binding from a Version to a tenant
Environment. It's the unit operators promote, canary,
approve, and roll back. The lifecycle is:

```
                    create
   (none) ───────────────────────▶ pending
                                       │
                          vote (Approve)        │
                                       ▼
                                    pending  ──── rollback ──▶ rolled_back
                                       │                            ▲
                          vote (Reject)                              │
                                       ▼                            │
                                    rejected                        │
                                                                        │
                          activate  (MakerChecker quorum + preconditions)
                                       │                            │
                                       ▼                            │
                                     active ────── rollback ────────┘
```

The full state machine:

| From | To | Trigger |
|------|----|---------|
| (none) | `pending` | `Create` (POST `/api/v1/versions/{id}/releases`). |
| `pending` | `pending` | `Vote` (POST `/api/v1/releases/{id}/votes`). Updates the approval trail. |
| `pending` | `active` | `Activate` (POST `/api/v1/releases/{id}/activate`). MakerChecker quorum required. Preconditions run. |
| `pending` | `rejected` | `Activate` with a `Reject` vote majority (MajorityPolicy). |
| `active` | `rolled_back` | `Rollback` (POST `/api/v1/releases/{id}/rollback`). |
| `active` | `superseded` | A new Release in the same Environment is Activated. |

A `superseded` Release keeps its history; the audit chain
records the supersession. Operators can still `Get` and
`Approval` a superseded Release for audit purposes.

## Approval policies

Two policies ship in v0.1.x. Both are fail-closed: an
empty or invalid configuration returns an error from
`Evaluate`, never silently approves.

### `MakerCheckerPolicy` (default)

```go
approval.MakerCheckerPolicy{
    RequiredApprovers: 1,
    Creator:           "alice",
}
```

- The creator's vote is rejected with `ErrCreatorVoted`.
- At least `RequiredApprovers` non-creator identities must
  cast an `Approve` vote.
- A `Reject` vote from any identity sends the state to
  `rejected`.

### `MajorityPolicy`

```go
approval.MajorityPolicy{
    Required: 2,
}
```

- At least `Required` identities must cast an `Approve` vote
  (regardless of creator).
- A `Reject` vote from any identity sends the state to
  `rejected`.

Configure via `PROMPTSHEON_APPROVAL_POLICY` and per-policy
constructor options at `cmd/promptsheond/main.go`.

## Activate path

`release.Service.Activate(ctx, releaseID)`:

1. Load the Release; 404 if not found.
2. If `status != pending`, return 409 "release is not active".
3. Run the MakerChecker policy against the vote trail.
   - Insufficient Approves → 409 with the policy's failure
     message.
4. Run the precondition runner against the Capability's
   enabled preconditions.
   - Any failure → 409 with the precondition failures.
5. Supersede the prior Active Release in the same
   Environment (if any).
6. Atomically: write the new Active Release + mark the
   prior Active as `superseded`.
7. Audit: `{"action": "activate", "release_id": ..., "supersedes": ...}`.

## Rollback

`release.Service.Rollback(ctx, releaseID)`:

1. Load the Release; 404 if not found.
2. If `status != active`, return 409.
3. Set status `rolled_back`, `superseded_at = now`.
4. Audit: `{"action": "rollback", "release_id": ...}`.

A rolled-back Release is terminal. Operators create a new
Release pointing at a different Version (or a fixed
Version) and Activate it.

## Approval trail

`GET /api/v1/releases/{id}/approval` returns the full vote
trail:

```json
{
  "release_id": "rel-1",
  "votes": [
    {"identity": "alice", "decision": "approve", "timestamp": "..."},
    {"identity": "bob", "decision": "approve", "timestamp": "..."}
  ],
  "updated_at": "..."
}
```

The trail is append-only and tied to the audit chain via
the `apikey_vote` / `release_approve` / `release_reject`
actions. `VerifyAuditChain` doesn't explicitly walk the
trail (it's an aggregate, not a per-vote chain), but the
release's `approval` JSON lives in the same DB so the
audit verify implicitly covers it.

## Go SDK

```go
client := sdk.New("http://localhost:8080", "ps_...")

// Create + vote + activate + invoke in one helper.
exec, err := client.ApproveAndInvoke(
    ctx, relID, "bob",
    sdk.InvokeRequest{Inputs: map[string]any{"q": "hello"}},
)
```

`ApproveAndInvoke` is the convenience for "vote as a
non-creator identity, activate, and invoke in one
round-trip". It exists because the round-trip is the
commonest path; production tenants that need a separate
audit boundary (e.g. an out-of-band vote) call the individual
methods.

## CLI

```bash
promptsheon release create v1 '{"environment":"prod"}'
promptsheon release vote <id> bob approve
promptsheon release activate <id>      # 409 if quorum / preconditions fail
promptsheon release invoke <id> '{"q":"hello"}'
promptsheon release approval <id>     # show the vote trail
promptsheon release rollback <id>     # active → rolled_back
```

## See also

- [docs/getting-started.md](getting-started.md) — the
  walkthrough that exercises the lifecycle end-to-end.
- [docs/security.md](security.md) — the audit chain
  contract.
- [docs/architecture.md](architecture.md) — the Release
  aggregate's role in the system.