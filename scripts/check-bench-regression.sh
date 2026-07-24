#!/usr/bin/env bash
# check-bench-regression.sh — fail the build if any benchmark
# regresses > 20% against the stored baseline.
#
# PERF-BENCH-2: the baseline lives in scripts/bench-baseline.txt in
# the format emitted by `go test -bench`. Each line is the bench
# name (relative path + benchmark name) followed by the ns/op.
# The script writes the new run to scripts/bench-current.txt so
# the diff can be eyeballed.
#
# Threshold: 20%. Above 20%, the job fails. Below 20%, the
# baseline is NOT updated automatically — the maintainer does
# that on purpose so a casual 5% speedup doesn't silently
# rewrite the production SLOs.
#
# Usage: scripts/check-bench-regression.sh [BENCHTIME]
#   BENCHTIME defaults to 1s; pass '100ms' for a fast smoke pass.
set -euo pipefail

cd "$(dirname "$0")/.."

benchtime="${1:-1s}"
baseline="scripts/bench-baseline.txt"
current="scripts/bench-current.txt"
threshold="20.0"

if [ ! -f "$baseline" ]; then
    echo "no baseline at $baseline; skipping regression check"
    exit 0
fi

# Run the curated benchmarks. The Makefile target already filters
# to the metrics that matter; here we reuse it.
make bench BENCHTIME="$benchtime" > "$current" 2>&1 || true

# Parse both files into a name -> ns/op map.
parse_bench() {
    awk '
        /Benchmark/ && /[0-9]+ ns\/op/ {
            for (i = 1; i <= NF; i++) {
                if ($i == "ns/op") {
                    name = $1
                    sub(/-[0-9]+$/, "", name)
                    ns = $(i-1)
                    gsub(",", "", ns)
                    print name " " ns
                    break
                }
            }
        }
    ' "$1" | sort -u
}

baseline_pairs=$(parse_bench "$baseline")
current_pairs=$(parse_bench "$current")

if [ -z "$current_pairs" ]; then
    echo "no benchmark output captured; check the bench output"
    head -50 "$current"
    exit 1
fi

# Compare each baseline entry to the current run.
regressed=0
while IFS=' ' read -r name base_ns; do
    cur_ns=$(echo "$current_pairs" | awk -v n="$name" '$1 == n {print $2; exit}')
    if [ -z "$cur_ns" ]; then
        echo "WARN: bench $name not in current run"
        continue
    fi
    # delta = (cur - base) / base * 100
    delta=$(awk -v b="$base_ns" -v c="$cur_ns" 'BEGIN { printf "%.2f", (c - b) / b * 100 }')
    pass=$(awk -v d="$delta" -v t="$threshold" 'BEGIN { print (d+0 <= t+0) ? 1 : 0 }')
    if [ "$pass" -eq 0 ]; then
        echo "FAIL: $name regressed ${delta}% (${base_ns} -> ${cur_ns} ns/op, threshold ${threshold}%)"
        regressed=1
    else
        echo "OK:   $name delta ${delta}%"
    fi
done <<< "$baseline_pairs"

if [ "$regressed" -ne 0 ]; then
    echo "bench regression exceeded ${threshold}%. See $current for the raw run."
    exit 1
fi

echo "no bench regressions beyond ${threshold}%"
