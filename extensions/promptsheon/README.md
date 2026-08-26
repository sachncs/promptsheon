# promptsheon VS Code extension

Author, validate, and ship prompt manifests from VS Code.

## What it does

- **Validate-on-save.** When you open or save a `.promptsheon.json` (or `.yaml`/`.yml`) file, the extension runs a local Zod pass against the shared `ManifestSchema`, then POSTs the body to the server's `/api/manifests/validate` endpoint for the canonical verdict. Issues surface as VS Code diagnostics on the file.
- **Hover docs.** Hover over `Planner`, `Agent`, `Tool`, or `Guardrail` in a manifest to see the node's purpose and the object shape it expects.
- **Send to Playground.** Run the `promptsheon: Send to Playground` command from the command palette to open `/app/playground` with the current manifest pre-loaded via the `manifest=` query parameter.

## Settings

| Key | Default | Purpose |
|---|---|---|
| `promptsheon.apiUrl` | `http://127.0.0.1:8080` | Base URL of the promptsheon server (used by the validator + the playground launcher). |
| `promptsheon.apiKey` | _(unset)_ | Optional org-scoped bearer token sent in the `authorization` header. |
| `promptsheon.playgroundUrl` | `http://127.0.0.1:8080` | Override for the playground URL used by `Send to Playground`. |

## Commands

| Command | Purpose |
|---|---|
| `promptsheon.validate` | Force a validation pass on the active editor's manifest. |
| `promptsheon.sendToPlayground` | Open the playground with the current manifest pre-loaded. |

## Status: v0.1

- JSON manifests in v1; YAML parsing requires the JSON-form conversion tool (planned).
- The extension does not depend on `vscode-languageserver` yet — diagnostics appear on save/open rather than continuously as you type. A proper language server is the next iteration.
- Tests cover the pure validation logic (`extensions/promptsheon/test/validate.test.ts`); the VS Code host surfaces are exercised by `@vscode/test-electron` in a future iteration.

## Development

```bash
cd extensions/promptsheon
pnpm install
pnpm typecheck
pnpm test
```

Build the VSIX:

```bash
pnpm package
```