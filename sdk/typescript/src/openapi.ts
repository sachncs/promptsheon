// Placeholder until scripts/codegen.sh regenerates from
// api/openapi.yaml via openapi-typescript. The codegen pipeline
// ships in M3 follow-on; the placeholder paths are committed so
// the SDK package compiles today and downstream consumers can
// adopt it before the codegen lands.
//
// Paths mirror api/openapi.yaml (v0.1.x): every API route is
// under /api/v1/. Update this file when the codegen lands; do not
// hand-edit the keys without also updating api/openapi.yaml.
export interface paths {
  "/api/v1/projects/{project_id}/capabilities": {
    get: { responses: { 200: { content: { "application/json": Array<unknown> } } } };
  };
  "/api/v1/releases/{release_id}/invoke": {
    post: { responses: { 201: { content: { "application/json": unknown } } } };
  };
}
