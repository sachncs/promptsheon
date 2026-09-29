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
});
