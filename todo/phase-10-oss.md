# Phase 10 — OSS Readiness

OSS items minus the seven excluded by the maintainer (no FUNDING.yml, no CODE_OF_CONDUCT enforcement, no SECURITY-INSIGHTS.yml, no security.txt, no co-maintainers list, no embargo policy, no PGP fingerprint).

## Release pipeline

- [ ] **OSS-REL-1** Add `docker_images:` to `.goreleaser.yml` so the GoReleaser pipeline publishes to GHCR.
  - **Accept**: A tag push produces a multi-arch image at `ghcr.io/sachncs/promptsheon:vX.Y.Z`.

- [ ] **OSS-REL-2** Add `signs:` to `.goreleaser.yml` with cosign keyless signing.
  - **Accept**: Every release artefact is signed; `cosign verify --certificate-identity-regexp ...` succeeds.

- [ ] **OSS-REL-3** Add `nfpms:` (deb/rpm) and `brews:` (homebrew tap) to `.goreleaser.yml`.

- [ ] **OSS-REL-4** Add `snapshot:` config for `goreleaser release --snapshot` so contributors can validate locally.

- [ ] **OSS-REL-5** Add SLSA provenance attestation via `slsa-github-generator`.

- [ ] **OSS-REL-6** Attach SBOM to GitHub release (not just workflow artefact).

## Signing

- [ ] **OSS-SIGN-1** Add DCO (`Signed-off-by:`) requirement to PR template + CI check.
- [ ] **OSS-SIGN-2** Document commit signing requirement in `CONTRIBUTING.md`.

## Issues / PRs

- [ ] **OSS-ISSUE-1** Add `docs/issue-template-config.yml` with default labels (`area:`, `kind:`, `priority:`).
- [ ] **OSS-ISSUE-2** Add a `feature_request.md` "Roadmap item" checkbox that links to `docs/roadmap.md`.
- [ ] **OSS-ISSUE-3** Add a `security.md` issue template that auto-redacts and routes to security@.

## Roadmap / governance

- [ ] **OSS-GOV-1** Publish `docs/roadmap.md` with the v0.2.0 / v0.3.0 / v1.0.0 commitments extracted from the README and the Phase 6/11 work.

- [ ] **OSS-GOV-2** Add `MAINTAINERS.md` listing current maintainers (even if just one) and the process for adding more.
- [ ] **OSS-GOV-3** Add `GOVERNANCE.md` describing decision-making (lazy consensus for non-controversial; vote on breaking changes; security embargo process).
- [ ] **OSS-GOV-4** Add `docs/release.md` covering: SemVer policy, deprecation policy (which we just decided is "no deprecation, fast forward"), supported versions table.

## Branding

- [ ] **OSS-BRAND-1** Add `logo.svg` and `logo.png` at repo root, link from README and chart.
- [ ] **OSS-BRAND-2** Update Helm chart icon to use the logo.
  - **Where**: `deploy/helm/promptsheon/Chart.yaml:22`.

## CI hardening

- [ ] **OSS-CI-1** Add a concurrency guard on `ci.yaml`.
- [ ] **OSS-CI-2** Add explicit `permissions:` block.
- [ ] **OSS-CI-3** Add `actions/cache` for Go module cache.
- [ ] **OSS-CI-4** Add CodeQL workflow for SAST.
- [ ] **OSS-CI-5** Add Trivy scan on built artefacts.
- [ ] **OSS-CI-6** Add `gitleaks` for secret scanning.
- [ ] **OSS-CI-7** Add SBOM diff gate — release blocked if SBOM changed without CHANGELOG.

## Dependabot / Renovate

- [ ] **OSS-DEP-1** Add `docker` ecosystem to Dependabot.
- [ ] **OSS-DEP-2** Add `npm` ecosystem to Dependabot for the TypeScript SDK.
- [ ] **OSS-DEP-3** Add `pip` ecosystem to Dependabot for the Python SDK.
- [ ] **OSS-DEP-4** Add Dependabot groups so internal-package updates batch into one PR.
- [ ] **OSS-DEP-5** Add Renovate alongside Dependabot.

## README

- [ ] **OSS-README-1** Add a "What is Promptsheon?" 1-paragraph at the top.
- [ ] **OSS-README-2** Add a "Status" badge (CI, release, license, GoReportCard, OpenSSF Scorecard).
- [ ] **OSS-README-3** Add a "Quick start" GIF or terminal recording.
- [ ] **OSS-README-4** Add a "Comparison" section: Promptsheon vs LangSmith vs Helicone vs Portkey.
- [ ] **OSS-README-5** Add a "Citation" section with a CITATION.cff.

## Discovery

- [ ] **OSS-DISC-1** Add `topics:` to `go.mod` GitHub repo metadata.
- [ ] **OSS-DISC-2** Submit to OpenSSF Best Practices badge program.
- [ ] **OSS-DISC-3** Add a CNCF landscape entry when ready.
- [ ] **OSS-DISC-4** Add a `docs/showcase.md` listing production users (once any exist).

## Examples / tutorials

- [ ] **OSS-EX-1** Add a "5-minute eval" tutorial in `examples/eval-quickstart/`.
- [ ] **OSS-EX-2** Add a "Promote-to-prod" tutorial in `examples/promote/`.
- [ ] **OSS-EX-3** Add a "Plugin authoring" tutorial in `examples/plugin/`.
- [ ] **OSS-EX-4** Add a "Webhook integration" tutorial in `examples/webhook/`.
- [ ] **OSS-EX-5** Replace the broken `examples/bash/invoke-release.sh` (see API-6) with a working version.

## SDK distribution

- [ ] **OSS-SDK-1** Publish Go SDK to pkg.go.dev (it's already importable; ensure go.mod has the right module path).
- [ ] **OSS-SDK-2** Publish Python SDK to PyPI from a tagged release.
- [ ] **OSS-SDK-3** Publish TypeScript SDK to npm from a tagged release.
- [ ] **OSS-SDK-4** Add SDK versioning policy in `docs/sdk-versioning.md`.

## Community signals

- [ ] **OSS-COMM-1** Add a `CODE_OF_CONDUCT.md` link from the PR template and issue templates.
- [ ] **OSS-COMM-2** Add `docs/decisions.md` summary index that links to ADRs (the existing `docs/design-decisions.md` only covers 10 of 22 ADRs — extend or replace).
