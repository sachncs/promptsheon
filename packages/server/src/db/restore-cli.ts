import { restoreDatabase, verifyDatabaseIntegrity } from './backup.js';

const source = process.env['PROMPTSHEON_RESTORE_SOURCE'];
const destination = process.env['PROMPTSHEON_RESTORE_PATH'];
if (!source || !destination) throw new Error('PROMPTSHEON_RESTORE_SOURCE and PROMPTSHEON_RESTORE_PATH are required');
if (process.env['PROMPTSHEON_RESTORE_CONFIRM'] !== 'I_UNDERSTAND') {
  throw new Error('Set PROMPTSHEON_RESTORE_CONFIRM=I_UNDERSTAND to restore a database; stop the server first');
}

const result = await restoreDatabase(source, destination);
const integrity = verifyDatabaseIntegrity(destination);
if (!integrity.valid) throw new Error(`Restored database failed integrity check: ${integrity.result}`);
console.log(JSON.stringify({ ...result, integrity: integrity.result }));
