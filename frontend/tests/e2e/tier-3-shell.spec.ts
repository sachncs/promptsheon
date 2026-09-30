import { test, expect } from '@playwright/test';
import { walkOnboarding } from './helpers/walk-onboarding';
import { bootstrapAdminViaApi, seedSession } from './helpers/seed-session';

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
      '/app/traces',
      '/app/agents',
      '/app/admin/budgets',
      '/app/admin/quotas',
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
    await page.goto('/app/admin/budgets');
    await expect(page.getByRole('heading', { name: 'Spend budgets', exact: true })).toBeVisible();
    await page.goto('/app/admin/quotas');
    await expect(page.getByRole('heading', { name: 'User quotas', exact: true })).toBeVisible();
    await page.goto('/app/traces');
    await expect(page.getByText('Prompt risk signals', { exact: true })).toBeVisible();
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
      const headers = upstream.headers();
      delete headers['set-cookie'];
      await page.context().clearCookies();
      await route.fulfill({ response: upstream, headers, json: body });
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
      const headers = upstream.headers();
      delete headers['set-cookie'];
      await page.context().clearCookies();
      await route.fulfill({ response: upstream, headers, json: body });
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
    await seedSession(page, session);

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

  test('creates and disables a user quota from the admin UI', async ({ page, baseURL }) => {
    if (!baseURL) throw new Error('E2E base URL is required');
    const session = await bootstrapAdminViaApi(baseURL, {
      orgName: `Quota UI Org ${Date.now()}`,
      adminEmail: `quota-ui-${Date.now()}@promptsheon.test`,
    });
    await seedSession(page, session);
    await page.goto('/app/admin/quotas');
    await page.getByLabel('User ID').fill('quota-user');
    await page.getByLabel('Label').fill('Daily simulator limit');
    await page.getByLabel('Daily runs').fill('2');
    await page.getByRole('button', { name: 'Add quota' }).click();
    await expect(page.getByText('Daily simulator limit', { exact: true })).toBeVisible();
    await page.getByRole('button', { name: 'Disable' }).click();
    await expect(page.getByText('disabled', { exact: true })).toBeVisible();
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
    await expect(page).toHaveURL(/\/app\/agents\/[0-9a-f]{64}\?workspace=[0-9a-f-]{36}$/);
    await expect(page.getByRole('heading', { name: 'Research assistant' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Run this revision' })).toBeVisible();
    await page.getByRole('link', { name: 'Create child revision' }).click();
    await expect(page).toHaveURL(/\/app\/agents\/new\?parent=[0-9a-f]{64}&workspace=[0-9a-f-]{36}$/);
    await expect(page.getByText(/Creating a child revision from/)).toBeVisible();
    const validationRequest = page.waitForRequest((request) => request.method() === 'POST' && request.url().includes('/agent-specifications/validate'));
    await page.getByRole('button', { name: 'Validate specification' }).click();
    const validationBody = JSON.parse((await validationRequest).postData() ?? '{}') as { specification?: Record<string, unknown> };
    expect(validationBody.specification).toEqual(expect.objectContaining({
      evaluationPolicy: expect.any(Object),
      resourceBudget: expect.any(Object),
      permissions: expect.any(Object),
    }));
    await expect(page.getByText('Specification is valid and ready to create.')).toBeVisible();

    const workspaceResponse = await request.get(`${baseURL}/api/workspaces`, { headers: { Authorization: `Bearer ${session.apiKey}` } });
    expect(workspaceResponse.ok()).toBeTruthy();
  });

  test('requires workspace context when more than one workspace exists', async ({ page, request, baseURL }) => {
    if (!baseURL) throw new Error('E2E base URL is required');
    const session = await bootstrapAdminViaApi(baseURL, {
      orgName: `Multi Workspace Org ${Date.now()}`,
      adminEmail: `multi-workspace-${Date.now()}@promptsheon.test`,
    });
    const headers = { Authorization: `Bearer ${session.apiKey}` };
    const first = await request.post(`${baseURL}/api/workspaces`, { headers, data: { name: `Research ${Date.now()}`, organization: session.orgName } });
    const secondName = `Production ${Date.now()}`;
    const second = await request.post(`${baseURL}/api/workspaces`, { headers, data: { name: secondName, organization: session.orgName } });
    expect(first.ok(), await first.text()).toBeTruthy();
    expect(second.ok(), await second.text()).toBeTruthy();

    await seedSession(page, session);
    await page.goto('/app/agents');
    const workspaceSelect = page.getByRole('combobox', { name: 'Select workspace for agent specifications' });
    await expect(workspaceSelect).toBeVisible();
    await workspaceSelect.click();
    await page.getByRole('option', { name: secondName }).click();
    await expect(workspaceSelect).toContainText(secondName);
  });

  test('runs an agent specification through the simulator and renders evidence', async ({ page, request, baseURL }) => {
    if (!baseURL) throw new Error('E2E base URL is required');
    const session = await bootstrapAdminViaApi(baseURL, {
      orgName: `Agent Execution Org ${Date.now()}`,
      adminEmail: `agent-execution-${Date.now()}@promptsheon.test`,
    });
    const headers = { Authorization: `Bearer ${session.apiKey}` };
    const workspace = await request.post(`${baseURL}/api/workspaces`, {
      headers,
      data: { name: `Execution workspace ${Date.now()}`, organization: session.orgName },
    });
    expect(workspace.ok(), await workspace.text()).toBeTruthy();

    await seedSession(page, session);
    await page.goto('/app/agents/new');
    await page.getByRole('button', { name: 'Validate specification' }).click();
    await expect(page.getByText('Specification is valid and ready to create.')).toBeVisible();
    await page.getByRole('button', { name: 'Create draft' }).click();
    await page.getByRole('link', { name: 'Open revision' }).click();
    await page.getByRole('button', { name: 'Run revision' }).click();
    await expect(page.getByText('completed', { exact: true }).last()).toBeVisible({ timeout: 20_000 });
    await expect(page.getByText('Execution evidence')).toBeVisible();
    await expect(page.getByText(/execution\.(started|completed)/).first()).toBeVisible({ timeout: 10_000 });
  });

  test('plans an editable DAG from an idea without a provider key', async ({ page }) => {
    await walkOnboarding(page);
    await page.goto('/app/editor');
    await page.getByLabel('Plan from an idea').fill('Triage support requests and draft safe replies');
    await page.getByRole('button', { name: 'Plan DAG' }).click();
    await expect(page.getByText('DAG planned', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('Understand', { exact: true })).toBeVisible();
    await expect(page.getByText('Execute', { exact: true })).toBeVisible();
    await expect(page.getByText('Review', { exact: true })).toBeVisible();
  });
});
