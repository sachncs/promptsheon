#!/usr/bin/env bash
# sdk/typescript/scripts/codegen.sh
#
# Generates sdk/typescript/src/openapi.ts from
# backend/spec/spec.yaml using openapi-typescript. Run this
# whenever the spec changes. CI verifies the SDK package
# compiles via `npm run test`.
set -euo pipefail
cd "$(dirname "$0")/.."
npx --yes openapi-typescript \
  ../../backend/spec/spec.yaml \
  --output src/openapi.ts
npx tsc --noEmit
echo "codegen: ok"
