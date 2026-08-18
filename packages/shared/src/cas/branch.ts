import type { CasStore } from './store.js';
import { join } from 'node:path';
import { readdir, readFile, writeFile, unlink } from 'node:fs/promises';

export async function createBranch(store: CasStore, name: string, fromHash: string): Promise<void> {
  if (!(await store.exists(fromHash))) {
    throw new Error(`commit ${fromHash} not found`);
  }
  const refPath = join(store.refsDir, 'heads', name);
  await writeFile(refPath, fromHash);
}

export async function deleteBranch(store: CasStore, name: string): Promise<void> {
  const refPath = join(store.refsDir, 'heads', name);
  await unlink(refPath);
}

export async function checkout(store: CasStore, name: string): Promise<string> {
  const refPath = join(store.refsDir, 'heads', name);
  return readFile(refPath, 'utf-8');
}

export async function listBranches(store: CasStore): Promise<Array<{ name: string; hash: string }>> {
  const headsDir = join(store.refsDir, 'heads');
  const files = await readdir(headsDir);
  return Promise.all(
    files.map(async (name) => ({
      name,
      hash: (await readFile(join(headsDir, name), 'utf-8')).trim(),
    }))
  );
}
