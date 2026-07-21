# Promptsheon Engineering TODO

Atomic, evidence-backed remediation plan derived from the Principal Staff engineering review.
Fast forward, no backward compatibility, breaking changes welcome.

## Phases

Progress is checked off in each phase file as work lands.

| # | Phase | File | Done / Total | Focus |
|---|---|---|---|---|
| 0 | Critical Blockers | [phase-0-critical-blockers.md](phase-0-critical-blockers.md) | 14 / 17 | Ship-blockers first |
| 1 | Security Hardening | [phase-1-security.md](phase-1-security.md) | 17 / 30 | All SEC-* items |
| 2 | Migration Chain Repair | [phase-2-migrations.md](phase-2-migrations.md) | 12 / 30 | All DB-* items |
| 3 | Observability Wiring | [phase-3-observability.md](phase-3-observability.md) | 8 / 34 | All OBS-* items |
| 4 | API Surface Repair | [phase-4-api.md](phase-4-api.md) | 5 / 36 | All API-* items |
| 5 | Domain Cleanup | [phase-5-domain.md](phase-5-domain.md) | 9 / 34 | All DEAD-* items |
| 6 | Operations | [phase-6-operations.md](phase-6-operations.md) | 5 / 40 | All OPS-* items |
| 7 | Testing | [phase-7-testing.md](phase-7-testing.md) | 5 / 35 | All TEST-* items |
| 8 | Documentation | [phase-8-documentation.md](phase-8-documentation.md) | 7 / 39 | All DOC-* items |
| 9 | Bug Fixes | [phase-9-bugs.md](phase-9-bugs.md) | 15 / 30 | All BUG-* items |
| 10 | OSS Readiness | [phase-10-oss.md](phase-10-oss.md) | 0 / 49 | Release pipeline, governance, distribution |
| 11 | Performance | [phase-11-performance.md](phase-11-performance.md) | 4 / 28 | All PERF-* items |
| 12 | Go Idiomatic | [phase-12-go.md](phase-12-go.md) | 2 / 37 | All GO-* items |

**Total: 439 atomic TODOs. ~83 done, ~356 remaining.** The progress numbers in this table are computed at commit time from the per-file `[x]` / `[ ]` markers; they are honest.

## Order of execution

1. **Phase 0** first — items here block shipping.
2. **Phase 1, 2, 3** in parallel after Phase 0 — security, migration, and observability are independent.
3. **Phase 4, 5, 9** after Phase 0-3 land.
4. **Phase 6, 7, 8, 11, 12** as the rest of the codebase stabilises.
5. **Phase 10** last — OSS distribution only matters once the project is shippable.

## Maintainer decisions applied

- **Migration 028-040**: not intentionally reserved — fill the gap or document in CHANGELOG.
- **MakerCheckerPolicy**: enforce separation-of-duties in the type itself.
- **Webhook `allow_private`**: replaced with an `ultimate RBAC model` — `PermWebhookAdmin` plus an explicit per-tenant SSRF allowlist.
- **OpenAPI 0.3.0 vs product 0.1.0**: drift, not roadmap — fix to a single source of truth.
- **ClickHouse build tag**: wire it up; it's the intended scale backend.
- **SSE log streaming**: wire it up; the endpoint is registered, the slog wiring is missing.
- **OAuth manager**: wire it up from `PROMPTSHEON_OAUTH_*` env vars.
- **genopenapi**: parser bug — extend the AST walker.
- **`PROMPTSHEON_*_BASE_URL` http://**: allowed only on loopback bind; rejected otherwise.
- **Tests' duplicated env vars**: copy-paste bug — dedupe.

## Excluded by maintainer

The following items from the original OSS gaps section were excluded:
- FUNDING.yml
- CODE_OF_CONDUCT enforcement in PR/issue templates
- SECURITY-INSIGHTS.yml
- well-known/security.txt
- co-maintainers / emeritus list
- embargo policy in SECURITY.md
- PGP fingerprint in SECURITY.md

## Convention

Each item is one concrete, verifiable change:
- `What`: the action
- `Where`: file:line
- `Accept`: the test or observation that proves it's done

Cross-references use the ID prefixes (`DB-`, `SEC-`, `OBS-`, `API-`, `DEAD-`, `OPS-`, `TEST-`, `DOC-`, `BUG-`, `PERF-`, `GO-`).
