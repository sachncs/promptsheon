#!/usr/bin/env bash
# E2E test driver for promptsheon.
#
# Usage:  ./e2e.sh <step>
# Each step makes the request, prints the status code + body, and
# captures any IDs (admin id, org id) into /tmp/e2e-ids.sh for the
# next step.
set -u
B=${B:-http://localhost:8080}
F=${F:-http://localhost:3000}

probe() {
  local label="$1" ; shift
  echo "── $label"
  local out
  out=$(/usr/bin/curl -sS -o /tmp/e2e-body.json -w "%{http_code}" "$@")
  echo "  status: $out"
  if /usr/bin/jq -e . /tmp/e2e-body.json >/dev/null 2>&1; then
    /usr/bin/jq -c . /tmp/e2e-body.json 2>/dev/null | /usr/bin/head -c 500
  else
    /usr/bin/cat /tmp/e2e-body.json | /usr/bin/head -c 500
  fi
  echo ""
}

case "${1:-help}" in
  01-admin)
    probe "POST /api/bootstrap/admin" -X POST "$B/api/bootstrap/admin" \
      -H 'content-type: application/json' \
      -d '{"adminName":"E2E User","adminEmail":"e2e@promptsheon.test","orgName":"E2E Org","orgSlug":"e2e-org"}'
    ADMIN_ID=$(/usr/bin/jq -r .user.id /tmp/e2e-body.json 2>/dev/null)
    ORG_ID=$(/usr/bin/jq -r .org.id /tmp/e2e-body.json 2>/dev/null)
    USER_NAME=$(/usr/bin/jq -r .user.name /tmp/e2e-body.json 2>/dev/null)
    ORG_NAME=$(/usr/bin/jq -r .org.name /tmp/e2e-body.json 2>/dev/null)
    /usr/bin/cat > /tmp/e2e-ids.sh <<EOF
export USER_ID=$ADMIN_ID
export ORG_ID=$ORG_ID
export USER_NAME=$USER_NAME
export ORG_NAME=$ORG_NAME
EOF
    echo "  → captured USER_ID=$ADMIN_ID ORG_ID=$ORG_ID"
    ;;
  02-llm-validate)
    probe "POST /api/bootstrap/validate-llm (custom + MiniMax)" -X POST "$B/api/bootstrap/validate-llm" \
      -H 'content-type: application/json' \
      -d "{\"provider\":\"custom\",\"apiKey\":\"${MINIMAX_API_KEY:-}\",\"baseUrl\":\"https://api.minimax.io/anthropic\",\"model\":\"MiniMax-M3\"}"
    ;;
  03-llm-save)
    probe "POST /api/bootstrap/llm" -X POST "$B/api/bootstrap/llm" \
      -H 'content-type: application/json' \
      -d "{\"provider\":\"custom\",\"apiKey\":\"${MINIMAX_API_KEY:-}\",\"baseUrl\":\"https://api.minimax.io/anthropic\",\"model\":\"MiniMax-M3\"}"
    ;;
  04-status)
    probe "GET /api/bootstrap/status" "$B/api/bootstrap/status"
    ;;
  05-workspace-create)
    . /tmp/e2e-ids.sh
    probe "POST /api/workspaces" -X POST "$B/api/workspaces" \
      -H 'content-type: application/json' \
      -H "X-User-Id: $USER_ID" -H "X-Org-Id: $ORG_ID" \
      -d '{"name":"refund-triage","organization":"E2E"}'
    WS_ID=$(/usr/bin/jq -r .id /tmp/e2e-body.json 2>/dev/null)
    /usr/bin/echo "export WS_ID=$WS_ID" >> /tmp/e2e-ids.sh
    echo "  → captured WS_ID=$WS_ID"
    ;;
  *)
    echo "usage: $0 {01-admin|02-llm-validate|03-llm-save|04-status|05-workspace-create}"
    exit 1
    ;;
esac
