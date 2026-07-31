# PHASE-0 — Critical Defect Patches

**54 commits.** The first PR to execute. Establishes the foundation before any
rename or refactor work.

## Pre-PR-0 manual checklist (read-only)

```bash
go list -m -mod=mod all 2>/dev/null | grep -v promptsheon | grep -v _test
grep -rn "Repositories\b\|BypassSSRF\|activeOAuthStates" --include="*.go" .
```

Both commands should return zero matches.

## Fixes (35 commits)

```
c0.0  chore(audit): no external consumers of release.Resolver
      Refs: PLAN-49/C-9
c0.1  chore(lint): add .golangci.yml with gofmt + govet + errcheck enabled
      Refs: PLAN-49/H-7 L-7
c0.2  fix(idempotency): Header() returns ResponseWriter.Header()
      Refs: PLAN-49/DEF-1
c0.3  fix(oauth): daemon.go:814 backend.github.com/user → api.github.com/user
      Refs: PLAN-49/DEF-2 C-4
c0.4  fix(pagination): rel="first" emitted on page 1
      Refs: PLAN-49/DEF-20
c0.5  fix(alerting): empty severity rejected unconditionally
      Refs: PLAN-49/DEF-18
c0.6  fix(auth): OAuth cookie Secure conditional on TLS
      Refs: PLAN-49/DEF-10
c0.7  fix(handlers): remove TrimPrefix fallback in handleRevokeAPIKey
      Refs: PLAN-49/H-1
c0.8  fix(handlers): translateDBError in contract handlers
      Refs: PLAN-49/MED-9
c0.9  fix(handlers): GetExecution workspace-scoped
      Refs: PLAN-49/2.4
c0.10 fix(audit): mustUnmarshal returns wrapped error
      Refs: PLAN-49/CRIT-1 C-1
c0.11 fix(audit): bound AppendAudit CAS retries
      Refs: PLAN-49/CRIT-4 C-1
c0.12 fix(audit): audit_chain_state immutability triggers
      Refs: PLAN-49/CRIT-5 C-1
c0.13 fix(audit): audit() deep-copies details map
      Refs: PLAN-49/HIGH-4 C-1
c0.14 fix(webhook): make ValidateURL a method, drop BypassSSRF global
      Refs: PLAN-49/CRIT-6 1.3
c0.15 fix(ratelimit): ConfigureTrustedProxies union + log invalid CIDRs
      Refs: PLAN-49/1.6
c0.16 fix(oauth): RegisterProvider SSRF-validates URLs
      Refs: PLAN-49/CRIT-3 1.4 C-4
c0.17 fix(pprof): loopback enforcement on pprofAddr
      Refs: PLAN-49/1.9 C-1
c0.18 fix(handlers): expose resource_kind/resource_id on ListAudit
      Refs: PLAN-49/MED-2 DEF-22
c0.19 fix(idempotency): recordingResponseWriter does not auto-stamp status
      Refs: PLAN-49/DEF-11 C-1
c0.20 fix(audit): evalRunner.Run streams via per-result CreateEvalResult
      Refs: PLAN-49/1.7 C-1
c0.21 fix(audit): StartAuditWorkers rejects second-call
      Refs: PLAN-49/1.8 C-1
c0.22 fix(store): honor audit_chain_state for verify even with cache miss
      Refs: PLAN-49/2.2
c0.23 fix(scripts): drop dead BenchmarkSelect baseline
      Refs: PLAN-49/M10 X-6
c0.24 chore(ci): actions/setup-go respects go.mod version (drop 1.26.5 pin)
      Refs: PLAN-49/H-1
c0.25 chore(docs): build paths in README, docs/development/*, tests/smoke
      Refs: PLAN-49/OS-1
c0.26 chore(make): git rm --cached frontend/dist/index.html + settings-*.js
      Refs: PLAN-49/OS-2 C-1
c0.27 fix(handlers): audit field APIKey renames wire-key "key" → "api_key" (dual-write)
      Refs: PLAN-49/P0-5 P1-7
c0.28 fix(store): UpdateUser revokes per-key with audit row each
      Refs: PLAN-49/HIGH-9
c0.29 fix(cofc): CODE_OF_CONDUCT.md Covenant v2.0 → v2.1
      Refs: PLAN-49/X-1
c0.30 fix(scripts): clean target removes cmd/promptsheond/frontend/
      Refs: PLAN-49/X-10
c0.31 fix(scripts): remove check-domain-purity.sh's forbidden backend/api entry
      Refs: PLAN-49/M9
c0.32 fix(helm): values.schema.json replicaCount maximum=1 for SQLite
      Refs: PLAN-49/OS-15
c0.33 fix(sdk-python): drop dead ssl import from client.py
      Refs: PLAN-49/X-4
c0.34 fix(handlers): classifyInvokeError covers all known sentinels
      Refs: PLAN-49/L-2
```

## Tests (19 commits, each lands same PR)

