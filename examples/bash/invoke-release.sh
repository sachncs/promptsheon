#!/usr/bin/env bash
# examples/bash/invoke-release.sh
#
# Smoke-test a daemon by invoking a Release via curl. The script
# exports PROMPTSHEON_BASE_URL and PROMPTSHEON_API_KEY, takes the
# release ID as $1, sends a small JSON body, and prints the
# response. Useful for Tier 1.04 end-to-end smoke testing of the
# invoke path.
#
# API-6: the previous version posted to
# /v1/releases/$RELEASE_ID/invoke, which is not a registered
# route. The actual handler is /api/v1/releases/{id}/invoke.
# The version+model+provider body fields are required by the
# invoke handler — see handlers_releases.go.

set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <release-id> [model] [provider]" >&2
  echo "  e.g. $0 rel-1 gpt-4o-mini openai" >&2
  exit 2
fi

RELEASE_ID="$1"
MODEL="${2:-gpt-4o-mini}"
PROVIDER="${3:-openai}"

BASE_URL="${PROMPTSHEON_BASE_URL:-http://localhost:8080}"
API_KEY="${PROMPTSHEON_API_KEY:-}"

if [[ -z "$API_KEY" ]]; then
  echo "PROMPTSHEON_API_KEY is required" >&2
  exit 2
fi

BODY=$(printf '{"inputs": {"q": "hello"}, "model": "%s", "provider": "%s"}' "$MODEL" "$PROVIDER")

curl -sS -X POST \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d "$BODY" \
  "$BASE_URL/api/v1/releases/$RELEASE_ID/invoke"
echo
