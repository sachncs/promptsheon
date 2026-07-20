# Phase 8 — Documentation

All documentation drift. Fast forward: rewrite, don't deprecate.

## Version and defaults

- [ ] **DOC-2** Single source of truth for the version string. (See Phase 4.)
- [ ] **DOC-3** Update `docs/configuration.md` and `docs/security.md` to say rate-limit default is 100 (matches code + chart).
  - **Where**: `docs/configuration.md:92`, `docs/security.md:67`.

- [ ] **DOC-4** Update `docs/configuration.md` and `docs/security.md` to say CORS default is deny-all (matches code + chart).
  - **Where**: `docs/configuration.md:16`, `docs/security.md:31`.

- [ ] **DOC-5** Delete env-var references to Azure, Ollama, NVIDIA from `docs/configuration.md:54-64`.
  - **Accept**: A grep for `AZURE`, `OLLAMA`, `NVIDIA` returns no hits in `docs/`.

## Coverage and CI

- [ ] **DOC-6** Update `docs/development.md:101` to say coverage gate is 60% (matches CI). Or raise CI to 70%.
  - **Where**: `docs/development.md:101` and `.github/workflows/ci.yaml:62`.

- [ ] **DOC-7** Fix `docs/development.md:111` to say `./tests/...` (plural, matches Makefile).
- [ ] **DOC-8** Update `docs/development.md:177` to remove "(Future) make security" — it's already wired.

## Getting started / API drift

- [ ] **DOC-13** Rewrite `docs/getting-started.md` to use Capability / Version / Release from line one. Delete the legacy `POST /api/v1/prompts` examples.
- [ ] **DOC-14** Regenerate `docs/api-reference.md` route counts from the current OpenAPI spec.
- [ ] **DOC-15** Rewrite `docs/getting-started.md:121-131` to document the Git-style CLI (`promptsheon init/log/write-object`).

## ADRs

- [ ] **DOC-9a** Delete `ADR 0015` and `ADR 0017` entries from `docs/adr/README.md`.
- [ ] **DOC-9b** Add stub `docs/adr/0013-...md`, `0014-...md`, `0015-...md`, `0017-...md` documenting the topics the index claims they cover.
- [ ] **DOC-10** Add `ADR 0024` and `ADR 0025` to `docs/adr/README.md`.
- [ ] **DOC-11** Rewrite `docs/adr/0016-plugins-over-grpc.md` as Markdown (currently `//`-prefixed).
- [ ] **DOC-12** Pick one of `ADR 0024` or `ADR 0025` to keep; delete the other. Or merge them.
- [ ] **DOC-ADR-1** Add ADRs for: SQLite trace removal (Phase 3 OBS-TR-1), idempotency SQLite backend (Phase 4 API-IDEMP-1), leader election (Phase 6 OPS-1b), HTTPS-only webhook URLs (Phase 1 SEC-4c).
- [ ] **DOC-ADR-2** Add a "Superseded" footer convention enforcement: every ADR carries a `Status: Accepted | Superseded by NNNN | Deprecated` line.

## Code-doc contradictions

- [ ] **DOC-1** Fix `internal/approval/approval.go:152-155` doc comment to match the actual fail-closed behaviour.

## Stale doc references

- [ ] **DOC-16** Link `CHANGELOG.md` from `docs/README.md`.
- [ ] **DOC-17** Add `docs/upgrade.md` covering in-place upgrades and destructive migrations.

## Per-doc freshness

- [ ] **DOC-FRESH-1** Add a "Last reviewed" footer to every doc in `docs/`; CI fails when any doc is older than 90 days without review.
  - **Where**: `docs/README.md:86` already documents the rule; make it executable.

- [ ] **DOC-FRESH-2** Add a `docs/llm-providers.md` table that lists only the providers present in `internal/llm/registry.go` (Anthropic + OpenAI today). Drop Azure/Ollama/NVIDIA references.

## CHANGELOG discipline

- [ ] **DOC-CHG-1** Add a CHANGELOG entry for every v0.1.x patch that lands a Phase 0–9 fix.

## Helm README

- [ ] **DOC-HELM-1** Run `helm-docs` and commit `deploy/helm/promptsheon/README.md`.

## Glossary

- [ ] **DOC-GLOSS-1** Add `Capability`, `Version`, `Release`, `Manifest`, `Environment`, `Activation`, `Precondition`, `EvalRun`, `Dataset`, `Scorer`, `Bandit` to `docs/glossary.md` if missing.

## New docs to write

- [ ] **DOC-NEW-1** `docs/multi-region.md` — multi-region strategy (or non-goal) for v0.1.x.
- [ ] **DOC-NEW-2** `docs/upgrade.md` — in-place upgrade procedure.
- [ ] **DOC-NEW-3** `docs/canary.md` — canary / blue-green deployment.
- [ ] **DOC-NEW-4** `docs/slos.md` — SLI / SLO / error-budget definitions.
- [ ] **DOC-NEW-5** `docs/operations.md` — backup, restore, chaos drill cadence.
- [ ] **DOC-NEW-6** `docs/observability.md` rewrite — describe the wired (not dormant) OTel / SSE / SLO story.

## SDK docs

- [ ] **DOC-SDK-1** Update `sdk/README.md` to point at the generated SDK in PyPI / npm; remove the "hand-written" framing.
- [ ] **DOC-SDK-2** Add a "Migrating from v0.0.x" page covering the Capability/Version/Release cutover.

## Contributing

- [ ] **DOC-CONTR-1** Rewrite `CONTRIBUTING.md` with: required Go version (1.26.5), `make bootstrap` target, pre-commit hook install, OpenAPI regeneration step, signing commits with DCO.
- [ ] **DOC-CONTR-2** Add `CONTRIBUTING.md` step for SDK contributors (Python / TypeScript).

## Docs CI

- [ ] **DOC-CI-1** Add `markdownlint` config and CI step.
- [ ] **DOC-CI-2** Add `vale` for prose style.
- [ ] **DOC-CI-3** Add a doc-freshness check: any `docs/**/*.md` older than 90 days fails CI unless marked `stale-ok`.

## TLS / cert docs

- [ ] **DOC-TLS-1** Add `docs/tls.md` covering: cert-manager integration, ACME, mTLS, cert rotation.
