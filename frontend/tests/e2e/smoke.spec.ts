import { test, expect } from '@playwright/test';
import { walkOnboarding } from './helpers/walk-onboarding';

test.describe('smoke: top-level entry point', () => {
  test('walk real onboarding → land on /app', async ({ page }) => {
    test.setTimeout(60_000);
    await walkOnboarding(page);
    await expect(page).toHaveURL(/\/app/);
  });
});
