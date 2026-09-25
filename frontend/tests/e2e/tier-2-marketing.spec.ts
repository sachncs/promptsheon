import { test, expect } from '@playwright/test';

test.describe('tier 2: marketing surface', () => {
  test('landing page renders the hero and CTAs', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
    await expect(page.getByRole('link', { name: /start building/i })).toBeVisible();
    await expect(page.getByText(/adaptive agent engineering platform/i).first()).toBeVisible();
  });

  test('docs index renders the section tabs', async ({ page }) => {
    await page.goto('/docs');
    await expect(page.getByRole('link', { name: /quickstart/i }).first()).toBeVisible();
  });

  test('docs index renders the product model', async ({ page }) => {
    await page.goto('/docs');
    await expect(page.locator('main')).toBeVisible();
    await expect(page.getByText(/executable specification/i)).toBeVisible();
  });

  test('onboarding step 1 (welcome) renders', async ({ page }) => {
    await page.goto('/onboarding');
    await expect(page.getByText(/set up promptsheon/i)).toBeVisible();
    await expect(page.getByRole('button', { name: /begin setup/i })).toBeVisible();
  });
});
