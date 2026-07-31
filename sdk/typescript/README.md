# @promptsheon/typescript

Auto-generated TypeScript client for the [Promptsheon](https://github.com/sachncs/promptsheon)
v1 API.

The `src/openapi.ts` file is regenerated from
`backend/spec/spec.yaml` by `openapi-typescript`. The output is
types-only: a `paths` map keyed by route, with request/response
schemas per method. Consumers instantiate the generated HTTP
client of their choice (fetch, axios, …) and wrap those types.

## Usage

```ts
import type { paths } from "@promptsheon/typescript";

type ListWorkspaces = paths["/api/v1/workspaces"]["get"]["responses"]["200"]["content"]["application/json"];
```

## Development

Regenerate the types from the production OpenAPI spec, then build:

```sh
cd sdk/typescript
npm install
npm run codegen   # regenerates src/openapi.ts from ../../backend/spec/spec.yaml
npm test          # tsc --noEmit; verifies the package compiles
npm run build     # emit dist/
```

The codegen script uses `openapi-typescript` and requires Node.js
>= 18. If `npm run codegen` produces a diff the SDK types are
out of sync with the daemon's OpenAPI spec. CI fails the build.
