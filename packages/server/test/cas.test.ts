import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtemp, rm, readFile, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { randomUUID } from 'node:crypto';
import {
  CasStore,
  createBlob,
  createTree,
  createCommit,
  commit,
  createBranch,
  listBranches,
  verify,
} from '@promptsheon/shared';

describe('CasStore', () => {
  let baseDir: string;
  let store: CasStore;

  beforeEach(async () => {
    baseDir = await mkdtemp(join(tmpdir(), `cas-test-${randomUUID()}-`));
    store = new CasStore(baseDir);
    await store.init();
  });

  afterEach(async () => {
    await rm(baseDir, { recursive: true, force: true });
  });

  it('writes a blob and reads it back', async () => {
    const blob = createBlob(Buffer.from('hello world'));
    const hash = await store.writeObject(blob);

    expect(hash).toMatch(/^[0-9a-f]{64}$/);

    const read = await store.readObject(hash);
    expect(read.type).toBe('blob');
    if (read.type === 'blob') {
      const data = Buffer.isBuffer(read.data)
        ? read.data
        : Buffer.from((read.data as { type: string; data: number[] }).data);
      expect(data.toString()).toBe('hello world');
    }
  });

  it('creates a tree, commit, and branch', async () => {
    const blob = createBlob(Buffer.from('file contents'));
    const blobHash = await store.writeObject(blob);

    const tree = createTree([{ name: 'README.md', hash: blobHash, type: 'blob' }]);
    const treeHash = await store.writeObject(tree);

    const commitObj = createCommit(treeHash, [], 'tester', 'initial commit');
    const commitHash = await commit(store, commitObj, 'main');

    expect(commitHash).toMatch(/^[0-9a-f]{64}$/);

    const branches = await listBranches(store);
    const names = branches.map((b) => b.name).sort();
    expect(names).toEqual(['main']);
    expect(branches[0].hash).toBe(commitHash);
  });

  it('creates a secondary branch from a commit', async () => {
    const blob = createBlob(Buffer.from('payload'));
    const blobHash = await store.writeObject(blob);
    const tree = createTree([{ name: 'doc.txt', hash: blobHash, type: 'blob' }]);
    const treeHash = await store.writeObject(tree);
    const commitHash = await commit(store, createCommit(treeHash, [], 'tester', 'root'), 'main');

    await createBranch(store, 'feature', commitHash);

    const branches = await listBranches(store);
    const byName = Object.fromEntries(branches.map((b) => [b.name, b.hash]));
    expect(byName.feature).toBe(commitHash);
    expect(byName.main).toBe(commitHash);
  });

  it('verifies chain integrity', async () => {
    const blob = createBlob(Buffer.from('one'));
    const blobHash = await store.writeObject(blob);
    const treeHash = await store.writeObject(createTree([{ name: 'a', hash: blobHash, type: 'blob' }]));
    await commit(store, createCommit(treeHash, [], 'tester', 'commit'), 'main');

    const result = await verify(store);
    expect(result.valid).toBe(true);
    expect(result.errors).toEqual([]);
  });

  it('detects corruption', async () => {
    const blob = createBlob(Buffer.from('two'));
    const blobHash = await store.writeObject(blob);
    const treeHash = await store.writeObject(createTree([{ name: 'a', hash: blobHash, type: 'blob' }]));
    await commit(store, createCommit(treeHash, [], 'tester', 'commit'), 'main');

    const objDir = join(store.objectsDir, blobHash.slice(0, 2));
    const objFile = join(objDir, blobHash.slice(2));
    const original = await readFile(objFile);
    await writeFile(objFile, Buffer.concat([original, Buffer.from('garbage')]));

    const result = await verify(store);
    expect(result.valid).toBe(false);
    expect(result.errors.length).toBeGreaterThan(0);
  });
});
