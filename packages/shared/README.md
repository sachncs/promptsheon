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
- `db/migrations/` — 21 SQLite migration files (verbatim from Go)

## Usage

```typescript
import { CasStore, CreateWorkspaceSchema, type Workspace } from '@promptsheon/shared';

const cas = new CasStore('.data/cas');
await cas.init();
const hash = await cas.writeObject({ type: 'blob', data: Buffer.from('hello') });
```

## Notes

- Same 21 SQLite migrations as the Go codebase — zero data migration needed
- `Buffer` type requires `@types/node`
