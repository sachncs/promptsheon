import { CasStore } from '@promptsheon/shared';

const casPath = process.env['PROMPTSHEON_CAS_PATH'];
if (!casPath) throw new Error('PROMPTSHEON_CAS_PATH is required');

const report = await new CasStore(casPath).verifyObjects();
console.log(JSON.stringify(report));
if (!report.valid) process.exitCode = 1;
