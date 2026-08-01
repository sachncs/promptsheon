# PHASE-5A — Foundational Renames + Dead Code

**15 commits.** Lands first to establish the ban-rule linter (c5.0) and then
drops dead code across the repo. Runs after PR-4D.

## STOP gate after PR-5A

Verify golangci-lint rule catches zero new offenders. If it does, fix the
violations before PR-5B opens.

## Commits

```
c5.0  chore(lint): custom golangci-lint plugin banning ~40 dropped identifiers
      Refs: PLAN-49/C-2 H-8 L-7
c5.1  refactor: drop auditField* redundancy (consolidate from Phase 2.c11)
      Refs: PLAN-49/C-3
c5.2  rename: DiffIntelligence → Compare
      Refs: PLAN-49/P1-15
c5.3  rename: ListRefDetails/ListRefs → ListBranches; HEADRefName → Head
      Refs: PLAN-49/P1-16
c5.4  rename: handle_observation.go → handlers_observation.go
      Refs: PLAN-49/P1-1
c5.5  rename: handlers_capabilities_merge_test.go → handlers_capabilities_test.go
      Refs: PLAN-49/P1-8
c5.6  refactor: drop OtelExporter (trace/exporter.go); replace with slog.Handler chain
      Refs: PLAN-49/
c5.7  refactor: drop CasPromptLoader
      Refs: PLAN-49/
c5.8  refactor: drop ArtifactLoader interface
      Refs: PLAN-49/
c5.9  refactor: drop release/id.go + evolve/id.go (inline)
      Refs: PLAN-49/
c5.10 refactor: drop auditVerifyCache + auditTail + 4 prepared statements
      Refs: PLAN-49/
c5.11 refactor: drop mustUnmarshal/marshalOrErr (proper error propagation)
      Refs: PLAN-49/L-6
c5.12 refactor: drop SQLiteIdempotencyStore wrapper
      Refs: PLAN-49/
c5.13 refactor: drop splitLeadingPragma/splitOnFirstStatement
      Refs: PLAN-49/
c5.14 refactor: drop Rollups.Sink
      Refs: PLAN-49/
c5.15 refactor: drop dead metrics + frontend dead exports (combined; resolves L-5)
      Refs: PLAN-49/L-5
```

## Files touched

| File | Commits |
|---|---|
| `.golangci.yml` (extend with custom plugin) | c5.0 |
| `tools/golangci-lint-promptsheon/main.go` (new plugin) | c5.0 |
| `backend/audit/keys.go` (consolidate) | c5.1 |
| `backend/cas/diff.go` (DiffIntelligence → Compare) | c5.2 |
| `backend/cas/branch.go`, `backend/cas/store.go` | c5.3 |
| `backend/handler_observation.go` → `backend/handlers_observation.go` | c5.4 |
| `backend/handlers_capabilities_merge_test.go` → `backend/handlers_capabilities_test.go` | c5.5 |
| `backend/trace/exporter.go` (delete), `backend/trace/otel.go` (edit) | c5.6 |
| `promptsheon/evolve/loader.go` (delete), `promptsheon/evolve/evolver.go` (edit) | c5.7 |
| `backend/release/resolver.go` (delete interface), `backend/release/service.go` | c5.8 |
| `backend/release/id.go` (delete), `backend/release/service.go` | c5.9 |
| `promptsheon/evolve/id.go` (delete), `promptsheon/evolve/promoter.go` | c5.9 |
| `backend/store/sqlite.go` (delete cache) | c5.10 |
| `backend/store/sqlite.go` (drop helpers) | c5.11 |
| `backend/store/idempotency_sqlite.go` (delete), `backend/store/repo.go` | c5.12 |
| `backend/store/migrate.go` (delete helpers) | c5.13 |
| `backend/rollups/rollups.go` (delete Sink) | c5.14 |
| `backend/metrics/collector.go` (drop dead counters) | c5.15 |
| `frontend/src/views/settings-view.js` (drop dead exports) | c5.15 |
| `frontend/src/views/catalog-view.js` (drop dead exports) | c5.15 |

