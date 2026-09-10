# @promptsheon/shared

Domain types, validation schemas, and shared utilities for the Promptsheon platform.

## Contents

- `src/types/` — 32 domain type files (Workspace, Project, Capability, Manifest, Release, etc.)
- `src/cas/` — Content-Addressable Store: SHA-256, gzip, sharded layout, blobs/trees/commits, branches, diffs, integrity verification
- `src/errors/` — `ErrorCode` enum, `AppError`, error→HTTP response helpers
- `src/sse.ts` — SSE event types (`log`, `progress`, `status`, `error`, `complete`, `heartbeat`, `alert`)
- `src/validation.ts` — Zod schemas for all API inputs (`CreateWorkspaceSchema`, `CreateProjectSchema`, ...)
- `src/constants.ts` — Domain constants
- `src/config.ts` — `AppConfig` interface
- `db/migrations/` — 51 SQLite migration files (the schema evolved
  through v0.1.0 → v0.4.2; migration 051 added the identity
  tables for SVID + apikey auth)

## Usage

```typescript
import { CasStore, CreateWorkspaceSchema, type Workspace } from '@promptsheon/shared';

const cas = new CasStore('.data/cas');
await cas.init();
const hash = await cas.writeObject({ type: 'blob', data: Buffer.from('hello') });
```

## Notes

- Migrations are forward-only and append new columns / tables; the
  earlier `organisation_id` column in `teams` was added by
  migration 047 and is now `NOT NULL`.
- `Buffer` type requires `@types/node`
