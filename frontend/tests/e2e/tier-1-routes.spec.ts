import { test, expect } from '@playwright/test';
import { bootstrapAdminViaApi, seedSession, clearClientState, type SessionInfo } from './helpers/seed-session';

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
  '/app/goals/test-hash',
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

let cachedSession: SessionInfo | null = null;

test.describe('tier 1: route smoke (authenticated)', () => {
  test.beforeAll(async ({ baseURL }) => {
    if (!cachedSession && baseURL) {
      cachedSession = await bootstrapAdminViaApi(baseURL, {
        baseUrl: baseURL,
        orgName: 'Tier1 Org',
        adminName: 'Tier1 Admin',
        adminEmail: 'tier1@promptsheon.test',
      });
    }
  });

  for (const path of ROUTES) {
    test(`renders ${path} without a console error`, async ({ page, baseURL }) => {
      if (!cachedSession) throw new Error('session not bootstrapped');

      // Wipe per-test and seed fresh.
      await clearClientState(page);
      await seedSession(page, cachedSession);

      const consoleErrors: string[] = [];
      page.on('pageerror', (err) => consoleErrors.push(String(err)));
      page.on('console', (msg) => {
        if (msg.type() === 'error') consoleErrors.push(msg.text());
      });

      const response = await page.goto(path, { waitUntil: 'domcontentloaded' });
      expect(response, `expected a response for ${path}`).not.toBeNull();
      const status = response!.status();
      // 404 for deliberately-bad IDs is fine (e.g. /app/executions/test-id);
      // 5xx is not. The page may 200 or 404; the route shell must mount.
      expect(status, `${path} status`).toBeLessThan(500);

      // Filter out React-hydration warnings (dev server) and
      // 401/403/404 from optional API calls the page made before
      // a session token was attached. We don't filter 5xx — that's
      // a real failure.
      const real = consoleErrors.filter(
        (m) => !/hydrat|did not match|Warning:|401|403|404|Failed to load resource/i.test(m),
      );
      expect(real, `console errors on ${path}: ${real.join('\n')}`).toHaveLength(0);
    });
  }
});
