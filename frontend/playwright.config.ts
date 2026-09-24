import { defineConfig, devices } from '@playwright/test';

const PORT = process.env['PROMPTSHEON_E2E_PORT'] ?? '3000';
const BACKEND_PORT = process.env['PROMPTSHEON_E2E_BACKEND_PORT'] ?? '8081';
const DATABASE_PATH = process.env['PROMPTSHEON_E2E_DB_PATH'] ?? 'promptsheon-test.db';
const BASE_URL = `http://127.0.0.1:${PORT}`;

export default defineConfig({
  testDir: './tests/e2e',
  timeout: 30_000,
  expect: { timeout: 5_000 },
  fullyParallel: false,
  workers: 1,
  reporter: [['list'], ['html', { open: 'never', outputFolder: 'playwright-report' }]],
  use: {
    baseURL: BASE_URL,
    headless: true,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],
  webServer: {
    command: `cd .. && PROMPTSHEON_PORT=${BACKEND_PORT} PROMPTSHEON_FRONTEND_PORT=${PORT} PROMPTSHEON_DB_PATH=${DATABASE_PATH} PROMPTSHEON_AUTH=true PROMPTSHEON_JWT_SECRET=e2e-only-secret-with-at-least-32-characters PROMPTSHEON_E2E=true PROMPTSHEON_RATE_LIMIT_MAX=10000 pnpm --dir packages dev`,
    url: BASE_URL,
    // Never attach to an unrelated process that happens to own the port.
    // The dev command starts both the frontend and the API; reusing only the
    // frontend leaves the API unavailable and can produce misleading failures.
    reuseExistingServer: false,
    timeout: 120_000,
    stdout: 'pipe',
    stderr: 'pipe',
  },
});
