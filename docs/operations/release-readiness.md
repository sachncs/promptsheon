# Release Readiness Checklist

This document is the authoritative release checklist for `promptsheon`.
The CI release-config job enforces the gates; this document names the
gates, the commands that exercise them, the expected results, and the
escalation path when a gate fails.

The checklist is sequenced roughly from cheap / fast at the top to
slow / cloud at the bottom. A green pipeline at every step is the
release-readiness signal; a red line is a release-blocker.

## Roles

| Role | Responsibility |
|---|---|
| Release manager | Cuts the tag, owns the release, runs the checklist |
| On-call | First responder for a release-time incident; rolls back if a step fails |
| Maintainer | Triages gate failures and merges fix-forward commits |

## Gates

### 1. Local environment — `make help` exits 0 and lists every target

```
make help
```

Expected: every target in the `Targets:` block is listed. Failure
escalates to the maintainer on call.

### 2. Toolchain — `go version` matches `go.mod`

```
go version
head -3 go.mod
```

Expected: the version on stdout matches the second line of `go.mod`
(`go 1.26.5`). Escalation: bump `go.mod` and the CI matrix in
`.github/workflows/ci.yaml` together, or pin a different toolchain.

### 3. Format and vet — `make fmt vet` exit 0

```
make fmt vet
```

Expected: zero files reported by `gofmt -l .`; `go vet` prints
nothing. Escalation: the gate that failed dictates the next step
(gofmt drift → `make fmt`; vet finding → fix the code).

### 4. Static analysis — `make lint` exit 0

```
make lint
```

Expected: `scripts/run-lint.sh` exits 0. The script wraps
`staticcheck` and diffs against `scripts/lint-baseline.txt`; new
findings fail the gate. Escalation: fix the finding, or add it to
the baseline (with justification) and re-run.

### 5. Tests — `go test -race -count=1 ./...` exit 0

```
go test -race -count=1 -timeout 120s ./...
```

Expected: every package in `./...` returns `ok` or `?` (no test
files). The CI matrix runs the same command; the local pre-push
hook enforces it. Escalation: a failing package is a release
blocker; debug locally, do not skip with `-short` to push a tag.

### 6. Coverage — `make coverage-raw` meets the floors

```
make coverage-raw
bash scripts/check-coverage.sh coverage.out
bash scripts/check-coverage.sh --self-test
```

Expected: `check-coverage.sh` exits 0 and prints `OK: …` for every
package bucket (`promptsheon`, `promptsheon/store`, `api handlers`,
plus the domain packages). The CI `test` job runs the same script.
Escalation: a new package with no coverage fails the gate; add at
least one test before adding the package to the registry.

### 7. Public SDK — `make check-public` exit 0

```
make check-public
```

Expected: `GOFLAGS=-tags=promptsheon go vet` and `go test` both
exit 0. The SDK facade is the only public Go surface for downstream
consumers; breakage here is a release blocker. Escalation: every
build-tag-gated symbol must compile, every export must round-trip
through a test, and the public `Client` must remain stable.

### 8. OpenAPI drift — `make openapi-check` exit 0

```
make openapi-check
```

Expected: `git diff --exit-code promptsheon/spec/spec.yaml` reports
no diff. The CI `test` job runs the same command. Escalation: run
`make openapi`, commit the regenerated `spec.yaml`, and re-run the
gate. If the diff is unexpected, treat it as a handler change that
needs review before release.

### 9. Docs freshness — `make docs-check` exit 0

```
make docs-check
```

Expected: the awk-driven check in `scripts/docs-freshness.awk`
reports no broken `docs/architecture/README.md` links and no stale
source-path references. Escalation: fix the link or path; do not
suppress with `<!-- stale-ok: -->` unless the reference is to a
deliberately historical document.

### 10. Container build — `docker build` succeeds

```
docker build --no-cache -t promptsheon:dev .
```

Expected: the multi-stage build finishes; the resulting image boots
as non-root (`USER promptsheon`); the embed is current (the
frontend-build stage populates `cmd/promptsheond/frontend/dist`).
The CI `smoke` job exercises a fresh image. Escalation: a
non-zero exit is a release blocker; reproduce locally, fix the
Dockerfile, retag.

### 11. Container smoke — `make web-smoke` exit 0

