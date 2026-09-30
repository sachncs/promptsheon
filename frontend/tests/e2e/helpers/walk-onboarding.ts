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

  // CI may start with a provider configured through the environment. The
  // product must require an existing API key in that state; the E2E harness
  // can use its dedicated recovery endpoint to establish the test session
  // without introducing a real provider credential.
  const recoveryScreen = page.getByText('Restore your browser session');
  if (await recoveryScreen.isVisible({ timeout: 5_000 }).catch(() => false)) {
    const response = await page.request.get('/api/bootstrap/admin');
    const body = (await response.json()) as {
      apiKey?: string;
      user: { id: string; email: string; name: string };
      org: { id: string; name: string };
    };
    if (!response.ok() || !body.apiKey) {
      throw new Error('E2E recovery endpoint did not return a session key');
    }
    await page.evaluate((session) => {
      window.localStorage.setItem('promptsheon:session:v1', JSON.stringify(session));
    }, {
      userId: body.user.id,
      userName: body.user.name,
      userEmail: body.user.email,
      orgId: body.org.id,
      orgName: body.org.name,
      apiKey: body.apiKey,
      completedAt: new Date().toISOString(),
    });
    await page.goto('/app');
    await page.waitForURL(/\/app(\/|$)/, { timeout: 15_000 });
    return;
  }

  // Welcome
  await page.getByRole('button', { name: /begin setup/i }).click();

  // Admin + org
  // A shared E2E database may already have an admin from the route-smoke
  // tier. In that case onboarding resumes directly at provider setup.
  const adminName = page.getByLabel(/admin name/i);
  if (await adminName.isVisible({ timeout: 2_000 }).catch(() => false)) {
    await adminName.fill(`E2E Admin ${UNIQUE}`);
    await page.getByLabel(/admin email/i).fill(`e2e-${UNIQUE}@promptsheon.test`);
    await page.getByLabel(/organisation name/i).fill(`E2E Org ${UNIQUE}`);
    await page.getByRole('button', { name: /continue/i }).click();
  }

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
