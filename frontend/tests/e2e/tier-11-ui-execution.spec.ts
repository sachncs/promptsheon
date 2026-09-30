import { test, expect, request } from '@playwright/test';
import { bootstrapAdminViaApi, clearClientState, seedSession, type SessionInfo } from './helpers/seed-session';

test.describe('tier 11: browser execution journey', () => {
  test('saves a capability, runs a simulator preview, and shows its trace', async ({ page, baseURL }) => {
    if (!baseURL) throw new Error('baseURL not provided');

    const admin: SessionInfo = await bootstrapAdminViaApi(baseURL, {
      baseUrl: baseURL,
      orgName: `Tier 11 Org ${Date.now()}`,
      adminName: 'Tier 11 Admin',
      adminEmail: `tier-11-${Date.now()}@promptsheon.test`,
    });
    const api = await request.newContext({ baseURL });
    const headers = { Authorization: `Bearer ${admin.apiKey}` };

    const llmResponse = await api.post('/api/bootstrap/llm', {
      headers,
      data: { provider: 'simulated', model: 'promptsheon-e2e-simulator' },
    });
    expect(llmResponse.ok(), await llmResponse.text()).toBeTruthy();

    const workspaceResponse = await api.post('/api/workspaces', {
      headers,
      data: { name: `Tier 11 workspace ${Date.now()}`, organization: admin.orgName },
    });
    expect(workspaceResponse.ok(), await workspaceResponse.text()).toBeTruthy();
    const workspace = (await workspaceResponse.json()) as { id: string };

    const projectResponse = await api.post('/api/projects', {
      headers,
      data: { workspaceId: workspace.id, name: `Tier 11 project ${Date.now()}`, description: '' },
    });
    expect(projectResponse.ok(), await projectResponse.text()).toBeTruthy();
    await api.dispose();

    await clearClientState(page);
    await seedSession(page, admin);

    await page.goto('/app/editor');
    await page.getByRole('button', { name: /customer support triage/i }).click();
    await page.getByLabel('Or create capability').fill('Tier 11 support triage');
    await page.getByRole('button', { name: /^save/i }).click();
    await page.waitForURL(/\/app\/editor\/[0-9a-f]{64}$/, { timeout: 15_000 });

    await page.getByRole('button', { name: /run preview/i }).click();
    await page.waitForURL(/\/app\/traces$/, { timeout: 15_000 });
    await expect(page.getByRole('heading', { name: 'Traces' })).toBeVisible();
    await expect(page.getByText('Recent runs')).toBeVisible();
    await expect(page.getByText('promptsheon-e2e-simulator', { exact: true })).toBeVisible({ timeout: 15_000 });
    await expect(page.getByText('success', { exact: true }).first()).toBeVisible();
  });
});
