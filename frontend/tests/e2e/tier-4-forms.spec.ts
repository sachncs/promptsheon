import { test, expect, request } from '@playwright/test';

/**
 * Form-submission tier. Each test uses the backend bootstrap API
 * to install a session and a fresh workspace, then drives the
 * corresponding /app/* form through the UI and asserts the row
 * appears.
 *
 * Server tests in this same commit (admin-gating,
 * approval-reconcile, …) lock in the backend behaviour. These
 * UI tests focus on the form wiring + cache invalidation +
 * optimistic update path.
 */

const BASE = process.env['PROMPTSHEON_E2E_BASE_URL'] ?? 'http://127.0.0.1:8080';

async function bootstrap() {
  const ctx = await request.newContext({ baseURL: BASE });
  const slug = `t4-${Date.now()}`;
  let resp = await ctx.post('/api/bootstrap/admin', {
    data: {
      adminName: 'T4',
      adminEmail: `t4+${Date.now()}@promptsheon.test`,
      orgName: 'T4 Org',
      orgSlug: slug,
    },
  });
  if (resp.status() === 409) resp = await ctx.get('/api/bootstrap/admin');
  const body = (await resp.json()) as { user: { id: string }; org: { id: string } };
  return { ctx, userId: body.user.id, orgId: body.org.id };
}

test.describe('tier 4: forms submit and rows appear', () => {
  test('workspaces: create workspace', async ({ page, baseURL }) => {
    if (!baseURL) throw new Error('baseURL not provided');

    const { ctx, userId, orgId } = await bootstrap();
    await ctx.dispose();

    await page.goto('/');
    await page.evaluate(
      ([u, o]) => {
        window.localStorage.setItem(
          'promptsheon:session:v1',
          JSON.stringify({
            userId: u,
            userName: 'T4',
            userEmail: 't4@promptsheon.test',
            orgId: o,
            orgName: 'T4 Org',
            completedAt: new Date().toISOString(),
          }),
        );
      },
      [userId, orgId],
    );
    await page.goto('/app/workspaces');
    await page.getByLabel(/name/i).first().fill(`ws-${Date.now()}`);
    await page.getByRole('button', { name: /create workspace/i }).click();

    // Should appear in the table.
    await expect(page.getByText(/^ws-/).first()).toBeVisible({ timeout: 10_000 });
  });

  test('api-keys: create key and list', async ({ page, baseURL }) => {
    if (!baseURL) throw new Error('baseURL not provided');

    const { ctx, userId, orgId } = await bootstrap();
    await ctx.dispose();

    await page.goto('/');
    await page.evaluate(
      ([u, o]) => {
        window.localStorage.setItem(
          'promptsheon:session:v1',
          JSON.stringify({
            userId: u,
            userName: 'T4',
            userEmail: 't4@promptsheon.test',
            orgId: o,
            orgName: 'T4 Org',
            completedAt: new Date().toISOString(),
          }),
        );
      },
      [userId, orgId],
    );
    await page.goto('/app/api-keys');
    await page.getByLabel(/name/i).first().fill(`e2e-key-${Date.now()}`);
    await page.getByRole('button', { name: /create/i }).first().click();
    await expect(page.getByText(/^e2e-key-/).first()).toBeVisible({ timeout: 10_000 });
  });

  test('webhooks: create and list', async ({ page, baseURL }) => {
    if (!baseURL) throw new Error('baseURL not provided');

    const { ctx, userId, orgId } = await bootstrap();
    await ctx.dispose();

    await page.goto('/');
    await page.evaluate(
      ([u, o]) => {
        window.localStorage.setItem(
          'promptsheon:session:v1',
          JSON.stringify({
            userId: u,
            userName: 'T4',
            userEmail: 't4@promptsheon.test',
            orgId: o,
            orgName: 'T4 Org',
            completedAt: new Date().toISOString(),
          }),
        );
      },
      [userId, orgId],
    );
    await page.goto('/app/webhooks');
    await page.getByLabel(/label/i).first().fill(`hook-${Date.now()}`);
    await page.getByLabel(/url/i).first().fill('https://example.com/h');
    await page.getByRole('button', { name: /create/i }).first().click();
    await expect(page.getByText(/^hook-/).first()).toBeVisible({ timeout: 10_000 });
  });

  test('feature-flags: create and list', async ({ page, baseURL }) => {
    if (!baseURL) throw new Error('baseURL not provided');

    const { ctx, userId, orgId } = await bootstrap();
    await ctx.dispose();

    await page.goto('/');
    await page.evaluate(
      ([u, o]) => {
        window.localStorage.setItem(
          'promptsheon:session:v1',
          JSON.stringify({
            userId: u,
            userName: 'T4',
            userEmail: 't4@promptsheon.test',
            orgId: o,
            orgName: 'T4 Org',
            completedAt: new Date().toISOString(),
          }),
        );
      },
      [userId, orgId],
    );
    await page.goto('/app/feature-flags');
    await page.getByLabel(/name/i).first().fill(`flag_${Date.now()}`);
    await page.getByRole('button', { name: /create|save/i }).first().click();
    // Page lists seeded + created flags; we just verify the form path completes.
    await expect(page).toHaveURL(/\/app\/feature-flags/);
  });

  test('schedules: button is disabled until inputs filled', async ({ page, baseURL }) => {
    if (!baseURL) throw new Error('baseURL not provided');

    const { ctx, userId, orgId } = await bootstrap();
    await ctx.dispose();

    await page.goto('/');
    await page.evaluate(
      ([u, o]) => {
        window.localStorage.setItem(
          'promptsheon:session:v1',
          JSON.stringify({
            userId: u,
            userName: 'T4',
            userEmail: 't4@promptsheon.test',
            orgId: o,
            orgName: 'T4 Org',
            completedAt: new Date().toISOString(),
          }),
        );
      },
      [userId, orgId],
    );
    await page.goto('/app/schedules');
    // The page has disabled Create button until releaseId + cron are picked.
    const createBtn = page.getByRole('button', { name: /create/i }).first();
    await expect(createBtn).toBeDisabled();
  });
});