## Custom golangci-lint plugin (c5.0)

### Plugin location

`tools/golangci-lint-promptsheon/main.go`

### Banned identifiers (~40)

```go
package main

import (
    "github.com/golangci/plugin-module-register/register"
    "github.com/golangci/plugin-module-register/settings"
    "golang.org/x/tools/go/analysis"
)

func init() {
    register.Plugin("promptsheon-bans", New)
}

type BanConfig struct {
    Identifiers []string `json:"identifiers"`
    Patterns    []string `json:"patterns"`
}

func New(settings *settings.PluginSettings) (*register.Plugin, error) {
    return &register.Plugin{
        Inspect: func(pass *analysis.Pass) (interface{}, error) {
            banned := []string{
                "BypassSSRF",
                "activeOAuthStates",
                "Repositories",  // type
                "NewAuthenticatorWithLogger",
                "NewManagerWithDB",
                "WithAllowInsecure",
                "SetDeliveryFunc",
                "inputHash",
                "modelRevision",
                "errProviderMissing",
                "manifestHash",
                "computeManifestHash",
                "marshalNoArgs",
                "MarshalNoArgs",
                "streamOK",  // field
                "tunedTransport",
                "PricingTable",
                "EstimateTokens",
                "envJudgeProvider",
                "AggregateMetrics",
                "JudgeClient",
                "ValidateBaseURLs",
                "OtelExporter",
                "CasPromptLoader",
                "ArtifactLoader",
                "generateReleaseID",
                "generateID",
                "auditVerifyCache",
                "auditTail",
                "mustUnmarshal",
                "marshalOrErr",
                "SQLiteIdempotencyStore",
                "splitLeadingPragma",
                "splitOnFirstStatement",
                "UsageTracker",
                "PersistedEnforcer",
                "apiReleaseInvoker",
                "evolverActivatorAdapter",
                "DiffIntelligence",
                "ListRefDetails",
                "ListRefs",
                "HEADRefName",
                "IsHEADDetached",
            }
            patterns := []string{
                `^Error[A-Z]`,  // Error* sentinels (must be Err*)
            }
            // Walk pass.Pkg, check each identifier
            // ...
            return nil, nil
        },
    }, nil
}
```

### .golangci.yml

```yaml
linters-settings:
  custom:
    promptsheon-bans:
      type: "module"
      description: "Bans identifiers removed in PLAN-49"
      identifiers: [...]
      patterns: [...]

linters:
  enable:
    - promptsheon-bans
```

### golangci-lint enablement in CI (L-7 fix)

`.github/workflows/ci.yaml` step:
```yaml
- uses: golangci/golangci-lint-action@v6
  with:
    version: v1.61.0
    args: --config .golangci.yml
```

## Exit criterion

```bash
golangci-lint run
go build ./...
go vet ./...
go test -race -count=1 ./...
grep -rn 'DiffIntelligence\|ListRefDetails\|HEADRefName' backend/ --include='*.go' | grep -v _test  # must return nothing
```

## STOP gate

After PR-5A closes, run `golangci-lint run` on every changed file. If the
custom plugin catches any banned identifier, fix it (the rule is meant to be
strict; no exceptions).

## Parallelization

3 agents:

| Agent | Files |
|---|---|
| 5A1 | tools/golangci-lint-promptsheon/, .golangci.yml |
| 5A2 | backend/cas/, backend/store/sqlite*.go, backend/store/idempotency_sqlite.go, backend/store/migrate.go, backend/release/id.go, promptsheon/evolve/id.go |
| 5A3 | backend/trace/, promptsheon/evolve/loader.go, backend/release/resolver.go (interface drop), backend/rollups/, backend/metrics/collector.go, frontend/src/views/{settings,catalog}-view.js |