import { createHash, randomUUID } from 'node:crypto';
import { gzip, gunzip } from 'node:zlib';
import { promisify } from 'node:util';
import { join } from 'node:path';
import { mkdir, readFile, writeFile, stat, rename, unlink, open, readdir } from 'node:fs/promises';
import type { CasObject } from './types.js';

const gzipAsync = promisify(gzip);
const gunzipAsync = promisify(gunzip);

const MAX_OBJECT_ON_DISK_BYTES = 64 * 1024 * 1024;
const MAX_OBJECT_INFLATED_BYTES = 256 * 1024 * 1024;

export interface CasVerificationReport {
  valid: boolean;
  objectsChecked: number;
  corruptObjects: string[];
  unexpectedEntries: string[];
}

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
    return this.writeBlob(data);
  }

  /** Store immutable bytes under their SHA-256 content address. */
  async writeBlob(data: Buffer): Promise<string> {
    if (data.length > MAX_OBJECT_INFLATED_BYTES) {
      throw new Error('object exceeds maximum inflated size');
    }
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
    const tmpPath = `${filePath}.${process.pid}.${randomUUID()}.tmp`;
    const handle = await open(tmpPath, 'wx');
    try {
      await handle.writeFile(gzipped);
      await handle.sync();
    } finally {
      await handle.close();
    }
    try {
      await rename(tmpPath, filePath);
    } catch (error) {
      try {
        await stat(filePath);
        await unlink(tmpPath);
      } catch {
        throw error;
      }
    }
    return hash;
  }

  async readObject(hash: string): Promise<CasObject> {
    const data = await this.readBlob(hash);
    const obj: CasObject = JSON.parse(data.toString());
    return obj;
  }

  /** Read and integrity-check immutable bytes from the CAS. */
  async readBlob(hash: string): Promise<Buffer> {
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

    const computed = createHash('sha256').update(data).digest('hex');
    if (computed !== hash) {
      throw new Error(`object corruption: expected ${hash}, got ${computed}`);
    }

    return data;
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

  /** Scan every stored object and report corruption or unexpected files. */
  async verifyObjects(): Promise<CasVerificationReport> {
    const corruptObjects: string[] = [];
    const unexpectedEntries: string[] = [];
    let objectsChecked = 0;
    let prefixes: import('node:fs').Dirent[] = [];

    try {
      prefixes = await readdir(this.objectsDir, { withFileTypes: true });
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code === 'ENOENT') {
        return { valid: false, objectsChecked, corruptObjects, unexpectedEntries: [this.objectsDir] };
      }
      throw error;
    }

    for (const prefix of prefixes) {
      if (!prefix.isDirectory() || !/^[0-9a-f]{2}$/.test(prefix.name)) {
        unexpectedEntries.push(join(this.objectsDir, prefix.name));
        continue;
      }
      const entries = await readdir(join(this.objectsDir, prefix.name), { withFileTypes: true });
      for (const entry of entries) {
        const relativePath = join(prefix.name, entry.name);
        const hash = `${prefix.name}${entry.name}`;
        if (!entry.isFile() || !/^[0-9a-f]{62}$/.test(entry.name)) {
          unexpectedEntries.push(relativePath);
          continue;
        }
        objectsChecked += 1;
        try {
          await this.readBlob(hash);
        } catch {
          corruptObjects.push(hash);
        }
      }
    }

    return {
      valid: corruptObjects.length === 0 && unexpectedEntries.length === 0,
      objectsChecked,
      corruptObjects,
      unexpectedEntries,
    };
  }
}
