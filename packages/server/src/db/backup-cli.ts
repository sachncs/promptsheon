import Database from 'better-sqlite3';
import { backupDatabase, verifyDatabaseIntegrity } from './backup.js';

const source = process.env['PROMPTSHEON_DB_PATH'];
const destination = process.env['PROMPTSHEON_BACKUP_PATH'];

if (!source || !destination) {
  throw new Error('PROMPTSHEON_DB_PATH and PROMPTSHEON_BACKUP_PATH are required');
}

const database = new Database(source, { readonly: true });
try {
  const result = await backupDatabase(database, destination);
  const integrity = verifyDatabaseIntegrity(destination);
  if (!integrity.valid) throw new Error(`Backup integrity check failed: ${integrity.result}`);
  console.log(JSON.stringify({ ...result, integrity: integrity.result }));
} finally {
  database.close();
}
