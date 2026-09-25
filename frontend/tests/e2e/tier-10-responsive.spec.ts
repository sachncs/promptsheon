import { test, expect } from '@playwright/test';
import { bootstrapAdminViaApi, seedSession, type SessionInfo } from './helpers/seed-session';

test.use({
  viewport: { width: 390, height: 844 },
  isMobile: true,
});

let session: SessionInfo | null = null;

test.describe('tier 10: responsive visitor and app surfaces', () => {
  test.beforeAll(async ({ baseURL }) => {
    if (baseURL) {
      session = await bootstrapAdminViaApi(baseURL, {
        baseUrl: baseURL,
        orgName: 'Responsive Org',
        adminName: 'Responsive Admin',
        adminEmail: 'responsive@promptsheon.test',
      });
    }
  });

  test('landing page keeps its layout within the viewport', async ({ page }) => {
    await page.goto('/');

    const dimensions = await page.evaluate(() => ({
      viewport: window.innerWidth,
      document: document.documentElement.scrollWidth,
    }));
    expect(dimensions.document).toBeLessThanOrEqual(dimensions.viewport);
  });

  test('mobile navigation contains the full visitor path', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: 'Menu' }).click();

    const navigation = page.locator('#mobile-navigation');
    await expect(navigation).toBeVisible();
    await expect(navigation.getByRole('link', { name: 'Product' })).toBeVisible();
    await expect(navigation.getByRole('link', { name: 'Docs' })).toBeVisible();
    await expect(navigation.getByRole('link', { name: 'Sign in' })).toBeVisible();
    await expect(navigation.getByRole('link', { name: 'Open dashboard' })).toBeVisible();
  });

  test('authenticated app header stays usable on a narrow screen', async ({ page }) => {
    if (!session) throw new Error('session not bootstrapped');

    await seedSession(page, session);
    await page.goto('/app');

    const dimensions = await page.evaluate(() => ({
      viewport: window.innerWidth,
      document: document.documentElement.scrollWidth,
    }));
    expect(dimensions.document).toBeLessThanOrEqual(dimensions.viewport);
    await expect(page.getByRole('button', { name: 'New capability' })).toBeVisible();
  });
});