```
make web-smoke
```

Expected: the dashboard's playwright smoke exits 0. Escalation: a
regression here is usually a UI contract change; coordinate with
the dashboard owner before bumping the embed.

### 12. Helm chart — `helm lint` and schema validation pass

```
helm lint deploy/helm/promptsheon/
helm template render-defaults deploy/helm/promptsheon/ > /tmp/r.yaml
```

Expected: `helm lint` exits 0; the rendered manifest is non-empty.
The CI `helm` job runs the same checks plus `kubeconform` against
the rendered output. Escalation: schema rejection (`replicaCount=3`,
`dbBackend=postgres`, `auth=false` without `insecureLoopback`) is
a deliberate guard rail; the template must remain restrictive.

### 13. Security — `gosec`, `govulncheck`, `gitleaks` clean

```
make security
go install honnef.co/go/tools/cmd/staticcheck@latest && staticcheck ./...
```

Expected: `govulncheck ./...` reports no known vulnerabilities;
`gosec ./...` has zero unexplained findings; the gitleaks
pre-commit hook reports no leaks. Escalation: every gosec finding
must either be fixed or annotated with a `// #nosec G###` line that
includes the justification in a comment.

### 14. Lint and domain-purity gates

```
make purity
```

Expected: `lint-domain` (no package-level mutable state) and
`lint-deps` (no infra imports from domain packages) both exit 0.
These gates enforce the project's domain-isolation rules. The CI
`lint` job runs the same gate. Escalation: a domain package that
imports an infra package is a design violation; refactor before
releasing.

### 15. Benchmarks — `make bench` exits 0

```
make bench
```

Expected: `scripts/run-benchmarks.sh` runs the eight curated
benchmarks from `scripts/benchmarks.txt` and exits 0 (each
benchmark executes exactly once). Escalation: if a benchmark
stops running, treat it as a missing test case — add a new entry
or fix the matcher before cutting a tag.

### 16. GoReleaser config — `goreleaser check` exits 0

```
goreleaser check
```

Expected: zero output. The CI `release-config` job runs the same
gate. Escalation: an invalid `.goreleaser.yml` is a release blocker
because `goreleaser release` will fail at tag time. Fix the config,
re-run the gate.

### 17. SBOM and signing paths — `make helm-docs` and signing key valid

```
make helm-docs
cosign version
```

Expected: the Helm chart README regenerates cleanly (no diff);
`cosign` is on the runner and reachable. The CI `build-release`
job runs the same checks plus the SBOM upload and attestation
steps. Escalation: a missing `helm-docs` binary is a no-op (the
target is a soft gate); a missing `cosign` is a release blocker
because the release artefacts must be signed.

### 18. Tag cut and release published

```
git tag -s vX.Y.Z -m "vX.Y.Z"
git push origin vX.Y.Z
```

Expected: the tag is signed (the project uses `git tag -s`); the
push triggers the CI `build-release` job; the resulting release
artefacts include archives, checksums, SBOMs, signatures, the
multi-arch image, and attestations. Escalation: a partial release
is worse than a delayed release; if `build-release` fails midway,
do not manually patch artefacts — roll forward with a fix commit
and re-tag.

## Rollback

If a release ships a regression:

1. Mark the release as a pre-release on GitHub so downstream
   consumers are warned.
2. Push a follow-up `git revert` (or a fix-forward commit) and
   re-tag as `vX.Y.(Z+1)`.
3. Pull the container image (`docker pull ghcr.io/.../promptsheond:vX.Y.Z`)
   and `docker tag` it as `:previous` so operators can pin to the
   last known-good image.
4. Notify on-call; the incident post-mortem is filed against the
   release tag.

## Escalation path

A failed gate above is a release blocker unless explicitly noted.
The maintainer on call triages in this order:

1. Reproduce the failure locally.
2. Identify the responsible change (the most recent commit on the
   release branch that touched the affected code).
3. Either revert the change or land a fix-forward commit.
4. Re-run the failing gate and the rest of the checklist.
5. Communicate the hold to the release manager.

## Cadence

This checklist runs on every PR (the CI jobs are the hard gate)
and on every tag push (the `build-release` job is the publishing
gate). Manual runs from a maintainer are reserved for hot-fix
branches and for debugging an in-flight release.
