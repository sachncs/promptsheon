#!/usr/bin/env bash
# scripts/stats.sh
# Regenerate the architecture numbers that appear in README.md and
# packages/server/README.md. Run before tagging a release.
set -euo pipefail

cd "$(dirname "$0")/.."

echo "=== promptsheon repo stats ==="
echo
echo "Packages:"
for d in packages/*/; do
  base=$(basename "$d")
  [ "$base" = "node_modules" ] && continue
  echo "  - packages/$base"
done
echo "  - frontend"
echo
echo "Top-level server modules:"
echo "  routes:     $(ls packages/server/src/routes | wc -l | tr -d ' ')"
echo "  middleware: $(ls packages/server/src/middleware | wc -l | tr -d ' ')"
echo "  repos:      $(ls packages/server/src/repos | wc -l | tr -d ' ')"
echo "  agents:     $(ls packages/server/src/agents | wc -l | tr -d ' ')"
echo "  scheduler:  $(ls packages/server/src/scheduler | wc -l | tr -d ' ')"
echo "  audit:      $(ls packages/server/src/audit | wc -l | tr -d ' ')"
echo "  sse:        $(ls packages/server/src/sse | wc -l | tr -d ' ')"
echo "  firewall:   $(ls packages/server/src/firewall 2>/dev/null | wc -l | tr -d ' ')"
echo "  policy:     $(ls packages/server/src/policy | wc -l | tr -d ' ')"
echo
echo "Tests:"
echo "  server test files: $(find packages/server/test -name '*.test.ts' | wc -l | tr -d ' ')"
echo "  server it() calls: $(grep -rh '^[[:space:]]*it(' packages/server/test/ 2>/dev/null | wc -l | tr -d ' ')"
echo "  shared test files: $(find packages/shared/test packages/shared/src -maxdepth 2 -name '*.test.ts' 2>/dev/null | wc -l | tr -d ' ')"
echo "  shared it() calls: $(grep -rh '^[[:space:]]*it(' packages/shared/test/ packages/shared/src/*.test.ts 2>/dev/null | wc -l | tr -d ' ')"
echo "  frontend e2e specs: $(find frontend/tests -name '*.spec.ts' 2>/dev/null | wc -l | tr -d ' ')"
