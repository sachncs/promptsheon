/**
 * SDK-TS-1: smoke test that the TypeScript client typechecks
 * and exposes every public method called by the SDK-PYTEST-1
 * mirror set in sdk/python/tests/test_client.py.
 */
import { PromptsheonClient, PromptsheonAPIError } from "../client";

describe("PromptsheonClient", () => {
  const baseConfig = { baseUrl: "https://api.example.com" };

  test("exposes baseUrl", () => {
    const c = new PromptsheonClient(baseConfig);
    expect(c.baseUrl()).toBe("https://api.example.com");
  });

  test("exposes every documented method", () => {
    const c = new PromptsheonClient(baseConfig);
    const methods = [
      "listCapabilities", "getCapability", "updateCapabilityContract",
      "getCapabilityContract", "getCapabilityReputation",
      "diffCapabilityVersions", "catalogSearch",
      "listReleases", "createRelease", "voteRelease",
      "activateRelease", "rollbackRelease", "invokeRelease",
      "runEval",
      "verifyAuditChain", "listSettings", "setSetting",
    ];
    for (const m of methods) {
      expect(typeof (c as any)[m]).toBe("function");
    }
  });

  test("PromptsheonAPIError carries status / method / path / body", () => {
    const err = new PromptsheonAPIError(404, "GET", "/api/v1/x", '{"error":"missing"}');
    expect(err.status).toBe(404);
    expect(err.method).toBe("GET");
    expect(err.path).toBe("/api/v1/x");
    expect(String(err)).toContain("GET /api/v1/x returned 404");
  });
});
