# Promptsheon Python SDK

Auto-generated Python client for the [Promptsheon](https://github.com/sachncs/promptsheon) v1 API.

The `src/promptsheon/` package is regenerated from
`backend/spec/spec.yaml` by `openapi-python-client`. The
generated API exposes a `Client` and an `AuthenticatedClient`
for every route in the spec — see the per-route modules under
`promptsheon.api.default.*` for the full surface.

## Usage

```python
from promptsheon import Client

with Client(base_url="https://api.promptsheon.example.com", token="<your API key>") as client:
    workspaces = client.api.list_workspaces()
```

## Development

Regenerate from the production OpenAPI spec:

```sh
cd sdk/python
python3 -m venv .venv && source .venv/bin/activate
pip install -e '.[codegen]'
bash scripts/codegen.sh   # regenerates src/promptsheon from ../../backend/spec/spec.yaml
python3 -m compileall -q src/promptsheon
python3 -m compileall -q tests
python3 -m pytest tests    # smoke test the package imports
```

If `bash scripts/codegen.sh` produces a diff the SDK is out of
sync with the daemon's OpenAPI spec. CI fails the build.
