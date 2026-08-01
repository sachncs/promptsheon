# Compliance Status

This document tracks the repository's compliance with
[`AGENTS.md`](../../AGENTS.md). It records deliberate exceptions
where the repository deviates from a stated rule, the rationale,
and the conditions under which the deviation is acceptable.

## Exceptions

### `.golangci.yml` lacks a `version:` field

The pre-existing `.golangci.yml` was authored before
`golangci-lint` v2. `make lint` exits with the v2 error
`unsupported version of the configuration: ""` and requires a
migration to the v2 schema (the `version: "2"` directive and
the new `linters.default` / `linters.settings` layout).

This work is outside the scope of the Phase 0–6 compliance
sweep; the migration is a tooling upgrade that should land as
its own change once the team agrees on the v2 linter set.

Until the migration lands, the quality gates that *are*
reliable in this repository are:

- `make fmt` (`gofmt -s -w . && goimports -w .`)
- `make vet` (`go vet ./...`)
- `make lint-domain` (`scripts/check-no-package-state.go`)
- `make docs-check` (`scripts/docs-freshness.awk`)
- `make test` (`go test -race -count=1 ./...`)
- `make openapi-check` (`scripts/genopenapi`)

These are the gates the Phase 0–6 sweep validates. The
golangci-lint v2 migration is tracked separately.
