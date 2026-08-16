#!/usr/bin/env bash
#
# scripts/run-lint.sh — the LINT-1 lint gate.
#
# Replaces the broken golangci-lint run. Runs staticcheck ./... and
# diffs against scripts/lint-baseline.txt. New findings (anything
# not already in the baseline) fail the gate. The baseline is the
# list of known legacy findings the team has accepted; this gate
# enforces "no new lint debt" without forcing a giant cleanup PR.
#
# See docs/research/audit-fixes-plan.md PR 1 for the rationale.

set -euo pipefail

if ! command -v staticcheck >/dev/null 2>&1; then
  echo "staticcheck not installed; run:"
  echo "  go install honnef.co/go/tools/cmd/staticcheck@latest"
  exit 1
fi

# Capture current findings (one per line, sorted, deduplicated).
NEW=$(staticcheck ./... 2>&1 | sort -u || true)

# Load the baseline (if present).
BASE=""
if [ -f scripts/lint-baseline.txt ]; then
  BASE=$(sort -u scripts/lint-baseline.txt)
fi

# Compute the diff: anything in NEW but not in BASE.
DIFF=$(printf '%s\n' "$NEW" | grep -F -v -f <(printf '%s\n' "$BASE") || true)

if [ -n "$DIFF" ]; then
  echo "new staticcheck findings (not in scripts/lint-baseline.txt):"
  echo "$DIFF"
  echo
  echo "If these are expected, append them to scripts/lint-baseline.txt and commit."
  echo "If they are not, fix them."
  exit 1
fi

echo "ok: lint (staticcheck)"
