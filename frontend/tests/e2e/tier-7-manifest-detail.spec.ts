import { test, expect } from '@playwright/test';
import { bootstrapAdminViaApi, seedSession, clearClientState, type SessionInfo } from './helpers/seed-session';

/**
 * Manifest-detail drilldown: navigate to a real manifest saved
 * by the DAG editor and verify the Overview / Source tab renders
 * the hash + manifest JSON.
 */

let admin: SessionInfo | null = null;

test.describe('tier 7: manifest detail (real page)', () => {
  test.beforeAll(async ({ baseURL, request }) => {
    if (!baseURL) throw new Error('baseURL not provided');
    if (!admin) {
      admin = await bootstrapAdminViaApi(baseURL, {
        baseUrl: baseURL,
        orgName: 'Tier7 Org',
        adminName: 'Tier7 Admin',
        adminEmail: 'tier7@promptsheon.test',
      });
    }
  });

  test('saving a manifest via the editor shows it on /app/manifests/[hash]', async ({
    page,
    baseURL,
  }) => {
    if (!admin || !baseURL) throw new Error('admin not bootstrapped');

    await clearClientState(page);
    await seedSession(page, admin);

    // 1. Open the editor and apply the triage template
    await page.goto('/app/editor');
    await page.getByRole('button', { name: /customer support triage/i }).click();

    // 2. Save
    await page.getByRole('button', { name: /save/i }).click();

    // 3. After save, URL navigates to /app/editor/<hash>
    await page.waitForURL(/\/app\/editor\//, { timeout: 10_000 });
    const url = new URL(page.url());
    const hash = url.pathname.replace('/app/editor/', '');
    expect(hash).toMatch(/^[0-9a-f]{64}$/);

    // 4. Navigate to the manifest detail page; Overview + Source tab should render
    await page.goto(`/app/manifests/${hash}`);
    await expect(page.getByText(/metadata/i)).toBeVisible();
    await expect(page.getByText(/source/i)).toBeVisible();
    await expect(page.getByText(/hash/i).first()).toBeVisible();
  });
});
