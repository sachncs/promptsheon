#!/usr/bin/env bash
# sdk/python/scripts/codegen.sh
#
# Regenerates sdk/python/src/promptsheon/ from api/openapi.yaml
# using openapi-python-client. Run this whenever the spec changes.
# CI verifies the package imports cleanly via 'python -m compileall'.
set -euo pipefail
cd "$(dirname "$0")/.."
if [[ "${OPENAPI_GENERATE:-1}" == "0" ]]; then
  echo "OPENAPI_GENERATE=0; skipping."
  exit 0
fi
python3 -m pip install --quiet "openapi-python-client>=0.21"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
python3 -m openapi_python_client generate \
  --path ../../api/openapi.yaml \
  --output-path "$tmp/promptsheon" \
  --overwrite
python3 -m compileall -q "$tmp/promptsheon"
python3 -m compileall -q tests
echo "codegen: ok"
