import { test, expect } from '@playwright/test';
import { walkOnboarding } from './helpers/walk-onboarding';

const LLM_KEY = process.env['E2E_LLM_KEY'] ?? process.env['MINIMAX_API_KEY'] ?? '';
const LLM_BASE = process.env['E2E_LLM_BASE_URL'] ?? 'https://api.minimax.io/anthropic';
const LLM_MODEL = process.env['E2E_LLM_MODEL'] ?? 'MiniMax-M3';

test.describe('smoke: top-level entry point', () => {
  test('walk real onboarding → land on /app', async ({ page, baseURL }) => {
    test.setTimeout(60_000);
    if (!LLM_KEY) {
      throw new Error('Set E2E_LLM_KEY (or MINIMAX_API_KEY) before running smoke.spec.ts');
    }
    await walkOnboarding(page, { baseUrl: baseURL, llmApiKey: LLM_KEY, llmBaseUrl: LLM_BASE, llmModel: LLM_MODEL });
    await expect(page).toHaveURL(/\/app/);
  });
});
