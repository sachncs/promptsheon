import { test, expect, type Page } from '@playwright/test';
import { bootstrapAdminViaApi, seedSession, type SessionInfo } from './helpers/seed-session';

const BACKEND_PORT = process.env['PROMPTSHEON_E2E_BACKEND_PORT'] ?? '8081';
const BACKEND_URL = `http://127.0.0.1:${BACKEND_PORT}`;

let session: SessionInfo | null = null;

async function assertAccessibleSurface(page: Page): Promise<void> {
  const violations = await page.evaluate(() => {
    const issues: string[] = [];
    const interactive = document.querySelectorAll<HTMLElement>(
      'a, button, input, select, textarea, [role="button"], [role="link"], [tabindex="0"]',
    );

    for (const element of interactive) {
      const hidden = element.getAttribute('aria-hidden') === 'true'
        || element.closest('[aria-hidden="true"]') !== null
        || (element instanceof HTMLButtonElement && element.disabled);
      if (hidden) continue;

      const hasName = Boolean(
        element.getAttribute('aria-label')?.trim()
        || element.getAttribute('aria-labelledby')?.trim()
        || element.textContent?.trim()
        || (element instanceof HTMLInputElement && element.labels?.length),
      );
      if (!hasName) {
        issues.push(`${element.tagName.toLowerCase()} has no accessible name`);
      }
    }

    for (const image of document.querySelectorAll<HTMLImageElement>('img')) {
      if (image.getAttribute('aria-hidden') === 'true') continue;
      if (!image.hasAttribute('alt')) issues.push('image is missing alt text');
    }

    if (!document.querySelector('main, [role="main"]')) issues.push('page has no main landmark');
    return issues;
  });

  expect(violations, `accessibility violations on ${page.url()}`).toEqual([]);
}

test.describe('tier 12: accessibility and keyboard foundations', () => {
  test.beforeAll(async () => {
    session = await bootstrapAdminViaApi(BACKEND_URL, {
      orgName: `Accessibility Org ${Date.now()}`,
      adminName: 'Accessibility Admin',
      adminEmail: `accessibility-${Date.now()}@promptsheon.test`,
    });
  });

  test('public landing page has named controls, images, and landmarks', async ({ page }) => {
    await page.goto('/');
    await assertAccessibleSurface(page);
    await expect(page.getByRole('main')).toBeVisible();
    const docsLink = page.getByRole('link', { name: 'Docs', exact: true });
    await docsLink.focus();
    await expect(docsLink).toBeFocused();
  });

  test('onboarding exposes a keyboard-reachable form surface', async ({ page }) => {
    await page.goto('/onboarding');
    await assertAccessibleSurface(page);
    await expect(page.getByRole('main')).toBeVisible();
    const firstInteractive = page.locator('main button:visible, main input:visible, main a:visible').first();
    await expect(firstInteractive).toBeVisible();
    await firstInteractive.focus();
    await expect(firstInteractive).toBeFocused();
  });

  test('authenticated shell has named controls and a main landmark', async ({ page }) => {
    if (!session) throw new Error('session was not bootstrapped');
    await seedSession(page, session);
    await page.goto('/app');
    await assertAccessibleSurface(page);
    await expect(page.getByRole('main')).toBeVisible();
    await page.getByRole('link', { name: 'Control plane' }).focus();
    await expect(page.getByRole('link', { name: 'Control plane' })).toBeFocused();
  });
});
