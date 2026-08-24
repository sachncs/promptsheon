import { test, expect } from '@playwright/test';

const ROUTES = [
  '/',
  '/onboarding',
  '/docs',
  '/docs/quickstart',
  '/app',
  '/app/compiler',
  '/app/capabilities',
  '/app/editor',
  '/app/executions/test-id',
  '/app/manifests/test-id',
  '/app/operations',
  '/app/schedules',
  '/app/feature-flags',
  '/app/users',
  '/app/webhooks',
  '/app/settings',
  '/app/approvals',
  '/app/approvals/test-id',
  '/app/alerts/active',
  '/app/alerts/rules',
  '/app/projects',
  '/app/audit',
  '/app/capabilities/test-id/self-evolve',
  '/app/capabilities/test-id/datasets',
  '/app/capabilities/test-id/preconditions',
  '/app/repos',
  '/app/merge-requests',
  '/app/search',
  '/app/workspaces',
  '/app/goals',
  '/app/eval',
  '/app/eval/suites',
  '/app/diff',
  '/app/admin/cost',
  '/app/vault',
  '/app/api-keys',
  '/app/releases',
  '/app/releases/test-id',
  '/app/projects/test-id/capabilities',
];

test.describe('tier 1: route smoke', () => {
  for (const path of ROUTES) {
    test(`renders ${path} without a console error`, async ({ page }) => {
      const consoleErrors: string[] = [];
      page.on('pageerror', (err) => consoleErrors.push(String(err)));
      page.on('console', (msg) => {
        if (msg.type() === 'error') consoleErrors.push(msg.text());
      });

      const response = await page.goto(path, { waitUntil: 'domcontentloaded' });
      expect(response, `expected 2xx for ${path}`).not.toBeNull();
      expect(response!.status(), `${path} status`).toBeLessThan(400);

      // Filter out:
      //  - React hydration warnings (dev server quirk)
      //  - 401/403/404 from API calls: expected when no session is
      //    present; tier 1 deliberately runs unauthenticated to
      //    verify the page shell mounts. Auth-gated testing lives in
      //    tier 3+.
      const real = consoleErrors.filter(
        (m) => !/hydrat|did not match|Warning:|401|403|404|Failed to load resource/i.test(m),
      );
      expect(real, `console errors on ${path}: ${real.join('\n')}`).toHaveLength(0);
    });
  }
});
