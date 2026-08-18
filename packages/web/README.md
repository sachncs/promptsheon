# @promptsheon/web

React 19 frontend with shadcn/ui, TanStack Query, and React Router v7.

## Setup

```bash
cd packages
pnpm install
```

## Run

```bash
# Dev server (proxies /api to localhost:8080)
npm run dev

# Production build
npm run build
npm run preview
```

## Stack

- React 19 + TypeScript
- Vite 6 with HMR
- Tailwind CSS v4
- shadcn/ui components (Radix UI primitives)
- TanStack Query v5
- React Router v7 (HashRouter for backwards compatibility)

## Structure

- `src/App.tsx` — HashRouter with 25 routes
- `src/components/Layout.tsx` — Sidebar with 5 collapsible navigation groups
- `src/components/ui/` — 15 shadcn/ui components
- `src/components/modals/` — 11 modal dialogs
- `src/views/` — 25 view components
- `src/lib/api.ts` — 22 API client modules
- `src/lib/utils.ts` — `cn()` Tailwind helper

## Routes

| Path | View |
|------|------|
| `/` | Dashboard |
| `/workspaces` | Workspace list |
| `/workspaces/:id/projects` | Project list |
| `/projects/:id/capabilities` | Capability list |
| `/capabilities/:id` | Capability detail |
| `/capabilities/:id/{releases,executions,datasets,eval,self-evolve,preconditions}` | Capability sub-views |
| `/operations` | Operations Hub (9 tabs) |
| `/settings` | Settings |
| `/alerts/{rules,active}` | Alert management |
| `/schedules` | Schedules |
| `/webhooks` | Webhooks |
| `/users` | Users |
| `/api-keys` | API keys |
| `/feature-flags` | Feature flags |
| `/audit` | Audit log |
| `/compiler` | Reasoning compiler |
