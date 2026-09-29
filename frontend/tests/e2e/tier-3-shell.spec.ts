import { test, expect } from '@playwright/test';
import { walkOnboarding } from './helpers/walk-onboarding';

test.describe('tier 3: app shell after onboarding', () => {
  test('walks real onboarding and lands on /app with all sub-routes reachable', async ({ page, baseURL }) => {
    test.setTimeout(60_000);

    await walkOnboarding(page);

    // AppShell sidebar should be visible.
    await expect(page.locator('aside').first()).toBeVisible();
    await expect(page.getByText(/control plane/i)).toBeVisible();

    // Visit each sub-route through the session.
    const subRoutes = [
      '/app/workspaces',
      '/app/audit',
      '/app/goals',
      '/app/eval',
      '/app/vault',
      '/app/operations',
    ];
    for (const path of subRoutes) {
      await page.goto(path);
      await expect(page).toHaveURL(new RegExp(`${path}$`));
      // Should NOT redirect to /onboarding (would mean session is broken).
      expect(page.url(), `${path} should not redirect to /onboarding`).not.toContain('/onboarding');
    }
  });
});
