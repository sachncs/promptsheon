#!/usr/bin/env bash
# TEST-COV-2: per-package coverage floors. Reads coverage.out
# (the same file the global-coverage check uses) and enforces
# three categories:
#
#   - domain packages (internal/<pkg>)     >= 50%
#   - infrastructure (internal/api+store)    >= 40%
#   - api handlers (internal/api handlers)    >= 60%
#
# The categories are a coarse-grained proxy for "high
# blast-radius, must be tested". A package below its floor
# is a regression worth investigating; the floor is low
# enough that newly-added code is allowed a learning curve
# before tripping the gate.
#
# Usage: bash scripts/check-coverage.sh coverage.out
set -e

COVERAGE_FILE="${1:-coverage.out}"
if [[ ! -f "$COVERAGE_FILE" ]]; then
  echo "check-coverage: $COVERAGE_FILE not found" >&2
  exit 1
fi

# Extract the per-package coverage from go tool cover -func
# output. The lines look like:
#   github.com/sachncs/promptsheon/internal/foo/bar.go:42: Bar 75.0%
# We aggregate by package path and report the percentage of
# statements covered.
declare -A pkg_total
declare -A pkg_covered
while IFS=':' read -r file line func pct; do
  # The pct field is "75.0%". Strip the % and convert to a
  # percentage (we'll accumulate the values directly; the
  # tool output is already a percentage per file, not per
  # statement).
  if [[ "$pct" =~ ([0-9.]+)% ]]; then
    p=${BASH_REMATCH[1]}
  else
    continue
  fi
  # Extract the package path (everything before the first
  # "/internal/" + suffix).
  if [[ "$file" =~ github.com/sachncs/promptsheon/(.+)\.go$ ]]; then
    pkg=${BASH_REMATCH[1]}
  else
    continue
  fi
  pkg_total[$pkg]=$((${pkg_total[$pkg]:-0} + 1))
  pkg_covered[$pkg]=$(echo "${pkg_covered[$pkg]:-0} + $p" | bc -l)
done < <(go tool cover -func="$COVERAGE_FILE" 2>/dev/null | grep -E '\.go:[0-9]+:' || true)

# Compute per-package average.
fail=0
declare -a failed_pkgs
for pkg in "${!pkg_total[@]}"; do
  total=${pkg_total[$pkg]}
  sum=${pkg_covered[$pkg]}
  if [[ "$total" -eq 0 ]]; then continue; fi
  avg=$(echo "scale=2; $sum / $total" | bc -l)
  # Classify.
  floor=0
  case "$pkg" in
    internal/api/handlers*) floor=60 ;;
    internal/api/server*) floor=60 ;;
    internal/api/pagination*|internal/api/validate*) floor=60 ;;
    internal/store/*) floor=40 ;;
    internal/api/*) floor=40 ;;
    internal/*) floor=50 ;;
  esac
  if [[ "$floor" -gt 0 ]] && [ $(echo "$avg < $floor" | bc -l) -eq 1 ]; then
    echo "FAIL: $pkg coverage $avg% < $floor% floor" >&2
    failed_pkgs+=("$pkg: $avg% < $floor%")
    fail=1
  else
    echo "OK:   $pkg $avg% (floor $floor%)"
  fi
done

if [[ "$fail" -ne 0 ]]; then
  echo "" >&2
  echo "per-package coverage gate failed:" >&2
  for line in "${failed_pkgs[@]}"; do
    echo "  $line" >&2
  done
  exit 1
fi
