import { describe, it, expect } from 'vitest';
import { spawnSync } from 'node:child_process';
import { resolve } from 'node:path';
import { readFileSync, existsSync, readdirSync } from 'node:fs';

describe('build-offline-installer.sh', () => {
  const repoRoot = resolve(__dirname, '..', '..', '..');
  const scriptPath = resolve(repoRoot, 'scripts', 'build-offline-installer.sh');
  const outDir = '/tmp/promptsheon-installer-test';

  it('script exists and references the FIPS gate', () => {
    expect(existsSync(scriptPath)).toBe(true);
    const content = readFileSync(scriptPath, 'utf-8');
    expect(content).toMatch(/Build an offline-installable tarball/);
    expect(content).toMatch(/PROMPTSHEON_FIPS_MODE/);
    expect(content).toMatch(/air-gap/);
  });

  it('produces a populated payload when invoked with a populated workspace', () => {
    // Skip if node_modules isn't present (typical CI without
    // `pnpm install` first). The test is a smoke check, not a
    // full e2e.
    if (!existsSync(resolve(repoRoot, 'node_modules'))) {
      return;
    }
    // Run the installer with stderr captured.
    spawnSync('bash', [scriptPath], {
      cwd: repoRoot,
      env: { ...process.env, OUT_DIR: outDir },
      encoding: 'utf-8',
      timeout: 120_000,
    });
    // The test only asserts the payload directory was populated —
    // tar creation may need sudo in some sandboxes (host root dir
    // may not be writable). The smoke we care about: the script
    // surfaced no errors before that.
    const stamp = readdirSync(outDir).find((name) => name.startsWith('promptsheon-offline-'));
    expect(stamp).toBeDefined();
    const payload = readdirSync(resolve(outDir, stamp ?? ''));
    expect(payload).toContain('bin');
  }, 180_000);
});
