import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtemp, rm, readFile, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { randomUUID } from 'node:crypto';
import { CasStore } from '@promptsheon/shared';

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
    const hash = await store.writeObject({ type: 'blob', data: Buffer.from('hello world') });

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

  it('writes a tree referencing a blob', async () => {
    const blobHash = await store.writeObject({ type: 'blob', data: Buffer.from('file contents') });
    const treeHash = await store.writeObject({
      type: 'tree',
      entries: [{ name: 'README.md', hash: blobHash, type: 'blob' }],
    });

    expect(treeHash).toMatch(/^[0-9a-f]{64}$/);

    const tree = await store.readObject(treeHash);
    expect(tree.type).toBe('tree');
  });

  it('writes a commit referencing a tree', async () => {
    const blobHash = await store.writeObject({ type: 'blob', data: Buffer.from('payload') });
    const treeHash = await store.writeObject({
      type: 'tree',
      entries: [{ name: 'doc.txt', hash: blobHash, type: 'blob' }],
    });
    const commitHash = await store.writeObject({
      type: 'commit',
      treeHash,
      parents: [],
      author: 'tester',
      message: 'initial commit',
    });

    expect(commitHash).toMatch(/^[0-9a-f]{64}$/);

    const commit = await store.readObject(commitHash);
    expect(commit.type).toBe('commit');
    if (commit.type === 'commit') {
      expect(commit.treeHash).toBe(treeHash);
      expect(commit.message).toBe('initial commit');
    }
  });

  it('detects corrupted objects on read', async () => {
    const hash = await store.writeObject({ type: 'blob', data: Buffer.from('two') });

    const objDir = join(store.objectsDir, hash.slice(0, 2));
    const objFile = join(objDir, hash.slice(2));
    const original = await readFile(objFile);
    await writeFile(objFile, Buffer.concat([original, Buffer.from('garbage')]));

    await expect(store.readObject(hash)).rejects.toThrow();
  });
});