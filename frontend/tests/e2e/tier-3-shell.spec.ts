import { test, expect } from '@playwright/test';
import { walkOnboarding } from './helpers/walk-onboarding';
import { bootstrapAdminViaApi } from './helpers/seed-session';

const BACKEND_PORT = process.env['PROMPTSHEON_E2E_BACKEND_PORT'] ?? '8081';
const BACKEND_URL = `http://127.0.0.1:${BACKEND_PORT}`;

test.describe('tier 3: app shell after onboarding', () => {
  test('walks real onboarding and lands on /app with all sub-routes reachable', async ({ page }) => {
    test.setTimeout(60_000);

    await walkOnboarding(page);

    // AppShell sidebar should be visible.
    await expect(page.locator('aside').first()).toBeVisible();
    await expect(page.getByRole('link', { name: 'Control plane' })).toBeVisible();

    // Visit each sub-route through the session.
    const subRoutes = [
      '/app/workspaces',
      '/app/audit',
      '/app/goals',
      '/app/eval',
      '/app/vault',
      '/app/operations',
      '/app/evidence',
      '/app/agents',
    ];
    for (const path of subRoutes) {
      await page.goto(path);
      await expect(page).toHaveURL(new RegExp(`${path}$`));
      // Should NOT redirect to /onboarding (would mean session is broken).
      expect(page.url(), `${path} should not redirect to /onboarding`).not.toContain('/onboarding');
    }
    await page.goto('/app/operations');
    await expect(page.getByText('EXECUTION QUEUE')).toBeVisible();
    await expect(page.getByText(/running/i).last()).toBeVisible();
    await expect(page.getByText('PLATFORM HEALTH')).toBeVisible();
    await expect(page.getByText('Healthy')).toBeVisible();
    await page.goto('/app/evidence');
    await expect(page.getByRole('heading', { name: 'Evidence', exact: true })).toBeVisible();
    await page.goto('/app/agents');
    await expect(page.getByRole('heading', { name: 'Agent specifications', exact: true })).toBeVisible();
  });

  test('restores a cleared browser session with an existing API key', async ({ page, request }) => {
    const session = await bootstrapAdminViaApi(BACKEND_URL, {
      orgName: `Recovery Org ${Date.now()}`,
      adminEmail: `recovery-${Date.now()}@promptsheon.test`,
    });
    const response = await request.post(`${BACKEND_URL}/api/bootstrap/llm`, {
      headers: { Authorization: `Bearer ${session.apiKey}` },
      data: { provider: 'simulated', model: 'promptsheon-recovery-simulator' },
    });
    expect(response.ok(), `simulator setup failed: ${await response.text()}`).toBeTruthy();

    await page.route('**/api/bootstrap/admin', async (route) => {
      const upstream = await route.fetch();
      const body = (await upstream.json()) as { apiKey?: string };
      delete body.apiKey;
      await route.fulfill({ response: upstream, json: body });
    });
    await page.goto('/');
    await page.evaluate(() => window.localStorage.clear());
    await page.goto('/onboarding');
    await expect(page.getByText('Restore your browser session')).toBeVisible();
    await page.getByLabel('Promptsheon API key').fill(session.apiKey);
    await page.getByRole('button', { name: 'Verify key' }).click();
    await page.waitForURL(/\/app(\/|$)/);
  });

  test('stops on the recovery screen when automatic session recovery is rejected', async ({ page, request }) => {
    const session = await bootstrapAdminViaApi(BACKEND_URL, {
      orgName: `Rejected Recovery Org ${Date.now()}`,
      adminEmail: `rejected-recovery-${Date.now()}@promptsheon.test`,
    });
    const response = await request.post(`${BACKEND_URL}/api/bootstrap/llm`, {
      headers: { Authorization: `Bearer ${session.apiKey}` },
      data: { provider: 'simulated', model: 'promptsheon-recovery-simulator' },
    });
    expect(response.ok(), `simulator setup failed: ${await response.text()}`).toBeTruthy();

    await page.route('**/api/bootstrap/admin', async (route) => {
      const upstream = await route.fetch();
      const body = (await upstream.json()) as { apiKey?: string };
      body.apiKey = 'pk_rejected_by_backend';
      await route.fulfill({ response: upstream, json: body });
    });
    await page.goto('/');
    await page.evaluate(() => window.localStorage.clear());
    await page.goto('/onboarding');

    await expect(page).toHaveURL(/\/onboarding$/);
    await expect(page.getByText('Restore your browser session')).toBeVisible();
    await expect(page.getByText(/requires an API key/i).first()).toBeVisible();
  });

  test('validates an existing session before redirecting from onboarding', async ({ page, request }) => {
    const session = await bootstrapAdminViaApi(BACKEND_URL, {
      orgName: `Stale Session Org ${Date.now()}`,
      adminEmail: `stale-session-${Date.now()}@promptsheon.test`,
    });
    const response = await request.post(`${BACKEND_URL}/api/bootstrap/llm`, {
      headers: { Authorization: `Bearer ${session.apiKey}` },
      data: { provider: 'simulated', model: 'promptsheon-stale-session-simulator' },
    });
    expect(response.ok(), `simulator setup failed: ${await response.text()}`).toBeTruthy();

    await page.goto('/');
    await page.evaluate((input) => {
      window.localStorage.setItem('promptsheon:session:v1', JSON.stringify({
        userId: input.userId,
        userName: input.userName,
        userEmail: input.userEmail,
        orgId: input.orgId,
        orgName: input.orgName,
        apiKey: 'pk_stale_session',
        completedAt: new Date().toISOString(),
      }));
    }, session);

    await page.goto('/onboarding');
    await expect(page).toHaveURL(/\/onboarding$/);
    await expect(page.getByText('Restore your browser session')).toBeVisible();
    await expect(page.getByText(/could not be verified|invalid|unauthorized/i).first()).toBeVisible();
  });

  test('resumes at provider setup when an authenticated admin has no provider', async ({ page }) => {
    const session = await bootstrapAdminViaApi(BACKEND_URL, {
      orgName: `Provider Resume Org ${Date.now()}`,
      adminEmail: `provider-resume-${Date.now()}@promptsheon.test`,
    });

    await page.route('**/api/bootstrap/status', async (route) => {
      const upstream = await route.fetch();
      const body = (await upstream.json()) as Record<string, unknown>;
      await route.fulfill({
        response: upstream,
        json: { ...body, needsAdmin: false, needsLlm: true, provider: null, model: null },
      });
    });
    await page.goto('/');
    await page.evaluate((input) => {
      window.localStorage.setItem('promptsheon:session:v1', JSON.stringify({
        userId: input.userId,
        userName: input.userName,
        userEmail: input.userEmail,
        orgId: input.orgId,
        orgName: input.orgName,
        apiKey: input.apiKey,
        completedAt: new Date().toISOString(),
      }));
    }, session);

    await page.goto('/onboarding');
    await expect(page.getByRole('heading', { name: 'Choose a model provider' })).toBeVisible();
    await expect(page.getByRole('button', { name: /local simulator/i })).toBeVisible();
  });

  test('rejects malformed local sessions without redirect churn', async ({ page }) => {
    await page.goto('/');
    await page.evaluate(() => {
      window.localStorage.setItem('promptsheon:session:v1', JSON.stringify({ userId: 'incomplete' }));
    });

    await page.goto('/app');
    await expect(page).toHaveURL(/\/onboarding$/);
    await page.waitForTimeout(500);
    await expect(page).toHaveURL(/\/onboarding$/);
  });

  test('creates and validates an agent specification without a provider key', async ({ page, request, baseURL }) => {
    if (!baseURL) throw new Error('E2E base URL is required');
    const session = await bootstrapAdminViaApi(baseURL, {
      orgName: `Agent Builder Org ${Date.now()}`,
      adminEmail: `agent-builder-${Date.now()}@promptsheon.test`,
    });
    await seedSession(page, session);
    await page.goto('/app/workspaces');
    await page.getByLabel('Name').fill(`agent-workspace-${Date.now()}`);
    await page.getByRole('button', { name: 'Create workspace' }).click();
    await expect(page).toHaveURL(/\/app\/workspaces\/[^/]+\/projects$/);

    await page.goto('/app/agents/new');
    await expect(page.getByRole('heading', { name: 'New agent specification' })).toBeVisible();
    await page.getByRole('button', { name: 'Validate specification' }).click();
    await expect(page.getByText('Specification is valid and ready to create.')).toBeVisible();
    await page.getByRole('button', { name: 'Create draft' }).click();
    await expect(page.getByText('Revision created')).toBeVisible();
    await page.getByRole('link', { name: 'Open revision' }).click();
    await expect(page).toHaveURL(/\/app\/agents\/[0-9a-f]{64}$/);
    await expect(page.getByRole('heading', { name: 'Research assistant' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Run this revision' })).toBeVisible();

    const workspaceResponse = await request.get(`${baseURL}/api/workspaces`, { headers: { Authorization: `Bearer ${session.apiKey}` } });
    expect(workspaceResponse.ok()).toBeTruthy();
  });
});
