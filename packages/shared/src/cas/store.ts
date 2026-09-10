import { createHash } from 'node:crypto';
import { gzip, gunzip } from 'node:zlib';
import { promisify } from 'node:util';
import { join } from 'node:path';
import { mkdir, readFile, writeFile, stat, rename } from 'node:fs/promises';
import type { CasObject } from './types.js';

const gzipAsync = promisify(gzip);
const gunzipAsync = promisify(gunzip);

const MAX_OBJECT_ON_DISK_BYTES = 64 * 1024 * 1024;
const MAX_OBJECT_INFLATED_BYTES = 256 * 1024 * 1024;

export class CasStore {
  readonly objectsDir: string;
  readonly refsDir: string;

  constructor(private basePath: string) {
    this.objectsDir = join(basePath, 'objects');
    this.refsDir = join(basePath, 'refs');
  }

  async init(): Promise<void> {
    await mkdir(this.objectsDir, { recursive: true });
    await mkdir(join(this.refsDir, 'heads'), { recursive: true });
  }

  async writeObject(obj: CasObject): Promise<string> {
    const data = Buffer.from(JSON.stringify(obj));
    const hash = createHash('sha256').update(data).digest('hex');
    const dir = join(this.objectsDir, hash.slice(0, 2));
    const filePath = join(dir, hash.slice(2));

    try {
      await stat(filePath);
      return hash;
    } catch {
      // Object doesn't exist, proceed to write
    }

    await mkdir(dir, { recursive: true });
    const gzipped = await gzipAsync(data);
    const tmpPath = filePath + '.tmp';
    await writeFile(tmpPath, gzipped);
    await rename(tmpPath, filePath);
    return hash;
  }

  async readObject(hash: string): Promise<CasObject> {
    if (!/^[0-9a-f]{64}$/.test(hash)) {
      throw new Error(`invalid hash: ${hash}`);
    }
    const dir = join(this.objectsDir, hash.slice(0, 2));
    const filePath = join(dir, hash.slice(2));

    const gzipped = await readFile(filePath);
    if (gzipped.length > MAX_OBJECT_ON_DISK_BYTES) {
      throw new Error('object exceeds maximum on-disk size');
    }

    const data = await gunzipAsync(gzipped);
    if (data.length > MAX_OBJECT_INFLATED_BYTES) {
      throw new Error('object exceeds maximum inflated size');
    }

    const obj: CasObject = JSON.parse(data.toString());
    const computed = createHash('sha256').update(Buffer.from(JSON.stringify(obj))).digest('hex');
    if (computed !== hash) {
      throw new Error(`object corruption: expected ${hash}, got ${computed}`);
    }

    return obj;
  }

  async objectHash(obj: CasObject): Promise<string> {
    const data = Buffer.from(JSON.stringify(obj));
    return createHash('sha256').update(data).digest('hex');
  }

  async exists(hash: string): Promise<boolean> {
    try {
      const dir = join(this.objectsDir, hash.slice(0, 2));
      await stat(join(dir, hash.slice(2)));
      return true;
    } catch {
      return false;
    }
  }
}
