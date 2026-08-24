import { test, expect, request } from '@playwright/test';

/**
 * Tier 8 — approvals flow: walk bootstrap → workspace → project →
 * capability → version → release → cast vote via the
 * reconciled POST /api/releases/:id/approvals. Use the raw API
 * (no UI clicks) so the test is fast and deterministic; the UI
 * version lives in tier-7-manifest-detail (Save) and tier-4
 * (forms).
 *
 * The end-to-end correctness of the maker-checker gate is
 * covered server-side in test/approval-reconcile.test.ts and
 * test/release-approval-gate.test.ts. This tier only verifies
 * that the route the frontend calls really exists and the
 * response shape matches what the page renders.
 */

const BASE = process.env['PROMPTSHEON_E2E_BASE_URL'] ?? 'http://127.0.0.1:8080';

test.describe('tier 8: approvals flow', () => {
  test('POST /api/releases/:id/approvals accepts a vote', async () => {
    const ctx = await request.newContext({ baseURL: BASE });

    // Bootstrap admin + workspace + project + capability + version + release.
    const ts = Date.now();
    let r = await ctx.post('/api/bootstrap/admin', {
      data: {
        adminName: 'T8',
        adminEmail: `t8+${ts}@promptsheon.test`,
        orgName: 'T8 Org',
        orgSlug: `t8-${ts}`,
      },
    });
    if (r.status() === 409) r = await ctx.get('/api/bootstrap/admin');
    const admin = (await r.json()) as { user: { id: string }; org: { id: string } };
    const H = { 'X-User-Id': admin.user.id, 'X-Org-Id': admin.org.id };

    const ws = await ctx.post('/api/workspaces', {
      headers: H,
      data: { name: `t8-ws-${ts}`, organization: 'T8' },
    });
    const wsBody = (await ws.json()) as { id: string };
    const proj = await ctx.post('/api/projects', {
      headers: H,
      data: { workspaceId: wsBody.id, name: 'P', description: '' },
    });
    const projBody = (await proj.json()) as { id: string };
    const cap = await ctx.post('/api/capabilities', {
      headers: H,
      data: { projectId: projBody.id, name: 'C', description: '' },
    });
    const capBody = (await cap.json()) as { id: string };
    const version = await ctx.post('/api/capability-versions', {
      headers: H,
      data: {
        capabilityId: capBody.id,
        version: 1,
        manifest: '{"nodes":[],"edges":[]}',
        manifestHash: 'x',
        goal: 'g',
      },
    });
    const vBody = (await version.json()) as { id: string };
    const release = await ctx.post('/api/releases', {
      headers: H,
      data: {
        capabilityId: capBody.id,
        capabilityVersion: 1,
        capabilityVersionId: vBody.id,
        environment: 'dev',
        manifest: '{"nodes":[],"edges":[]}',
        canaryPercent: 0,
      },
    });
    const rBody = (await release.json()) as { id: string };

    // GET approvals — empty initially
    let r2 = await ctx.get(`/api/approvals?releaseId=${rBody.id}`, { headers: H });
    expect(r2.ok(), `GET approvals ok: ${await r2.text()}`).toBeTruthy();

    // POST vote (admin self-vote is the maker-checker violation case — for
    // this tier we just exercise the route, not the policy).
    r2 = await ctx.post(`/api/releases/${rBody.id}/approvals`, {
      headers: H,
      data: { decision: 'approve', comment: 'e2e' },
    });
    expect(r2.status(), `POST approval ok: ${await r2.text()}`).toBe(201);
    const vote = (await r2.json()) as { decision: string; distinctApprovers: number };
    expect(vote.decision).toBe('approve');
    expect(vote.distinctApprovers).toBe(1);

    await ctx.dispose();
  });
});
