import type { CasStore } from './store.js';
import { readdir } from 'node:fs/promises';
import { join } from 'node:path';

export interface VerifyResult {
  valid: boolean;
  errors: string[];
  orphans: string[];
}

export async function verify(store: CasStore): Promise<VerifyResult> {
  const errors: string[] = [];
  const orphans: string[] = [];

  const dirs = await readdir(store.objectsDir);
  for (const dir of dirs) {
    const files = await readdir(join(store.objectsDir, dir));
    for (const file of files) {
      const hash = dir + file;
      try {
        await store.readObject(hash);
      } catch {
        errors.push(`corrupt object: ${hash}`);
      }
    }
  }

  return { valid: errors.length === 0, errors, orphans };
}
