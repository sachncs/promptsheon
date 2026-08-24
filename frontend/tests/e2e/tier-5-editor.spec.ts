import { test, expect } from '@playwright/test';
import { walkOnboarding } from './helpers/walk-onboarding';

const LLM_KEY = process.env['E2E_LLM_KEY'] ?? process.env['MINIMAX_API_KEY'] ?? '';
const LLM_BASE = process.env['E2E_LLM_BASE_URL'] ?? 'https://api.minimax.io/anthropic';
const LLM_MODEL = process.env['E2E_LLM_MODEL'] ?? 'MiniMax-M3';

test.describe('tier 5: editor', () => {
  test.beforeAll(() => {
    if (!LLM_KEY) throw new Error('E2E_LLM_KEY (or MINIMAX_API_KEY) must be set');
  });

  test('pick "Customer support triage" template seeds 4 nodes in the canvas', async ({ page, baseURL }) => {
    test.setTimeout(60_000);
    await walkOnboarding(page, { baseUrl: baseURL, llmApiKey: LLM_KEY, llmBaseUrl: LLM_BASE, llmModel: LLM_MODEL });

    await page.goto('/app/editor');
    // Apply the customer-support-triage template.
    await page.getByRole('button', { name: /customer support triage/i }).click();
    // The React Flow viewport renders nodes; the brand template
    // creates 4: classify / retrieve / decide / respond.
    await expect(page.locator('.react-flow__node')).toHaveCount(4, { timeout: 5_000 });
  });
});
