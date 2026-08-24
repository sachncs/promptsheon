import { test, expect } from '@playwright/test';
import { walkOnboarding } from './helpers/walk-onboarding';

const LLM_KEY = process.env['E2E_LLM_KEY'] ?? process.env['MINIMAX_API_KEY'] ?? '';
const LLM_BASE = process.env['E2E_LLM_BASE_URL'] ?? 'https://api.minimax.io/anthropic';
const LLM_MODEL = process.env['E2E_LLM_MODEL'] ?? 'MiniMax-M3';

test.describe('tier 6: audit + releases', () => {
  test.beforeAll(() => {
    if (!LLM_KEY) throw new Error('E2E_LLM_KEY (or MINIMAX_API_KEY) must be set');
  });

  test('audit: clicking a row opens the drawer with the hash chip', async ({ page, baseURL }) => {
    test.setTimeout(60_000);
    await walkOnboarding(page, { baseUrl: baseURL, llmApiKey: LLM_KEY, llmBaseUrl: LLM_BASE, llmModel: LLM_MODEL });

    await page.goto('/app/audit');
    // Open the first row (only one exists for a fresh DB: the
    // admin creation audit row).
    const firstRow = page.locator('tbody tr').first();
    await firstRow.click();
    // The Drawer should open with a Hash chip inside.
    await expect(page.locator('[role="dialog"]').filter({ hasText: /hash/i }).first()).toBeVisible({ timeout: 5_000 });
  });

  test('releases: list page renders without redirecting to /onboarding', async ({ page, baseURL }) => {
    test.setTimeout(60_000);
    await walkOnboarding(page, { baseUrl: baseURL, llmApiKey: LLM_KEY, llmBaseUrl: LLM_BASE, llmModel: LLM_MODEL });
    await page.goto('/app/releases');
    await expect(page).toHaveURL(/\/app\/releases$/);
    expect(page.url(), 'releases should not redirect to /onboarding').not.toContain('/onboarding');
  });
});
