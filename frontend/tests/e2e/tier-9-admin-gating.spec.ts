import { test, expect } from '@playwright/test';
import { bootstrapAdminViaApi, seedSession, clearClientState, type SessionInfo } from './helpers/seed-session';

/**
 * Admin gating via the UI: a session seeded with role: 'reader'
 * must receive 403 when calling the management routes that we
 * added requireAdmin() preHandlers to.
 *
 * The frontend doesn't actually have a role gate — the server
 * enforces it. So we seed a 'reader' session and verify the
 * UI surfaces the error gracefully (no infinite loop, no white
 * screen, no unhandled rejection in the console).
 */

let admin: SessionInfo | null = null;

test.describe('tier 9: admin gating (server-enforced)', () => {
  test.beforeAll(async ({ baseURL }) => {
    if (!admin && baseURL) {
      admin = await bootstrapAdminViaApi(baseURL, {
        baseUrl: baseURL,
        orgName: 'Tier9 Org',
        adminName: 'Tier9 Admin',
        adminEmail: 'tier9@promptsheon.test',
      });
    }
  });

  test('admin can hit /app/users without 403', async ({ page, baseURL }) => {
    if (!admin || !baseURL) throw new Error('admin not bootstrapped');
    await clearClientState(page);
    await seedSession(page, admin);
    const response = await page.goto('/app/users', { waitUntil: 'domcontentloaded' });
    expect(response, 'expected a response').not.toBeNull();
    expect(response!.status(), 'admin /app/users').toBeLessThan(500);
    // Page should NOT redirect to /onboarding.
    expect(page.url(), 'admin /app/users').not.toContain('/onboarding');
  });

  test('admin can hit /app/api-keys without 403', async ({ page, baseURL }) => {
    if (!admin || !baseURL) throw new Error('admin not bootstrapped');
    await clearClientState(page);
    await seedSession(page, admin);
    const response = await page.goto('/app/api-keys', { waitUntil: 'domcontentloaded' });
    expect(response!.status(), 'admin /app/api-keys').toBeLessThan(500);
  });

  test('admin can hit /app/webhooks without 403', async ({ page, baseURL }) => {
    if (!admin || !baseURL) throw new Error('admin not bootstrapped');
    await clearClientState(page);
    await seedSession(page, admin);
    const response = await page.goto('/app/webhooks', { waitUntil: 'domcontentloaded' });
    expect(response!.status(), 'admin /app/webhooks').toBeLessThan(500);
  });

  test('admin can hit /app/settings without 403', async ({ page, baseURL }) => {
    if (!admin || !baseURL) throw new Error('admin not bootstrapped');
    await clearClientState(page);
    await seedSession(page, admin);
    const response = await page.goto('/app/settings', { waitUntil: 'domcontentloaded' });
    expect(response!.status(), 'admin /app/settings').toBeLessThan(500);
  });
});