```
c0.t1  test(idempotency): preserves headers on replay
       Refs: PLAN-49/DEF-1
c0.t2  test(idempotency): preserves status on 5xx replay
       Refs: PLAN-49/DEF-11
c0.t3  test(oauth): RegisterProvider rejects loopback/private/metadata
       Refs: PLAN-49/CRIT-3 DEF-4
c0.t4  test(oauth): state cookie Secure conditional on TLS
       Refs: PLAN-49/DEF-10
c0.t5  test(audit): mustUnmarshal malformed JSON returns typed error
       Refs: PLAN-49/CRIT-1
c0.t6  test(audit): AppendAudit stops after max retries
       Refs: PLAN-49/CRIT-4
c0.t7  test(audit): audit_chain_state UPDATE/DELETE blocked
       Refs: PLAN-49/CRIT-5
c0.t8  test(audit): audit() does not mutate caller map
       Refs: PLAN-49/HIGH-4
c0.t9  test(webhook): no global BypassSSRF leak across dispatchers
       Refs: PLAN-49/CRIT-6 1.3
c0.t10 test(ratelimit): union of CIDRs preserved
       Refs: PLAN-49/1.6
c0.t11 test(pagination): first link on page 1
       Refs: PLAN-49/DEF-20
c0.t12 test(alerting): empty severity rejected
       Refs: PLAN-49/DEF-18
c0.t13 test(handlers): contract not-found returns 404
       Refs: PLAN-49/MED-9
c0.t14 test(handlers): GetExecution cross-workspace rejected
       Refs: PLAN-49/2.4
c0.t15 test(handlers): ListAudit kind/id filter works
       Refs: PLAN-49/MED-2 DEF-22
c0.t16 test(eval): streaming at 10k cases bounded memory
       Refs: PLAN-49/1.7
c0.t17 test(audit): StartAuditWorkers second-call rejected
       Refs: PLAN-49/1.8
c0.t18 test(pprof): non-loopback pprofAddr rejected
       Refs: PLAN-49/1.9
c0.t19 test(oauth): GitHub OAuth URL is api.github.com/user
       Refs: PLAN-49/DEF-2
```

## Files touched

| File | Commits |
|---|---|
| `backend/idempotency.go` | c0.2, c0.19, c0.t1, c0.t2 |
| `backend/audit_workers.go` | c0.13, c0.20, c0.21, c0.t5-t0.t8, c0.t16, c0.t17 |
| `backend/store/sqlite.go` | c0.10, c0.11, c0.22, c0.t5, c0.t6 |
| `backend/store/sqlite_audit.go` | c0.11, c0.t6 |
| `backend/store/migrations/00X_*.up.sql` | c0.12, c0.t7 |
| `backend/store/sqlite_users.go` | c0.28, c0.t19 (impl) |
| `backend/webhook/webhook.go` | c0.14, c0.t9 |
| `backend/ratelimit/ratelimit.go` | c0.15, c0.t10 |
| `backend/auth/oauth.go` | c0.16, c0.t3, c0.t19 |
| `cmd/promptsheond/daemon.go` | c0.3, c0.17 |
| `backend/handlers_auth.go` | c0.6, c0.7, c0.t4 |
| `backend/handlers_alerting.go` | c0.5, c0.t12 |
| `backend/handlers_contract.go` | c0.8, c0.t13 |
| `backend/handlers_executions.go` | c0.9, c0.34, c0.t14 |
| `backend/handlers_audit.go` | c0.18, c0.27, c0.t15 |
| `backend/pagination.go` | c0.4, c0.t11 |
| `backend/audit/*.go` (new for c0.18 split) | c0.18 |
| `Makefile` | c0.26, c0.30 |
| `Dockerfile` | – |
| `.github/workflows/*.yaml` | c0.24, c0.25 |
| `tests/smoke/run.sh` | c0.25 |
| `README.md` | c0.25 |
| `docs/development/*.md` | c0.25 |
| `scripts/bench-baseline.txt` | c0.23 |
| `scripts/check-domain-purity.sh` | c0.31 |
| `CODE_OF_CONDUCT.md` | c0.29 |
| `deploy/helm/promptsheon/values.schema.json` | c0.32 |
| `sdk/python/src/promptsheon/client.py` | c0.33 |
| `.golangci.yml` (new) | c0.1 |

## Exit criterion

- All 22 confirmed DEFs fixed
- All 19 new test commits land in the same PR
- `go build ./...` clean
- `go vet ./...` clean
- `go test -race -count=1 ./...` passes
- `golangci-lint run` passes
- No `git diff` noise on `vendor/modules.txt` (will be regenerated in PR-2)

## Parallelization

5 agents can run in parallel, each owning a disjoint file slice:

| Agent | Files |
|---|---|
| 0A | idempotency, auth handlers, oauth handlers |
| 0B | audit chain, store, migrations |
| 0C | webhook, ratelimit, alerting, daemon |
| 0D | exec handlers, audit handlers, pagination, users store |
| 0E | build files (Makefile, Dockerfile, workflows, helm, docs) |

Each agent works in its own branch (`pr/0/agent-<X>`). Sequencer merges in
dependency order and runs full verification.

## Coordination notes

- c0.12 (audit_chain_state triggers) lands BEFORE c0.11 (CAS retry bound)
  because the retry-bound test verifies the trigger blocks retries.
- c0.16 (RegisterProvider SSRF) lands AFTER c0.3 (GitHub URL) because both
  touch OAuth plumbing.
- c0.34 (classifyInvokeError) is the last fix because it consolidates
  DEF-21's error mapping.
- c0.0 (read-only consumer check) is a no-op commit but MUST be first to
  establish baseline.