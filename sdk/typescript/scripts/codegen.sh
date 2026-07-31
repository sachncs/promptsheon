#!/usr/bin/env bash
# sdk/typescript/scripts/codegen.sh
#
# Generates sdk/typescript/src/openapi.ts and
# sdk/typescript/src/_generated/openapi.yaml from
# backend/spec/spec.yaml. CI verifies the SDK package compiles
# via `npm run test` and the snapshot yaml is in sync via
# `make sdk-check`.
#
# OS-5: the previous script only wrote src/openapi.ts. The
# snapshot yaml (used by the SDK runtime to discover routes)
# drifted to v0.2.0 while the canonical spec advanced to v0.3.0.
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p src/_generated
npx --yes openapi-typescript \
  ../../backend/spec/spec.yaml \
  --output src/openapi.ts
cp ../../backend/spec/spec.yaml src/_generated/openapi.yaml
npx tsc --noEmit
echo "codegen: ok"
