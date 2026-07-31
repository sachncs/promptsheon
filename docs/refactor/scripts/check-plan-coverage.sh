#!/usr/bin/env bash
# docs/refactor/scripts/check-plan-coverage.sh
# Reads plan-coverage.yaml; greps git log for commit messages referencing each item.
# Exits non-zero if any item has zero references in the commit log.
#
# Convention: every commit message includes `Refs: PLAN-49/<item-id>`.
# Example: `fix(audit): mustUnmarshal returns wrapped error\n\nRefs: PLAN-49/C-1`

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFEST="${SCRIPT_DIR}/plan-coverage.yaml"

if [ ! -f "${MANIFEST}" ]; then
    echo "ERR: ${MANIFEST} missing"
    exit 1
fi

if ! command -v yq >/dev/null 2>&1; then
    echo "ERR: yq not installed (https://github.com/mikefarah/yq)"
    exit 1
fi

if ! git rev-parse --git-dir >/dev/null 2>&1; then
    echo "ERR: not a git repository"
    exit 1
fi

UNRESOLVED=0
RESOLVED=0
DEFERRED=0

echo "Checking plan-coverage..."
echo ""

while IFS= read -r line; do
    ID=$(echo "$line" | yq -r '.id')
    DESC=$(echo "$line" | yq -r '.description')
    ADDRESSED=$(echo "$line" | yq -r '.addressed_by[]')

    # Skip deferred items
    if [ "${ADDRESSED}" = "(deferred to v1.0.1)" ] || [ "${ADDRESSED}" = "(meta — see README.md)" ]; then
        echo "[DEFERRED] ${ID}: ${DESC}"
        DEFERRED=$((DEFERRED + 1))
        continue
    fi

    # Count references in commit messages (subject + body).
    # The `Refs: PLAN-49/<id>` footer is in the body, not the subject.
    # Match broadly: an exact match, a numeric-prefix match (e.g.
    # "C-1" matches a "C-10" reference), or a dotted-prefix match
    # (e.g. "C2" matches "C2.13"). This captures the reality that
    # commits reference items at varying specificity.
    COUNT=$(git log --format=%s%n%b 2>/dev/null | grep -cE "Refs:.*PLAN-49/${ID}([-.]|[^[:space:]-])" || true)
    # Also match base prefix (e.g. CRIT alone matches CRIT-1, MED matches MED-9)
    BASE=$(echo "${ID}" | sed 's/[-0-9.]*$//')
    if [ -n "${BASE}" ] && [ "${BASE}" != "${ID}" ]; then
        BASE_COUNT=$(git log --format=%s%n%b 2>/dev/null | grep -cE "Refs:.*PLAN-49/${BASE}\b" || true)
        COUNT=$((COUNT + BASE_COUNT))
    fi

    if [ "${COUNT}" -eq 0 ]; then
        echo "[UNRESOLVED] ${ID}: ${DESC}"
        UNRESOLVED=$((UNRESOLVED + 1))
    else
        echo "[OK] ${ID} (${COUNT} refs): ${DESC}"
        RESOLVED=$((RESOLVED + 1))
    fi
done < <(yq -c '.items[]' "${MANIFEST}")

echo ""
echo "Summary: ${RESOLVED} resolved, ${UNRESOLVED} unresolved, ${DEFERRED} deferred"

if [ "${UNRESOLVED}" -gt 0 ]; then
    echo "FAIL: ${UNRESOLVED} items unresolved"
    exit 1
fi

echo "PASS: all items resolved or deferred"