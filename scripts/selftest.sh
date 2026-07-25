#!/usr/bin/env bash
# scripts/selftest.sh — end-to-end smoke test for the
# closed-loop self-evolution path. Boots the daemon with
# PROMPTSHEON_SELF_EVOLVE set against a deliberately-bad
# prompt, polls the audit chain and the metrics endpoint,
# and asserts the cycle ran.
#
# Usage: scripts/selftest.sh
#
# Requires the daemon binary to be built:
#   go build -o promptsheond ./cmd/promptsheond
# and the promptsheon CLI:
#   go build -o promptsheon ./cmd/promptsheon
# Both are looked up via PATH; fall back to ./promptsheond
# and ./cmd/promptsheond/.

set -euo pipefail

DAEMON_BIN="${DAEMON_BIN:-./promptsheond}"
CLI_BIN="${CLI_BIN:-./promptsheon}"
PORT="${SELFTEST_PORT:-18099}"
DB_PATH="$(mktemp -t promptsheon-selftest.XXXXXX.db)"
DAEMON_LOG="$(mktemp -t promptsheon-selftest.XXXXXX.log)"
ADMIN_KEY=""

cleanup() {
  if [[ -n "${DAEMON_PID:-}" ]]; then
    kill "$DAEMON_PID" 2>/dev/null || true
    wait "$DAEMON_PID" 2>/dev/null || true
  fi
  rm -f "$DB_PATH" "$DB_PATH-shm" "$DB_PATH-wal"
}
trap cleanup EXIT

echo "selftest: using db=$DB_PATH port=$PORT"

# Load env (api key + provider config)
if [[ -f .env ]]; then
  set -a; . ./.env; set +a
fi

# Boot daemon
PROMPTSHEON_ADDR="127.0.0.1:$PORT" \
PROMPTSHEON_DB_PATH="$DB_PATH" \
PROMPTSHEON_AUTH=true \
PROMPTSHEON_BOOTSTRAP_TOKEN="selftest-bootstrap-secret" \
PROMPTSHEON_LOG_LEVEL=info \
PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS=true \
PROMPTSHEON_HARNESS_PRECONDITIONS=false \
PROMPTSHEON_SELF_EVOLVE="" \
"$DAEMON_BIN" > "$DAEMON_LOG" 2>&1 &
DAEMON_PID=$!

# Wait for /health
for i in {1..30}; do
  if curl -fsS "http://127.0.0.1:$PORT/health" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done
if ! curl -fsS "http://127.0.0.1:$PORT/health" >/dev/null; then
  echo "selftest: daemon failed to start" >&2
  cat "$DAEMON_LOG" >&2
  exit 1
fi

# Bootstrap admin
ADMIN_KEY=$(curl -fsS -X POST -H "X-Bootstrap-Token: selftest-bootstrap-secret" \
  "http://127.0.0.1:$PORT/api/v1/setup" -d '{}' -H "Content-Type: application/json" \
  | grep -o '"key":"[^"]*' | cut -d'"' -f4)
if [[ -z "$ADMIN_KEY" ]]; then
  echo "selftest: bootstrap failed" >&2
  exit 1
fi
H="Authorization: Bearer $ADMIN_KEY"
BASE="http://127.0.0.1:$PORT"

# Seed workspace, project, capability with a deliberately-bad prompt
WS=$(curl -fsS -X POST -H "$H" -H "Content-Type: application/json" \
  -d '{"name":"selftest"}' "$BASE/api/v1/workspaces" | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4)
PROJ=$(curl -fsS -X POST -H "$H" -H "Content-Type: application/json" \
  -d "{\"name\":\"proj\"}" "$BASE/api/v1/workspaces/$WS/projects" | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4)

# Bad prompt that won't match "pong"
BAD_PROMPT="audit code"
BAD_HASH=$("$CLI_BIN" write-object "$BAD_PROMPT" | head -1)

CAP=$(curl -fsS -X POST -H "$H" -H "Content-Type: application/json" \
  -d "{\"name\":\"selftest-cap\"}" "$BASE/api/v1/projects/$PROJ/capabilities" | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4)
M27_HASH=$("$CLI_BIN" hash-object "m27" 2>/dev/null || echo "m27hash")
RT_HASH=$("$CLI_BIN" hash-object "rt" 2>/dev/null || echo "rthash")
VER=$(curl -fsS -X POST -H "$H" -H "Content-Type: application/json" \
  -d "{\"version\":1,\"manifest\":{\"prompt\":{\"kind\":\"prompt\",\"hash\":\"$BAD_HASH\"},\"model_policy\":{\"kind\":\"model_policy\",\"hash\":\"$BAD_HASH\"},\"runtime_policy\":{\"kind\":\"runtime_policy\",\"hash\":\"$BAD_HASH\"},\"context_contract\":{\"kind\":\"context_contract\",\"hash\":\"$BAD_HASH\"},\"memory\":{\"kind\":\"memory\",\"hash\":\"$BAD_HASH\"}}}" \
  "$BASE/api/v1/capabilities/$CAP/versions" | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4)

