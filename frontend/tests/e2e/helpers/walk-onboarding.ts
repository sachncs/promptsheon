import type { Page, BrowserContext } from '@playwright/test';

const UNIQUE = `e2e-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

/**
 * Walks the full 4-step /onboarding flow:
 *   1. Welcome
 *   2. Admin + org
 *   3. LLM provider (credential-free local simulator)
 *   4. Finish → /app
 */
export async function walkOnboarding(page: Page): Promise<void> {
  await page.goto('/onboarding');

  // The E2E web server intentionally reuses one database for the suite.
  // Once the first test completes bootstrap, onboarding restores the admin
  // session and redirects directly to the control plane. Treat that as the
  // completed form of this journey instead of trying to repeat setup.
  await page.waitForLoadState('networkidle', { timeout: 10_000 }).catch(() => undefined);
  if (new URL(page.url()).pathname.startsWith('/app')) return;
  // Welcome
  await page.getByRole('button', { name: /begin setup/i }).click();

  // Admin + org
  await page.getByLabel(/admin name/i).fill(`E2E Admin ${UNIQUE}`);
  await page.getByLabel(/admin email/i).fill(`e2e-${UNIQUE}@promptsheon.test`);
  await page.getByLabel(/organisation name/i).fill(`E2E Org ${UNIQUE}`);
  await page.getByRole('button', { name: /continue/i }).click();

  // LLM step: use the built-in simulator so the browser suite is
  // deterministic and never needs a live provider credential.
  await page.getByRole('button', { name: /local simulator/i }).click();
  await page.getByLabel(/model name/i).fill('promptsheon-e2e-simulator');

  // Test connection against the local simulator.
  await page.getByRole('button', { name: /test connection/i }).click();

  // Wait for probe to succeed: "Connected · NNms · model" appears.
  await page.getByText(/connected/i).waitFor({ timeout: 15_000 });

  // Save → step 4
  await page.getByRole('button', { name: /continue/i }).click();

  // Finish → /app
  await page.getByRole('button', { name: /open the control plane|open dashboard/i }).click();
  await page.waitForURL(/\/app(\/|$)/, { timeout: 15_000 });
}

/**
 * Wipes the localStorage session on a fresh page so a test can
 * re-bootstrap from scratch. Idempotent.
 */
export async function clearClientState(page: Page): Promise<void> {
  await page.context().clearCookies();
  await page.goto('/');
  await page.evaluate(() => {
    try { window.localStorage.clear(); } catch { /* ignore */ }
  });
}

/**
 * Seed the same localStorage session shape that AdminStep.onSuccess
 * produces, so tests can skip the 4-step onboarding when they only
 * need authenticated reads.
 *
 * The session stores the organization metadata used by the UI. In a
 * protected environment, authenticated API calls also require the
 * bootstrap-issued Bearer API key.
 */
export async function seedSession(
  context: BrowserContext,
  baseUrl: string,
  orgId: string,
  userId: string,
  orgName: string,
  userName: string,
): Promise<void> {
  const page = await context.newPage();
  await page.goto(baseUrl);
  await page.evaluate(([id, oid, on, un]) => {
    window.localStorage.setItem('promptsheon:session:v1', JSON.stringify({
      userId: id, userName: un, userEmail: 'e2e@promptsheon.test',
      orgId: oid, orgName: on, completedAt: new Date().toISOString(),
    }));
  }, [userId, orgId, orgName, userName]);
  await page.close();
}
