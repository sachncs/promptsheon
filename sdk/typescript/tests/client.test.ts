/**
 * SDK-TS-1: the generated OpenAPI bindings typecheck. This file
 * exists to ensure the codegen output compiles against the
 * project's strict tsconfig — it is the type-level smoke test
 * for the typescript SDK.
 */
import type { paths } from "../src/openapi";

// Force the `paths` map to be referenced by the type checker.
type _P = paths;
const _paths: _P | undefined = undefined;
void _paths;

describe("openapi-generated client", () => {
  it("typechecks", () => {
    // paths is type-only; we just confirm the import resolved
    // and the test file compiled by reaching this line.
    expect(true).toBe(true);
  });
});
