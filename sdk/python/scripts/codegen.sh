#!/usr/bin/env bash
# sdk/python/scripts/codegen.sh
#
# Regenerates sdk/python/src/promptsheon/ and the _generated
# snapshot yaml from backend/spec/spec.yaml using
# openapi-python-client. Run this whenever the spec changes.
# CI verifies the package imports cleanly via 'python -m compileall'.
#
# OS-6: the previous script wrote to a tmp dir and never touched
# sdk/python/src/promptsheon/ — that directory was a hand-kept
# copy that drifted from the canonical spec. Now writes directly
# to the source tree and commits any diff.
set -euo pipefail
cd "$(dirname "$0")/.."
if [[ "${OPENAPI_GENERATE:-1}" == "0" ]]; then
  echo "OPENAPI_GENERATE=0; skipping."
  exit 0
fi
python3 -m pip install --quiet "openapi-python-client>=0.21"
python3 -m openapi_python_client generate \
  --path ../../backend/spec/spec.yaml \
  --output-path src/promptsheon \
  --overwrite
mkdir -p src/promptsheon/_generated
cp ../../backend/spec/spec.yaml src/promptsheon/_generated/openapi.yaml
python3 -m compileall -q src/promptsheon
python3 -m compileall -q tests
echo "codegen: ok"
