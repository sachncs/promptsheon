import { test, expect } from '@playwright/test';
import { walkOnboarding } from './helpers/walk-onboarding';

test.describe('tier 5: editor', () => {
  test('pick "Customer support triage" template seeds 4 nodes in the canvas', async ({ page }) => {
    test.setTimeout(60_000);
    await walkOnboarding(page);

    await page.goto('/app/editor');
    // Apply the customer-support-triage template.
    await page.getByRole('button', { name: /customer support triage/i }).click();
    // The React Flow viewport renders nodes; the brand template
    // creates 4: classify / retrieve / decide / respond.
    await expect(page.locator('.react-flow__node')).toHaveCount(4, { timeout: 5_000 });
  });

  test('selects a node and attaches a credential-free built-in tool', async ({ page }) => {
    test.setTimeout(60_000);
    await walkOnboarding(page);

    await page.goto('/app/editor');
    await page.getByRole('button', { name: /customer support triage/i }).click();
    await expect(page.locator('.react-flow__node')).toHaveCount(4, { timeout: 5_000 });
    await page.locator('.react-flow__node').first().click();
    await expect(page.getByText('Node: n1', { exact: false })).toBeVisible();

    await page.getByLabel('Tools', { exact: true }).waitFor();
    await page.getByLabel('Tools', { exact: true }).selectOption('json.parse');
    await page.getByRole('button', { name: 'Add', exact: true }).click();
    await expect(page.getByText('json.parse', { exact: true })).toBeVisible();
    await expect(page.getByText('Allowed: json.parse', { exact: true })).toBeVisible();
  });
});
