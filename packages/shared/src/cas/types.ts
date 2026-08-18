export type ObjectType = 'blob' | 'tree' | 'commit';

export interface BlobObject {
  type: 'blob';
  data: Buffer;
}

export interface TreeEntry {
  name: string;
  hash: string;
  type: ObjectType;
}

export interface TreeObject {
  type: 'tree';
  entries: TreeEntry[];
}

export interface CommitObject {
  type: 'commit';
  treeHash: string;
  parents: string[];
  author: string;
  message: string;
  telemetry: Record<string, unknown>;
}

export type CasObject = BlobObject | TreeObject | CommitObject;

export interface DiffEntry {
  name: string;
  status: 'added' | 'removed' | 'modified';
  oldHash?: string;
  newHash?: string;
}

export interface DiffResult {
  added: DiffEntry[];
  removed: DiffEntry[];
  modified: DiffEntry[];
}
