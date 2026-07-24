#!/usr/bin/env bash
set -euo pipefail

list=${1:-scripts/benchmarks.txt}
output=${BENCH_OUTPUT:-bench.txt}
benchmarks=()
while IFS= read -r benchmark; do
  benchmarks+=("$benchmark")
done < <(awk 'NF && $1 !~ /^#/ { print $1 }' "$list")
if (( ${#benchmarks[@]} != 8 )); then
  printf 'benchmark list must contain exactly 8 names, got %d\n' "${#benchmarks[@]}" >&2
  exit 1
fi
if (( $(printf '%s\n' "${benchmarks[@]}" | sort -u | wc -l) != 8 )); then
  printf 'benchmark list contains duplicates\n' >&2
  exit 1
fi
pattern="^($(IFS='|'; printf '%s' "${benchmarks[*]}"))$"
go test -run='^$' -bench="$pattern" -benchmem -benchtime="${BENCHTIME:-1s}" ./... | tee "$output"
for benchmark in "${benchmarks[@]}"; do
  count=$(awk -v name="$benchmark" '$1 ~ ("^" name "-[0-9]+$") { n++ } END { print n+0 }' "$output")
  if (( count != 1 )); then
    printf '%s executed %d times; expected exactly once\n' "$benchmark" "$count" >&2
    exit 1
  fi
done
printf 'ok: exactly 8 curated benchmarks executed\n'
