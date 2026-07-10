// Placeholder until scripts/codegen.sh regenerates from
// api/openapi.yaml via openapi-typescript. The codegen pipeline
// ships in M3 follow-on; the placeholder path is committed so
// the SDK package compiles today and downstream consumers can
// adopt it before the codegen lands.
export interface paths {
  "/v1/projects/{project_id}/capabilities": {
    get: { responses: { 200: { content: { "application/json": Array<unknown> } } } };
  };
  "/v1/releases/{release_id}/invoke": {
    post: { responses: { 201: { content: { "application/json": unknown } } } };
  };
}
