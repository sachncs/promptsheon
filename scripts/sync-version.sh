#!/usr/bin/env bash
# scripts/sync-version.sh
#
# DOC-2a: single source of truth for the version string.
# Reads VERSION from the repo root and patches every location
# that pins a version literal:
#
#   - api/openapi.yaml          (info.version)
#   - sdk/python/src/promptsheon/__init__.py (__version__)
#   - sdk/typescript/package.json ("version")
#   - deploy/helm/promptsheon/Chart.yaml (version)
#
# The internal/buildinfo.Version variable is set at link time
# via -ldflags="-X .../buildinfo.Version=$(cat VERSION)"; this
# script keeps the four static files in sync with that.

set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="$(cat VERSION)"
if [[ -z "$VERSION" ]]; then
  echo "VERSION file is empty" >&2
  exit 1
fi

# api/openapi.yaml: info.version: "..."
sed -i.bak -E "s|(version:[[:space:]]*\")[^\"]*(\")|\1${VERSION}\2|" api/openapi.yaml
rm -f api/openapi.yaml.bak

# sdk/python/src/promptsheon/__init__.py: __version__ = "..."
sed -i.bak -E "s|(__version__[[:space:]]*=[[:space:]]*\")[^\"]*(\")|\1${VERSION}\2|" \
    sdk/python/src/promptsheon/__init__.py
rm -f sdk/python/src/promptsheon/__init__.py.bak

# sdk/typescript/package.json: "version": "..."
sed -i.bak -E "s|(\"version\"[[:space:]]*:[[:space:]]*\")[^\"]*(\")|\1${VERSION}\2|" \
    sdk/typescript/package.json
rm -f sdk/typescript/package.json.bak

# deploy/helm/promptsheon/Chart.yaml: version: 0.1.0 (unquoted)
sed -i.bak -E "s|(^version:[[:space:]]*)[0-9]+\.[0-9]+\.[0-9]+|\1${VERSION}|" \
    deploy/helm/promptsheon/Chart.yaml
rm -f deploy/helm/promptsheon/Chart.yaml.bak

echo "synced version ${VERSION} to openapi.yaml, sdk/python, sdk/typescript, helm chart"
