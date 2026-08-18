import type { CasStore } from './store.js';
import type { CommitObject, TreeObject, DiffResult, TreeEntry } from './types.js';

export async function diffIntelligence(
  store: CasStore,
  oldHash: string,
  newHash: string,
): Promise<DiffResult> {
  const oldCommit = (await store.readObject(oldHash)) as CommitObject;
  const newCommit = (await store.readObject(newHash)) as CommitObject;

  const oldTree = (await store.readObject(oldCommit.treeHash)) as TreeObject;
  const newTree = (await store.readObject(newCommit.treeHash)) as TreeObject;

  const oldMap = new Map(oldTree.entries.map((e) => [e.name, e]));
  const newMap = new Map(newTree.entries.map((e) => [e.name, e]));

  const added: DiffResult['added'] = [];
  const removed: DiffResult['removed'] = [];
  const modified: DiffResult['modified'] = [];

  for (const [name, entry] of newMap) {
    const old = oldMap.get(name);
    if (!old) {
      added.push({ name, status: 'added', newHash: entry.hash });
    } else if (old.hash !== entry.hash) {
      modified.push({ name, status: 'modified', oldHash: old.hash, newHash: entry.hash });
    }
  }

  for (const [name, entry] of oldMap) {
    if (!newMap.has(name)) {
      removed.push({ name, status: 'removed', oldHash: entry.hash });
    }
  }

  return { added, removed, modified };
}
