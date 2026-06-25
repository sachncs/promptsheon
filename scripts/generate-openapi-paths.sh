#!/usr/bin/env bash
#
# scripts/generate-openapi-paths.sh
#
# Closes the M-15 gap by appending skeleton path entries to
# api/openapi.yaml for every route registered in
# internal/api/server.go that is not already in the spec.
#
# Each entry is a minimal stub (summary + 200 response) that
# documents the route exists and its HTTP method, without
# duplicating request/response schemas that operators should
# hand-craft or generate from code annotations.
#
# This script is idempotent: running it twice does not duplicate
# entries.

set -euo pipefail

cd "$(dirname "$0")/.."

# Extract every (method, path) pair registered in server.go.
grep -oE '"[A-Z]+ /[^"]+"' internal/api/server.go | tr -d '"' | sort -u > /tmp/_openapi_routes.txt

# Extract every path already in the spec.
grep -E "^  /" api/openapi.yaml | tr -d ' ' | tr -d ':' | sort -u > /tmp/_openapi_existing.txt

added=0
while IFS=' ' read -r method path; do
    if grep -qxF "$path" /tmp/_openapi_existing.txt; then
        continue
    fi
    {
        echo "  $path:"
        echo "    $method:"
        echo "      summary: \"TODO: M-15 generated stub, flesh out\""
        echo "      responses:"
        echo "        \"200\":"
        echo "          description: \"OK\""
    } >> api/openapi.yaml
    added=$((added + 1))
done < /tmp/_openapi_routes.txt

echo "M-15: appended $added missing route(s) to api/openapi.yaml"
