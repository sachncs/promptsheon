# Promptsheon Console

Single-page control plane for the [Promptsheon](https://github.com/sachncs/promptsheon)
daemon. Hash-routed SPA (`#/`, `#/capabilities/{id}`, `#/releases`, `#/audit`,
`#/observability`, `#/guardrails`, `#/evaluations`, `#/operations/{tab}`, `#/logs`)
that calls the same REST API the Go SDK ships against.

## Develop

```bash
# 1. Boot the daemon (any address on the loopback; see notes below).
PROMPTSHEON_AUTH=false \
PROMPTSHEON_ADDR=127.0.0.1:8080 \
PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS=true \
../promptsheond

# 2. Boot the console.
npm install
npm run dev          # http://localhost:4173
```

First load triggers a **Connect** modal:

- **Bootstrap now** — `POST /api/v1/setup`. Works only when the daemon runs with
  `PROMPTSHEON_AUTH=false` or `PROMPTSHEON_BOOTSTRAP_TOKEN` is set. The returned
  `key` is stored in `localStorage` and used for every subsequent request.
- **Save and reload** — paste an existing `ps_…` key, then submit.

The proxy targets `localhost:8080`; if your daemon binds to a different loopback
address (e.g. only `[::1]`), use the **API base URL** field in **Connection** to
point at the explicit URL. The dashboard will then bypass the dev proxy.

## Production build

```bash
npm run build   # → web/dist
```

`dist/` is a static bundle (HTML + one CSS + one JS, gzipped < 35 KB total). Serve
behind any static host. The dashboard reads the API base URL from
`localStorage.promptsheon.settings.v1.apiBase`. If the field is empty, requests
hit the **same origin** (`/api/v1/...`). Cross-origin deployments must configure
CORS on the daemon via `PROMPTSHEON_CORS_ORIGINS`.

## Route map

| Hash route | Reads | Writes |
|---|---|---|
| `#/` | workspaces / projects / capabilities / releases / metrics / providers / alerts / audit | refresh, new capability |
| `#/capabilities/{id}` | capability / contract / versions / reputation | contract, self-evolve, edit, delete, new version, diff |
| `#/releases` | capability-scoped releases | vote, rollback |
| `#/audit` | audit log + verify chain | filter (action/resource), export CSV |
| `#/observability` | metrics summary + top capabilities | none |
| `#/guardrails` | active alerts | resolve |
| `#/evaluations` | version executions | run new execution |
| `#/operations/alerts` | alert rules + notification groups | CRUD rules |
| `#/operations/webhooks` | webhooks | CRUD |
| `#/operations/vault` | vault keys | save / delete |
| `#/operations/providers` | provider registry | test connection |
| `#/operations/users` | users | CRUD (admin only) |
| `#/operations/reasoning` | none | compile intent → CapabilityPlan |
| `#/logs` | SSE `/api/v1/logs/stream` | filter level |

Top nav (sidebar) routes are `<a href="#/…">`. The back button works.

## Smoke test

```bash
node scripts/smoke.mjs   # boots daemon, seeds, walks every route, asserts
```

The smoke assumes `puppeteer-core` is reachable and uses the bundled Chromium at
`~/Library/Caches/ms-playwright/chromium_headless_shell-*/chrome-headless-shell-mac-arm64/chrome-headless-shell`.
