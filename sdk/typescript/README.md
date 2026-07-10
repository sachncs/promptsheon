# @promptsheon/typescript

Auto-generated TypeScript client for the [Promptsheon](https://github.com/sachncs/promptsheon)
v1 API.

## Usage

```ts
import { PromptsheonClient } from "@promptsheon/typescript";

const client = new PromptsheonClient({
  baseUrl: "https://api.promptsheon.example.com",
  apiKey: process.env.PROMPTSHEON_API_KEY,
});

const capabilities = await client.listCapabilities("project-1");
```

## Development

Regenerate the types from the production OpenAPI spec, then build:

```sh
cd sdk/typescript
npm install
npm run codegen   # regenerates src/openapi.ts from ../../api/openapi.yaml
npm test          # tsc --noEmit; verifies the package compiles
npm run build     # emit dist/
```

The codegen script uses `openapi-typescript` and requires Node.js
>= 18. Today `src/openapi.ts` is a placeholder hand-written against
the public resource list in the architecture review; the M3
follow-on commit wires the codegen pipeline.
