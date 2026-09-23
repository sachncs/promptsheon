import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const children = [
  spawn('pnpm', ['--dir', 'packages/server', 'dev'], {
    cwd: repositoryRoot,
    env: process.env,
    stdio: 'inherit',
  }),
  spawn('pnpm', ['--dir', 'frontend', 'dev'], {
    cwd: repositoryRoot,
    env: process.env,
    stdio: 'inherit',
  }),
];

let shuttingDown = false;
function shutdown(signal) {
  if (shuttingDown) return;
  shuttingDown = true;
  for (const child of children) child.kill(signal);
}

process.on('SIGINT', () => shutdown('SIGINT'));
process.on('SIGTERM', () => shutdown('SIGTERM'));

await new Promise((resolveProcess) => {
  let remaining = children.length;
  for (const child of children) {
    child.once('exit', (code, signal) => {
      if (!shuttingDown && ((code ?? 0) !== 0 || signal !== null)) {
        process.exitCode = code ?? 1;
        shutdown('SIGTERM');
      }
      remaining -= 1;
      if (remaining === 0) resolveProcess();
    });
  }
});
