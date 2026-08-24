import type { Page, BrowserContext } from '@playwright/test';

export interface OnboardingOptions {
  baseUrl: string;
  llmApiKey: string;
  llmBaseUrl: string;
  llmModel: string;
}

const UNIQUE = `e2e-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

/**
 * Walks the full 4-step /onboarding flow:
 *   1. Welcome
 *   2. Admin + org
 *   3. LLM provider (custom URL + key + model) — real probe call
 *   4. Finish → /app
 *
 * The probe is a live call to whatever URL the caller provided.
 * Caller is expected to pass a key that the URL accepts. The
 * MINIMAX_API_KEY env var is the production target; tests can
 * pass any working endpoint.
 */
export async function walkOnboarding(
  page: Page,
  options: OnboardingOptions,
): Promise<void> {
  await page.goto('/onboarding');

  // Welcome
  await page.getByRole('button', { name: /begin setup/i }).click();

  // Admin + org
  await page.getByLabel(/admin name/i).fill(`E2E Admin ${UNIQUE}`);
  await page.getByLabel(/admin email/i).fill(`e2e-${UNIQUE}@promptsheon.test`);
  await page.getByLabel(/organisation name/i).fill(`E2E Org ${UNIQUE}`);
  await page.getByRole('button', { name: /continue/i }).click();

  // LLM step
  // Pick the 'Custom endpoint' tile (the 4th option in the new
  // provider grid).
  await page.getByRole('button', { name: /custom endpoint/i }).click();

  await page.getByLabel(/base url/i).fill(options.llmBaseUrl);
  await page.getByLabel(/model name/i).fill(options.llmModel);
  await page.getByLabel(/^api key$/i).fill(options.llmApiKey);

  // Test connection (real HTTP to llmBaseUrl)
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
 * The session is what the API client's X-User-Id / X-Org-Id
 * request headers are derived from. Without seeding, every /app/*
 * call returns 401 and the page redirects to /onboarding.
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
