# PHASE-3 — Open-Source Hygiene

**16 commits.** Adds all the OSS-readiness files: funding, license headers,
issue templates, security policy, SDK publish workflows, scorecard, gitleaks,
TLA+ enforcement, CODEOWNERS rewrite, Renovate gate.

**Can run in parallel with Phase-2, Phase-4, Phase-5.**

## Pre-PR-3 manual checklist

```
[ ] PYPI_TOKEN created and added to repo secret PYPI_TOKEN
[ ] NPM_TOKEN created and added to repo secret NPM_TOKEN
[ ] Branch protection rules enabled on master
[ ] OpenSSF Scorecard configured (Settings → Code security → Scorecard)
[ ] Issue labels: good-first-issue, help-wanted, security, priority/*
```

## Commits

```
c3.0  chore(security): SECURITY.md supported versions = 0.1.x, 0.2.x, 0.3.x, 0.4.x
      Refs: PLAN-49/
c3.1  chore(release): cut annotated v0.4.0 git tag
      Refs: PLAN-49/M-8
c3.2  chore(release): goreleaser prerelease strategy matches tag rules
      Refs: PLAN-49/
c3.3  chore(github): FUNDING.yml
      Refs: PLAN-49/
c3.4  chore(github): ISSUE_TEMPLATE/{config.yml,security.md,question.md}
      Refs: PLAN-49/
c3.5  chore(license): SPDX headers via tiny awk script + gofmt
      Refs: PLAN-49/
c3.6  chore(license): add NOTICE file
      Refs: PLAN-49/
c3.7  chore(release): git-cliff config (clift.toml)
      Refs: PLAN-49/
c3.8  chore(ci): publish-python.yml (manual approval gate, secrets-based auth)
      Refs: PLAN-49/C-10
c3.9  chore(ci): publish-npm.yml (manual approval gate, secrets-based auth)
      Refs: PLAN-49/C-10
c3.10 chore(ci): scorecard workflow
      Refs: PLAN-49/
c3.11 chore(docs): move docs/security/audit-2026-07-26.md → docs/research/
      Refs: PLAN-49/
c3.12 chore(codeowners): rewrite for current backend/<pkg>/ paths (no /internal/)
      Refs: PLAN-49/
c3.13 chore(renovate): requiredStatusChecks gate enabled
      Refs: PLAN-49/
c3.14 chore(ci): tla job installs tlaplus/tlaplus action
      Refs: PLAN-49/
c3.15 chore(gitleaks): .gitleaks.toml with ps_<hex> API key pattern
      Refs: PLAN-49/L-8
```

## Files touched (added unless noted)

| File | Commit |
|---|---|
| `SECURITY.md` (edit) | c3.0 |
| `VERSION` (edit) | c3.1 |
| `.goreleaser.yml` (edit) | c3.2 |
| `.github/FUNDING.yml` (new) | c3.3 |
| `.github/ISSUE_TEMPLATE/config.yml` (new) | c3.4 |
| `.github/ISSUE_TEMPLATE/security.md` (new) | c3.4 |
| `.github/ISSUE_TEMPLATE/question.md` (new) | c3.4 |
| `scripts/add-spdx-headers.sh` (new) | c3.5 |
| `NOTICE` (new) | c3.6 |
| `clift.toml` (new) | c3.7 |
| `.github/workflows/publish-python.yml` (new) | c3.8 |
| `.github/workflows/publish-npm.yml` (new) | c3.9 |
| `.github/workflows/scorecard.yml` (new) | c3.10 |
| `docs/security/audit-2026-07-26.md` (move) | c3.11 |
| `CODEOWNERS` (rewrite) | c3.12 |
| `renovate.json` (edit) | c3.13 |
| `.github/workflows/tla.yml` (edit) | c3.14 |
| `.gitleaks.toml` (new) | c3.15 |

## Key configurations

### publish-python.yml (c3.8)

```yaml
name: publish-python
on:
  push:
    tags: ['v*']
jobs:
  publish:
    runs-on: ubuntu-latest
    environment: production-pypi  # manual approval gate
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-python@v5
        with:
          python-version: "3.11"
      - name: Build distributions
        working-directory: sdk/python
        run: |
          python -m pip install --upgrade build
          python -m build
      - name: Publish to PyPI
        working-directory: sdk/python
        env:
          TWINE_USERNAME: __token__
          TWINE_PASSWORD: ${{ secrets.PYPI_TOKEN }}
        run: twine upload dist/*
```

### publish-npm.yml (c3.9)

```yaml
name: publish-npm
on:
  push:
    tags: ['v*']
jobs:
  publish:
    runs-on: ubuntu-latest
    environment: production-npm  # manual approval gate
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v6
        with:
          node-version: "22"
          registry-url: https://registry.npmjs.org
      - name: Build and publish
        working-directory: sdk/typescript
        env:
          NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
        run: |
          npm ci
          npm run build
          npm publish --provenance --access public
```

### scorecard.yml (c3.10)

```yaml
name: scorecard
on:
  branch_protection_rule:
  schedule:
    - cron: '0 6 * * 1'
jobs:
  analysis:
    runs-on: ubuntu-latest
    permissions:
      security-events: write
    steps:
      - uses: actions/checkout@v4
      - uses: github/codeql-action/init@v3
      - uses: ossf/scorecard-action@v2
        with:
          results_file: results.sarif
          results_format: sarif
      - uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: results.sarif
```

### .gitleaks.toml (c3.15)

```toml
title = "promptsheon secrets"

[extend]
useDefault = true

[[rules]]
id = "promptsheon-api-key"
description = "promptsheon API key (ps_ prefix + 64 hex)"
regex = '''\bps_[a-f0-9]{64}\b'''
keywords = ["ps_"]

[[rules]]
id = "promptsheon-bootstrap-token"
description = "PROMPTSHEON_BOOTSTRAP_TOKEN env var leaked"
regex = '''PROMPTSHEON_BOOTSTRAP_TOKEN[ ]*=[ ]*["']?[A-Za-z0-9+/=]{16,}'''
keywords = ["PROMPTSHEON_BOOTSTRAP_TOKEN"]
```

### CODEOWNERS rewrite (c3.12)

```
# Default
*                                       @sachncs

# Backend domain areas
/backend/auth/                          @sachncs
/backend/audit/                         @sachncs
/backend/release/                       @sachncs
/backend/capability/                    @sachncs
/backend/harness/                       @sachncs
/backend/handlers_*.go                  @sachncs
/backend/store/                         @sachncs
/backend/vault/                         @sachncs

# Specs and SDKs
/backend/spec/                          @sachncs
/sdk/                                   @sachncs

# Infrastructure
/.github/workflows/                     @sachncs
/Dockerfile                             @sachncs
/Makefile                               @sachncs
/.goreleaser.yml                        @sachncs
/deploy/helm/                           @sachncs

# Frontend
/frontend/                              @sachncs

# Docs
/docs/                                  @sachncs
```

## Exit criterion

```bash
gitleaks detect --no-git
scorecard-action  # locally run via ossf/scorecard-action
gh workflow run tla.yml  # verify tlc runs and exits 0
gh workflow run scorecard.yml  # verify scorecard runs
```

## Parallelization

2 agents:

| Agent | Files |
|---|---|
| 3A | SECURITY.md, LICENSE, NOTICE, .github/, CODEOWNERS, renovate.json, .gitleaks.toml |
| 3B | .github/workflows/publish-*, .github/workflows/scorecard.yml, .goreleaser.yml |