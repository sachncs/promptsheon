#!/usr/bin/env bash
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
      if (file ~ /\/internal\/api\//) package = "internal/api"
      else if (file ~ /\/internal\/store\//) package = "internal/store"
      else if (file ~ /\/internal\//) {
        split(file, parts, "/internal/")
        split(parts[2], name, "/")
        package = name[1]
      }
      if (package == "internal/api" || package == "internal/store") {
        total[package] += statements
        hit[package] += covered
      }
      if (file ~ /\/internal\/api\/handlers_[^\/]*\.go$/) {
        total["api handlers"] += statements
        hit["api handlers"] += covered
      }
      if (package != "internal/api" && package != "internal/store" && package != "") {
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
        floor = (package == "api handlers" ? 60 : (package == "internal/api" || package == "internal/store" ? 40 : 50))
        pct = 100 * hit[package] / total[package]
        printf "%s: %.2f%% (%d/%d statements, floor %d%%)\n", (pct >= floor ? "OK" : "FAIL"), pct, hit[package], total[package], floor
        if (pct < floor) failed = 1
      }
      for (i = 1; i <= 2; i++) {
        package = (i == 1 ? "internal/api" : "internal/store")
        if (total[package] == 0) {
          printf "FAIL: %s has no statements\n", package > "/dev/stderr"
          failed = 1
        }
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
github.com/sachncs/promptsheon/internal/release/release.go:1.1,2.1 5 1
github.com/sachncs/promptsheon/internal/release/release.go:3.1,4.1 15 0
github.com/sachncs/promptsheon/internal/optimizer/optimizer.go:1.1,2.1 10 1
github.com/sachncs/promptsheon/internal/optimizer/optimizer.go:3.1,4.1 10 1
github.com/sachncs/promptsheon/internal/api/server.go:1.1,2.1 10 1
github.com/sachncs/promptsheon/internal/store/sqlite.go:1.1,2.1 10 1
github.com/sachncs/promptsheon/internal/api/handlers_health.go:1.1,2.1 10 1
EOF
  if check_profile "$weak" >/dev/null 2>&1; then
    printf 'coverage self-test failed: weak package was hidden\n' >&2
    exit 1
  fi
  cat >"$pass" <<'EOF'
mode: atomic
github.com/sachncs/promptsheon/internal/release/release.go:1.1,2.1 10 1
github.com/sachncs/promptsheon/internal/optimizer/optimizer.go:1.1,2.1 10 1
github.com/sachncs/promptsheon/internal/api/server.go:1.1,2.1 10 1
github.com/sachncs/promptsheon/internal/store/sqlite.go:1.1,2.1 10 1
github.com/sachncs/promptsheon/internal/api/handlers_health.go:1.1,2.1 10 1
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
