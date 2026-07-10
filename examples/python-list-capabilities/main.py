"""
Example: list the Capabilities in a Project using the Promptsheon
Python SDK.

This is a reference example for the architecture review's
Tier 2.39 Python SDK scaffold. Run it against a local daemon:

    PROMPTSHEON_BASE_URL=http://localhost:8080 \\
    PROMPTSHEON_API_KEY=$PROMPTSHEON_TOKEN \\
    python3 examples/python-list-capabilities/main.py
"""
from __future__ import annotations

import os
import sys

from promptsheon import Client, ClientConfig, PromptsheonAPIError


def main() -> int:
    base_url = os.environ.get("PROMPTSHEON_BASE_URL", "http://localhost:8080")
    api_key = os.environ.get("PROMPTSHEON_API_KEY", "")
    if not api_key:
        print("PROMPTSHEON_API_KEY is required; export it before running.", file=sys.stderr)
        return 1

    project_id = sys.argv[1] if len(sys.argv) > 1 else "project-1"
    cfg = ClientConfig(base_url=base_url, api_key=api_key)
    with Client(cfg) as client:
        try:
            capabilities = client.list_capabilities(project_id)
        except PromptsheonAPIError as e:
            print(f"API error: {e}", file=sys.stderr)
            return 2

    print(f"Project {project_id!r}: {len(capabilities)} capability(ies)")
    for c in capabilities:
        name = c.get("name") if isinstance(c, dict) else getattr(c, "name", "?")
        cid = c.get("id") if isinstance(c, dict) else getattr(c, "id", "?")
        print(f"  - {cid}  {name}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
