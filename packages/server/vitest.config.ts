import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'node',
    globals: false,
    include: ['test/**/*.test.ts'],
    // Keep concurrent SSR transforms bounded on small CI runners.
    maxWorkers: 2,
  },
});
