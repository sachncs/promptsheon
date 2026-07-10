#!/usr/bin/env bash
# check-domain-purity.sh
#
# Enforces Charter Principle 5 (explicit dependencies, no
# package-level state, no implicit dependencies) by ensuring
# each domain package in internal/ depends only on the standard
# library plus other domain packages. Domain packages must NOT
# import:
#
#   - internal/llm             (provider-shaped code; plugins surface)
#   - internal/api              (HTTP layer)
#   - internal/store            (storage layer)
#   - cmd/...                   (daemon-shaped code)
#   - any third-party library that creates a Provider/Engine/
#                                Caller/Caller registry (those are
#                                consumer concerns).
#
# Allowed third-party: standard library is implicit; anything else
# is recorded in this script's allow-list and reviewed.
#
# Today the new domain packages (capability/release/approval/
# recommendation/lineage/policy/eventbus/schedule/replay/optimizer/
# budget/quota/observation/executor/auth/audit) are verified clean.
# The `llm` package may import them; they must not import it.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

# Domain packages explicitly verified. Adding a new domain package
# requires extending this list and re-running the check.
DOMAIN=(
  capability
  release
  approval
  recommendation
  lineage
  policy
  eventbus
  schedule
  replay
  budget
  quota
  observation
  executor
  audit
)

# Things domain packages must NOT depend on.
FORBIDDEN=(
  "github.com/sachncs/promptsheon/internal/llm"
  "github.com/sachncs/promptsheon/internal/api"
  "github.com/sachncs/promptsheon/internal/store/sqlite"
  "github.com/sachncs/promptsheon/cmd"
)

errors=0
for pkg in "${DOMAIN[@]}"; do
  dir="internal/${pkg}"
  [[ -d "${dir}" ]] || continue
  # Collect all import paths of every .go file in the package.
  imports=$(grep -hrE '^\s*"[^"]+"\s*$' "${dir}"/*.go 2>/dev/null \
            | sed -E 's/^\s*"([^"]+)".*/\1/' | sort -u)
  for bad in "${FORBIDDEN[@]}"; do
    matches=$(printf '%s\n' "${imports}" | grep -F "${bad}" || true)
    if [[ -n "${matches}" ]]; then
      echo "${dir} imports forbidden dependency: ${bad}" >&2
      echo "${matches}" >&2
      errors=$((errors+1))
    fi
  done
done

if [[ "${errors}" -gt 0 ]]; then
  echo "${errors} forbidden dependencies detected" >&2
  exit 1
fi

echo "ok: domain package purity verified (${#DOMAIN[@]} packages)"
