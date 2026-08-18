import type { CasStore } from './store.js';
import type { CommitObject, TreeObject } from './types.js';
import { join } from 'node:path';
import { readFile, writeFile, mkdir, rename } from 'node:fs/promises';

export async function commit(
  store: CasStore,
  commitObj: CommitObject,
  refName: string,
): Promise<string> {
  const tree = await store.readObject(commitObj.treeHash);
  if (tree.type !== 'tree') throw new Error('treeHash does not point to a tree');

  for (const parent of commitObj.parents) {
    if (!(await store.exists(parent))) {
      throw new Error(`parent commit ${parent} not found`);
    }
  }

  const hash = await store.writeObject(commitObj);

  const refPath = join(store.refsDir, 'heads', refName);
  await mkdir(join(store.refsDir, 'heads'), { recursive: true });
  const tmpPath = refPath + '.tmp';
  await writeFile(tmpPath, hash);
  await rename(tmpPath, refPath);

  return hash;
}

export async function currentCommitHash(store: CasStore, refName: string): Promise<string | null> {
  try {
    const refPath = join(store.refsDir, 'heads', refName);
    const hash = await readFile(refPath, 'utf-8');
    return hash.trim();
  } catch {
    return null;
  }
}