# Create a Bob user for the second vote
BOB=$(curl -fsS -X POST -H "$H" -H "Content-Type: application/json" \
  -d '{"email":"bob@selftest","name":"bob","role":"admin"}' \
  "$BASE/api/v1/users" | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4)
BOBKEY=$(curl -fsS -X POST -H "$H" -H "Content-Type: application/json" \
  -d "{\"name\":\"bob\",\"user_id\":\"$BOB\",\"role\":\"admin\"}" \
  "$BASE/api/v1/apikeys" | grep -o '"key":"[^"]*' | head -1 | cut -d'"' -f4)

REL=$(curl -fsS -X POST -H "$H" -H "Content-Type: application/json" \
  -d '{"environment":"dev"}' "$BASE/api/v1/versions/$VER/releases" | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4)
curl -fsS -X POST -H "Authorization: Bearer $BOBKEY" -H "Content-Type: application/json" \
  -d '{"decision":"approve"}' "$BASE/api/v1/releases/$REL/votes" >/dev/null
curl -fsS -X POST -H "Authorization: Bearer $BOBKEY" "$BASE/api/v1/releases/$REL/activate" >/dev/null

# Build a "expect pong" dataset
DS=$(curl -fsS -X POST -H "$H" -H "Content-Type: application/json" \
  -d '{"name":"ds","description":"expect pong","cases":[
    {"inputs":{"q":"ping"},"expected":"pong"},
    {"inputs":{"q":"hi"},"expected":"pong"}
  ]}' \
  "$BASE/api/v1/capabilities/$CAP/datasets" | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4)

# Baseline: a failing eval (0/2)
curl -fsS -X POST -H "$H" -H "Content-Type: application/json" \
  -d "{\"dataset_id\":\"$DS\",\"scorer\":\"contains\"}" \
  "$BASE/api/v1/releases/$REL/evals" >/dev/null

# Enable self-evolve via the CLI
"$CLI_BIN" selfevolve enable "$CAP" --dataset "$DS" --min-score 0.5 --max-revisions 3 --cooldown-sec 5

# Restart daemon with PROMPTSHEON_SELF_EVOLVE so the loop actually runs
kill "$DAEMON_PID" 2>/dev/null || true
wait "$DAEMON_PID" 2>/dev/null || true
PROMPTSHEON_ADDR="127.0.0.1:$PORT" \
PROMPTSHEON_DB_PATH="$DB_PATH" \
PROMPTSHEON_AUTH=true \
PROMPTSHEON_BOOTSTRAP_TOKEN="selftest-bootstrap-secret" \
PROMPTSHEON_LOG_LEVEL=info \
PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS=true \
PROMPTSHEON_HARNESS_PRECONDITIONS=false \
PROMPTSHEON_SELF_EVOLVE="$CAP:$DS:0.5:dev:3:5" \
PROMPTSHEON_SELF_EVOLVE_MODEL="MiniMax-M2.7" \
"$DAEMON_BIN" > "$DAEMON_LOG" 2>&1 &
DAEMON_PID=$!

# Wait for /health
for i in {1..30}; do
  if curl -fsS "http://127.0.0.1:$PORT/health" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done
if ! curl -fsS "http://127.0.0.1:$PORT/health" >/dev/null; then
  echo "selftest: daemon restart failed" >&2
  cat "$DAEMON_LOG" >&2
  exit 1
fi

# Wait up to 90s for the cycle to either promote or reject
echo "selftest: waiting for cycle to complete..."
for i in {1..90}; do
  STATUS=$(sqlite3 "$DB_PATH" "SELECT last_status FROM self_evolve_state WHERE capability_id='$CAP';" 2>/dev/null || echo "")
  if [[ "$STATUS" == "promoted" || "$STATUS" == "rejected" ]]; then
    break
  fi
  sleep 1
done
STATUS=$(sqlite3 "$DB_PATH" "SELECT last_status FROM self_evolve_state WHERE capability_id='$CAP';" 2>/dev/null || echo "")
echo "selftest: final state = $STATUS"
if [[ "$STATUS" != "promoted" && "$STATUS" != "rejected" ]]; then
  echo "selftest: cycle did not complete" >&2
  cat "$DAEMON_LOG" >&2
  exit 1
fi

# Audit chain must contain the actions
AUDIT_COUNT=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM audit_entries WHERE action LIKE 'self_evolve.%';")
echo "selftest: self_evolve audit rows = $AUDIT_COUNT"
if [[ "$AUDIT_COUNT" -lt 1 ]]; then
  echo "selftest: no self_evolve audit rows" >&2
  exit 1
fi

# Metrics endpoint must report the counters
METRICS=$(curl -fsS "http://127.0.0.1:$PORT/metrics" || true)
if ! echo "$METRICS" | grep -q "^promptsheon_self_evolve_runs_total "; then
  echo "selftest: promptsheon_self_evolve_runs_total not exposed" >&2
  exit 1
fi
echo "$METRICS" | grep "^promptsheon_self_evolve"

echo "selftest: PASSED"
