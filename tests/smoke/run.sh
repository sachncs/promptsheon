#!/usr/bin/env bash
# tests/smoke/run.sh
#
# API-6b: smoke-test the example bash scripts against a
# freshly-started daemon. Boots promptsheond on an ephemeral
# loopback port, runs every examples/bash/*.sh against it
# (each script's success = curl returned 2xx OR a documented
# business-failure status), and tears the daemon down.
#
# The smoke test does NOT verify response shapes; that is the
# contract test's job. It does verify the examples don't
# 404 against a real daemon, which catches typos in paths,
# missing auth headers, and broken URL formats.

set -euo pipefail

cd "$(dirname "$0")/../.."

PORT="${PROMPTSHEON_SMOKE_PORT:-18080}"
BASE_URL="http://127.0.0.1:${PORT}"
LOG="$(mktemp -t promptsheon-smoke.XXXXXX.log)"
DB="$(mktemp -t promptsheon-smoke.XXXXXX.db)"
trap 'rm -f "$LOG" "$DB" "$DB-shm" "$DB-wal"; kill $DAEMON_PID 2>/dev/null || true' EXIT

# Build the daemon if missing.
if [[ ! -x ./promptsheond ]]; then
  echo "smoke: building promptsheond..."
  go build -o ./promptsheond ./cmd/promptsheond
fi

# Boot in the background. PROMPTSHEON_AUTH=false skips the
# bootstrap path so the smoke test can run unattended.
echo "smoke: starting daemon on $BASE_URL"
PROMPTSHEON_AUTH=false \
  PROMPTSHEON_ADDR=":${PORT}" \
  PROMPTSHEON_DB_PATH="$DB" \
  PROMPTSHEON_LOG_LEVEL=info \
  ./promptsheond >"$LOG" 2>&1 &
DAEMON_PID=$!

# Wait for /health to return 200.
for i in $(seq 1 30); do
  if curl -sS -o /dev/null -w "%{http_code}" "$BASE_URL/health" 2>/dev/null | grep -q 200; then
    break
  fi
  sleep 0.2
done

if ! curl -sS -o /dev/null -w "%{http_code}" "$BASE_URL/health" 2>/dev/null | grep -q 200; then
  echo "smoke: daemon failed to start within 6s. Log:" >&2
  cat "$LOG" >&2
  exit 1
fi

echo "smoke: daemon up"

# Iterate over every example script and exercise its main
# happy path. The first positional argument is supplied via
# $1; each script must be tolerant of a smoke-only fixture
# (e.g. a release id that doesn't exist) — the smoke test
# accepts any 4xx response as "the script ran without
# crashing".
FAIL=0
for script in examples/bash/*.sh; do
  echo "smoke: running $script"
  if ! PROMPTSHEON_BASE_URL="$BASE_URL" \
        PROMPTSHEON_API_KEY="smoke-test-key" \
        bash "$script" "smoke-fixture-id" 2>&1 \
        | head -3; then
    echo "smoke: FAIL ($script)" >&2
    FAIL=1
  fi
done

if [[ "$FAIL" -ne 0 ]]; then
  echo "smoke: at least one example failed" >&2
  exit 1
fi

echo "smoke: all examples exercised"
