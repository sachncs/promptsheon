import { test, expect } from '@playwright/test';

test.describe('tier 2: marketing surface', () => {
  test('landing page renders the hero and CTAs', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
    await expect(page.getByRole('link', { name: /start building/i })).toBeVisible();
    await expect(page.getByRole('link', { name: /read the docs/i })).toHaveAttribute('href', '/docs');
    await expect(page.getByText(/adaptive agent engineering platform/i).first()).toBeVisible();
  });

  test('visitor navigation opens the developer docs route', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByRole('link', { name: 'Docs', exact: true }).first()).toHaveAttribute('href', '/docs');
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

  test('onboarding entry renders for a fresh or configured installation', async ({ page }) => {
    await page.goto('/onboarding');
    await expect(page).toHaveURL(/\/(onboarding|app)(\/|$)/);

    // A configured installation redirects onboarding to the control plane.
    // Assert the settled destination before inspecting onboarding-only UI so
    // the test does not race the client-side bootstrap redirect.
    if (new URL(page.url()).pathname.startsWith('/app')) {
      await expect(page.getByRole('heading', { name: /capability health/i })).toBeVisible();
      return;
    }

    await expect(page.getByText(/set up promptsheon/i)).toBeVisible();
    const beginSetup = page.getByRole('button', { name: /begin setup/i });
    const providerStep = page.getByRole('heading', { name: /choose a model provider|workspace ready/i });
    await expect(beginSetup.or(providerStep).first()).toBeVisible();
  });
});
