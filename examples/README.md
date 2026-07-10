# Examples

Runnable examples for the Promptsheon SDKs and CLI. Each example
has its own directory with a `pyproject.toml` (Python) or
`package.json` (TypeScript / Node) and a README that documents
the environment variables required.

## Python

- [python-list-capabilities](python-list-capabilities/) — calls
  `list_capabilities` against a local daemon using
  `sdk/python`.

## TypeScript / Node

- (Tier 2.41 follow-on: shipped with the codegen pipeline.)

## Bash

- `bash/invoke-release.sh` — calls `POST /v1/releases/{r}/invoke`
  with a JSON body; useful for smoke-testing a daemon.

The SDK install paths assume the workspace is a monorepo (i.e.
you are running from the repo root). External consumers should
install the SDK from npm or PyPI once the production
publishing pipeline ships.
