import { test, expect } from '@playwright/test';
import { walkOnboarding, clearClientState } from './helpers/walk-onboarding';

const LLM_KEY = process.env['E2E_LLM_KEY'] ?? process.env['MINIMAX_API_KEY'] ?? '';
const LLM_BASE = process.env['E2E_LLM_BASE_URL'] ?? 'https://api.minimax.io/anthropic';
const LLM_MODEL = process.env['E2E_LLM_MODEL'] ?? 'MiniMax-M3';

test.describe('tier 3: app shell after onboarding', () => {
  test.beforeAll(() => {
    if (!LLM_KEY) {
      throw new Error(
        'E2E_LLM_KEY (or MINIMAX_API_KEY) must be set; the smoke test makes a real LLM probe call during onboarding.',
      );
    }
  });

  test('walks real onboarding and lands on /app with all sub-routes reachable', async ({ page, baseURL }) => {
    test.setTimeout(60_000);

    await walkOnboarding(page, {
      baseUrl: baseURL,
      llmApiKey: LLM_KEY,
      llmBaseUrl: LLM_BASE,
      llmModel: LLM_MODEL,
    });

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
