import type { BlobObject, TreeObject, TreeEntry, CommitObject } from './types.js';

export function createBlob(data: Buffer): BlobObject {
  return { type: 'blob', data };
}

export function createTree(entries: TreeEntry[]): TreeObject {
  return { type: 'tree', entries };
}

export function createCommit(
  treeHash: string,
  parents: string[],
  author: string,
  message: string,
  telemetry: Record<string, unknown> = {},
): CommitObject {
  return { type: 'commit', treeHash, parents, author, message, telemetry };
}
