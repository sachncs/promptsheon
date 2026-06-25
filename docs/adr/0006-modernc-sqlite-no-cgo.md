# ADR 0006: Use `modernc.org/sqlite` instead of `mattn/go-sqlite3`

- **Status:** Accepted
- **Date:** 2024-01-01
- **Supersedes:** —
- **Superseded by:** —

## Context

We need a SQLite driver. The two dominant options for Go are:

- `github.com/mattn/go-sqlite3` — a `cgo` binding to the C SQLite library. Fast, but requires a C toolchain at build time and ships a dynamically linked C library. Cross-compilation requires a C cross-toolchain.
- `modernc.org/sqlite` — a pure-Go translation of the SQLite source. Slower (roughly 2–3×) but requires no C toolchain, produces a fully static binary, and cross-compiles with `GOOS`/`GOARCH` alone.

## Decision

We use `modernc.org/sqlite`. The single direct dependency of the project is `modernc.org/sqlite`. The build does not require `CGO_ENABLED=1` and produces a static binary for every supported target.

## Options considered

1. **`modernc.org/sqlite` (chosen).** Pure Go, no CGO, no toolchain, fully static.
2. **`mattn/go-sqlite3`.** Faster, but CGO dependency makes the build brittle (CI runners need a C compiler, cross-compilation is awkward).
3. **Embedded Postgres or another RDBMS.** Stronger than we need, requires a separate process.
4. **A custom file format.** Would lose all of the SQL tooling we rely on.

## Consequences

Positive:

- A `go build` from a clean checkout produces a working binary with no C toolchain.
- The Docker image is `FROM scratch` or `FROM alpine` with no `build-base` stage.
- Cross-compilation (`GOOS=linux GOARCH=arm64 go build`) works without a C cross-toolchain.

Negative:

- Performance. The pure-Go driver is slower than the C binding. For our write rates (tens of thousands of audit entries per day at most) this is invisible.
- Binary size. `modernc.org/sqlite` is several megabytes larger than `mattn/go-sqlite3` after stripping. Acceptable for a server binary.
- We cannot use SQLite extensions that have C-only bindings (none required by us today).

## References

- `go.mod` — only direct dependency
- `internal/store/sqlite.go` — driver registration
- `docs/deployment.md` — Docker build, no `gcc` required
