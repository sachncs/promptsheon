#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
repo=github.com/sachncs/promptsheon
DOMAIN=(
  alerting approval audit auth budget capability eventbus executor experiment
  lineage observation optimizer policy quota recommendation release replay schedule
)
FORBIDDEN=("$repo/internal/store" "$repo/internal/llm" "$repo/internal/api" "$repo/cmd")
errors=0

for pkg in "${DOMAIN[@]}"; do
  dir="./internal/$pkg"
  [[ -d "${dir#./}" ]] || continue
  imports=$(go list -f '{{join .Imports "\n"}}' "$dir")
  for bad in "${FORBIDDEN[@]}"; do
    while IFS= read -r imported; do
      if [[ "$imported" == "$bad" || "$imported" == "$bad/"* ]]; then
        printf 'internal/%s imports forbidden dependency: %s\n' "$pkg" "$imported" >&2
        errors=$((errors + 1))
      fi
    done <<<"$imports"
  done
done

if (( errors )); then
  printf '%d forbidden dependencies detected\n' "$errors" >&2
  exit 1
fi
printf 'ok: domain package purity verified (%d packages)\n' "${#DOMAIN[@]}"
