#!/usr/bin/env bash
# check-coverage.sh
#
# Reads a Go coverage profile (cover.out) and enforces per-package
# coverage floors on the production daemon. The buckets are aligned
# with the current `promptsheon/` package layout (post-Phase-1 of
# the compliance refactor; the previous `backend/` paths were
# removed when the package was renamed).
#
# Buckets:
#
#   promptsheon/<domain>            floor 50%
#   promptsheon (root pkg)          floor 40%
#   promptsheon/store               floor 40%
#   promptsheon/handlers_*.go files floor 60%
#
# Domain packages get a 50% floor; core wiring gets a 40% floor;
# HTTP handlers get a 60% floor. The floors are intentionally
# conservative; tighten in a follow-up if the project outgrows
# them.
#
# Usage:
#   check-coverage.sh [--self-test] [PROFILE]
#
#   --self-test  run a synthetic positive + negative pair against
#                the AWK parser and exit non-zero if the parser
#                accepts the weak profile or rejects the strong one.
#   PROFILE      path to a Go coverage profile (default: coverage.out).
set -euo pipefail

check_profile() {
  awk -v domain_packages="alerting approval audit auth budget capability eventbus executor experiment lineage observation optimizer policy quota recommendation release replay schedule" '
    NR == 1 { next }
    {
      file = $1
      sub(/:[0-9].*$/, "", file)
      statements = $(NF-1)
      covered = ($NF > 0 ? statements : 0)
      package = ""
      if (file ~ /\/promptsheon\/store\//) package = "promptsheon/store"
      else if (file ~ /\/promptsheon\/[^\/]+\.go$/) {
        # Direct file in promptsheon/, e.g. promptsheon/server.go.
        package = "promptsheon"
      }
      else if (file ~ /\/promptsheon\//) {
        split(file, parts, "/promptsheon/")
        split(parts[2], name, "/")
        package = name[1]
      }
      if (package == "promptsheon" || package == "promptsheon/store") {
        total[package] += statements
        hit[package] += covered
      }
      if (file ~ /\/promptsheon\/handlers_[^\/]*\.go$/) {
        total["api handlers"] += statements
        hit["api handlers"] += covered
      }
      if (package != "promptsheon" && package != "promptsheon/store" && package != "") {
        wanted = " " package " "
        if (index(" " domain_packages " ", wanted)) {
          total[package] += statements
          hit[package] += covered
        }
      }
    }
    END {
      failed = 0
      for (package in total) {
        floor = (package == "api handlers" ? 60 : (package == "promptsheon" || package == "promptsheon/store" ? 40 : 50))
        pct = 100 * hit[package] / total[package]
        printf "%s: %s: %.2f%% (%d/%d statements, floor %d%%)\n", (pct >= floor ? "OK" : "FAIL"), package, pct, hit[package], total[package], floor
        if (pct < floor) failed = 1
      }
      if (total["promptsheon"] == 0) {
        printf "FAIL: promptsheon has no statements\n" > "/dev/stderr"
        failed = 1
      }
      if (total["promptsheon/store"] == 0) {
        printf "FAIL: promptsheon/store has no statements\n" > "/dev/stderr"
        failed = 1
      }
      if (total["api handlers"] == 0) {
        printf "FAIL: api handlers has no statements\n" > "/dev/stderr"
        failed = 1
      }
      exit failed
    }
  ' "$1"
}

if [[ "${1:-}" == "--self-test" ]]; then
  weak=$(mktemp)
  pass=$(mktemp)
  trap 'rm -f "$weak" "$pass"' EXIT
  cat >"$weak" <<'EOF'
mode: atomic
github.com/sachncs/promptsheon/promptsheon/release/release.go:1.1,2.1 5 1
github.com/sachncs/promptsheon/promptsheon/release/release.go:3.1,4.1 15 0
github.com/sachncs/promptsheon/promptsheon/optimizer/optimizer.go:1.1,2.1 10 1
github.com/sachncs/promptsheon/promptsheon/optimizer/optimizer.go:3.1,4.1 10 1
github.com/sachncs/promptsheon/promptsheon/server.go:1.1,2.1 5 1
github.com/sachncs/promptsheon/promptsheon/server.go:3.1,4.1 5 0
github.com/sachncs/promptsheon/promptsheon/store/sqlite.go:1.1,2.1 10 1
github.com/sachncs/promptsheon/promptsheon/handlers_health.go:1.1,2.1 10 1
EOF
  if check_profile "$weak" >/dev/null 2>&1; then
    printf 'coverage self-test failed: weak package was hidden\n' >&2
    exit 1
  fi
  cat >"$pass" <<'EOF'
mode: atomic
github.com/sachncs/promptsheon/promptsheon/release/release.go:1.1,2.1 10 1
github.com/sachncs/promptsheon/promptsheon/optimizer/optimizer.go:1.1,2.1 10 1
github.com/sachncs/promptsheon/promptsheon/server.go:1.1,2.1 10 1
github.com/sachncs/promptsheon/promptsheon/store/sqlite.go:1.1,2.1 10 1
github.com/sachncs/promptsheon/promptsheon/handlers_health.go:1.1,2.1 10 1
EOF
  check_profile "$pass" >/dev/null
  printf 'ok: coverage profile parser self-test\n'
  exit
fi

profile=${1:-coverage.out}
if [[ ! -f "$profile" ]]; then
  printf 'check-coverage: %s not found\n' "$profile" >&2
  exit 1
fi
check_profile "$profile"
