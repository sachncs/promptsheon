import { test, expect, type Page, request } from '@playwright/test';

/**
 * Boot a fresh admin via the backend bootstrap API and seed the
 * browser's localStorage with the resulting session so the test
 * can hit authenticated pages without walking onboarding.
 *
 * This replaces the old walkOnboarding() flow that made a real
 * LLM probe call. The functional onboarding flow still has its
 * own tier in `smoke.spec.ts`; everything else uses this
 * bypass.
 */
export interface SessionInfo {
  userId: string;
  userName: string;
  userEmail: string;
  orgId: string;
  orgName: string;
  completedAt: string;
}

export interface SeedOptions {
  baseUrl: string;
  orgName?: string;
  adminName?: string;
  adminEmail?: string;
}

/**
 * Hit /api/bootstrap/admin via raw fetch (bypassing the Fastify
 * auth chain) to install the canonical admin + org. Returns the
 * session shape that the frontend's axios client expects.
 *
 * If an admin already exists (409), falls back to GET
 * /api/bootstrap/admin to recover the same shape.
 */
export async function bootstrapAdminViaApi(baseUrl: string, opts: SeedOptions): Promise<SessionInfo> {
  const apiContext = await request.newContext({ baseURL: baseUrl });
  const orgName = opts.orgName ?? `E2E Org ${Date.now()}`;
  const adminName = opts.adminName ?? 'E2E Admin';
  const adminEmail = opts.adminEmail ?? `e2e+${Date.now()}@promptsheon.test`;
  let resp = await apiContext.post('/api/bootstrap/admin', {
    data: { adminName, adminEmail, orgName, orgSlug: orgName.toLowerCase().replace(/\s+/g, '-') },
  });
  if (resp.status() === 409) {
    resp = await apiContext.get('/api/bootstrap/admin');
  }
  if (!resp.ok()) {
    throw new Error(`bootstrap admin failed: ${resp.status()} ${await resp.text()}`);
  }
  const body = (await resp.json()) as { user: { id: string; name: string; email: string }; org: { id: string; name: string } };
  return {
    userId: body.user.id,
    userName: body.user.name,
    userEmail: body.user.email,
    orgId: body.org.id,
    orgName: body.org.name,
    completedAt: new Date().toISOString(),
  };
}

export async function seedSession(page: Page, session: SessionInfo): Promise<void> {
  await page.goto('/');
  await page.evaluate((s) => {
    window.localStorage.setItem('promptsheon:session:v1', JSON.stringify(s));
  }, session);
}

export async function clearClientState(page: Page): Promise<void> {
  await page.context().clearCookies();
  await page.goto('/');
  await page.evaluate(() => {
    try {
      window.localStorage.clear();
    } catch {
      /* ignore */
    }
  });
}
