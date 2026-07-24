#!/usr/bin/env bash
# scripts/sync-version.sh
#
# DOC-2a: single source of truth for the version string.
# Reads VERSION from the repo root and patches every location
# that pins a version literal:
#
#   - api/openapi.yaml          (info.version and the 0.1 prose)
#   - sdk/python/src/promptsheon/__init__.py (__version__)
#   - sdk/python/pyproject.toml  (project.version)
#   - sdk/typescript/package.json ("version")
#   - deploy/helm/promptsheon/Chart.yaml (version and appVersion)
#
# The internal/buildinfo.Version variable is set at link time
# via -ldflags="-X .../buildinfo.Version=$(cat VERSION)"; this
# script keeps the static files in sync with that.

set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="$(cat VERSION)"
if [[ -z "$VERSION" ]]; then
  echo "VERSION file is empty" >&2
  exit 1
fi

# api/openapi.yaml: info.version: "..." + the "Version N.N.N ..." prose comment.
# The generic regex matches any semver (with optional pre-release suffix)
# in the prose so a future bump from 0.1.0 -> 0.2.0 -> 0.3.0 doesn't
# require editing this script.
sed -i.bak -E "s|(version:[[:space:]]*\")[^\"]*(\")|\1${VERSION}\2|" api/openapi.yaml
sed -i.bak -E "s|(Version )[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]*)?[[:space:]]|\1${VERSION} |" api/openapi.yaml
rm -f api/openapi.yaml.bak

# sdk/python/src/promptsheon/__init__.py: __version__ = "..."
sed -i.bak -E "s|(__version__[[:space:]]*=[[:space:]]*\")[^\"]*(\")|\1${VERSION}\2|" \
    sdk/python/src/promptsheon/__init__.py
rm -f sdk/python/src/promptsheon/__init__.py.bak

# sdk/python/pyproject.toml: project.version = "..." (TOML, unquoted
# in this file, but the form is identical to the JSON pattern).
sed -i.bak -E "s|(^version[[:space:]]*=[[:space:]]*\")[^\"]*(\")|\1${VERSION}\2|" \
    sdk/python/pyproject.toml
rm -f sdk/python/pyproject.toml.bak

# sdk/typescript/package.json: "version": "..."
sed -i.bak -E "s|(\"version\"[[:space:]]*:[[:space:]]*\")[^\"]*(\")|\1${VERSION}\2|" \
    sdk/typescript/package.json
rm -f sdk/typescript/package.json.bak

# deploy/helm/promptsheon/Chart.yaml: version and appVersion
# (both are unquoted semver). BSD sed has no `(a|b)`; run
# two anchored substitutions.
sed -i.bak -E "s|^(version:[[:space:]]*)[0-9]+\.[0-9]+\.[0-9]+|\1${VERSION}|" \
    deploy/helm/promptsheon/Chart.yaml
sed -i.bak -E "s|^(appVersion:[[:space:]]*)[0-9]+\.[0-9]+\.[0-9]+|\1${VERSION}|" \
    deploy/helm/promptsheon/Chart.yaml
rm -f deploy/helm/promptsheon/Chart.yaml.bak

echo "synced version ${VERSION} to openapi.yaml, sdk/python, sdk/typescript, helm chart"