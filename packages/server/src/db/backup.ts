import Database from 'better-sqlite3';
import { chmod, copyFile, mkdir, rename, rm } from 'node:fs/promises';
import { dirname } from 'node:path';

export interface DatabaseBackupResult {
  destination: string;
  totalPages: number;
  remainingPages: number;
}

export interface DatabaseIntegrityResult {
  valid: boolean;
  result: string;
}

export interface DatabaseRestoreResult {
  source: string;
  destination: string;
}

/**
 * Create an atomic SQLite backup beside the live database.
 *
 * SQLite's online backup API keeps the source database available while the
 * copy is taken. The temporary file is renamed only after the copy succeeds,
 * so an interrupted backup cannot replace the last known-good artifact.
 */
export async function backupDatabase(
  database: Database.Database,
  destination: string,
): Promise<DatabaseBackupResult> {
  const temporaryDestination = `${destination}.tmp`;
  await mkdir(dirname(destination), { recursive: true });
  await rm(temporaryDestination, { force: true });

  try {
    const metadata = await database.backup(temporaryDestination);
    await chmod(temporaryDestination, 0o600);
    await rename(temporaryDestination, destination);
    return {
      destination,
      totalPages: metadata.totalPages,
      remainingPages: metadata.remainingPages,
    };
  } catch (error) {
    await rm(temporaryDestination, { force: true });
    throw error;
  }
}

/** Verify a SQLite backup and atomically restore it to a stopped database path. */
export async function restoreDatabase(source: string, destination: string): Promise<DatabaseRestoreResult> {
  const integrity = verifyDatabaseIntegrity(source);
  if (!integrity.valid) throw new Error(`Restore source failed integrity check: ${integrity.result}`);
  const temporaryDestination = `${destination}.restore.tmp`;
  await mkdir(dirname(destination), { recursive: true });
  await rm(temporaryDestination, { force: true });
  try {
    await copyFile(source, temporaryDestination);
    await chmod(temporaryDestination, 0o600);
    await rename(temporaryDestination, destination);
    return { source, destination };
  } catch (error) {
    await rm(temporaryDestination, { force: true });
    throw error;
  }
}

/**
 * Run SQLite's integrity check against a backup without mutating it.
 */
export function verifyDatabaseIntegrity(databasePath: string): DatabaseIntegrityResult {
  const database = new Database(databasePath, { readonly: true });
  try {
    const row = database.pragma('integrity_check', { simple: true }) as unknown;
    const result = typeof row === 'string' ? row : String(row);
    return { valid: result === 'ok', result };
  } catch (error) {
    return {
      valid: false,
      result: error instanceof Error ? error.message : String(error),
    };
  } finally {
    database.close();
  }
}
