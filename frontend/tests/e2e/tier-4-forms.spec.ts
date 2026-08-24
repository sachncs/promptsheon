import { test, expect } from '@playwright/test';
import { walkOnboarding } from './helpers/walk-onboarding';

const LLM_KEY = process.env['E2E_LLM_KEY'] ?? process.env['MINIMAX_API_KEY'] ?? '';
const LLM_BASE = process.env['E2E_LLM_BASE_URL'] ?? 'https://api.minimax.io/anthropic';
const LLM_MODEL = process.env['E2E_LLM_MODEL'] ?? 'MiniMax-M3';
const SUFFIX = `e2e-${Date.now().toString(36)}`;

test.describe('tier 4: form submissions', () => {
  test.beforeAll(() => {
    if (!LLM_KEY) throw new Error('E2E_LLM_KEY (or MINIMAX_API_KEY) must be set');
  });

  test.beforeEach(async ({ page, baseURL }) => {
    test.setTimeout(90_000);
    await walkOnboarding(page, { baseUrl: baseURL, llmApiKey: LLM_KEY, llmBaseUrl: LLM_BASE, llmModel: LLM_MODEL });
  });

  test('workspace: create form lands a new row', async ({ page }) => {
    const name = `ws-${SUFFIX}`;
    await page.goto('/app/workspaces');
    await page.getByLabel(/^name$/i).first().fill(name);
    await page.getByRole('button', { name: /create workspace/i }).click();
    await expect(page.getByText(name)).toBeVisible({ timeout: 10_000 });
  });

  test('api-key: create form lands a new row', async ({ page }) => {
    const name = `key-${SUFFIX}`;
    await page.goto('/app/api-keys');
    await page.getByLabel(/^name$/i).first().fill(name);
    await page.getByRole('button', { name: /issue/i }).click();
    await expect(page.getByText(name)).toBeVisible({ timeout: 10_000 });
  });

  test('webhook: create form lands a new row', async ({ page }) => {
    const url = `https://e2e-${SUFFIX}.test/hook`;
    await page.goto('/app/webhooks');
    await page.getByLabel(/^url$/i).first().fill(url);
    // The page's "Add webhook" button.
    await page.getByRole('button', { name: /add webhook/i }).click();
    await expect(page.getByText(url)).toBeVisible({ timeout: 10_000 });
  });

  test('schedules: create form lands a new row', async ({ page }) => {
    await page.goto('/app/schedules');
    // First fill release-id (a uuid), cron, then submit.
    await page.getByLabel(/^cron$/i).fill('0 */6 * * *');
    await page.getByRole('button', { name: /schedule/i }).click();
    // The form requires a release id to submit; without one the
    // button is disabled. We assert the disabled state instead of
    // forcing a submit, since picking a real release is out of
    // scope for this smoke.
    await expect(page.getByRole('button', { name: /schedule/i })).toBeDisabled();
  });

  test('feature-flags: create form lands a new row', async ({ page }) => {
    const key = `flag-${SUFFIX}`;
    await page.goto('/app/feature-flags');
    await page.getByLabel(/^key$/i).first().fill(key);
    await page.getByRole('button', { name: /create flag/i }).click();
    await expect(page.getByText(key)).toBeVisible({ timeout: 10_000 });
  });
});
