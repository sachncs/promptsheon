# Promptsheon Python SDK

Auto-generated Python client for the [Promptsheon](https://github.com/sachncs/promptsheon) v1 API.

## Usage

```python
from promptsheon import Client, ClientConfig

with Client(ClientConfig(
    base_url="https://api.promptsheon.example.com",
    api_key="<your API key>",
)) as client:
    capabilities = client.list_capabilities("project-1")
```

## Development

Regenerate from the production OpenAPI spec:

```sh
cd sdk/python
python3 -m venv .venv && source .venv/bin/activate
pip install -e '.[codegen]'
bash scripts/codegen.sh   # regenerates src/promptsheon from ../../api/openapi.yaml
python3 -m compileall src/promptsheon tests
```

Today `src/promptsheon/client.py` is a hand-written scaffold
covering the public-resource list in the architecture review
(§7: listCapabilities, invokeRelease). The M3 follow-on commit
regenerates against the production spec once it covers every v1
resource.
