import { execFileSync, spawn } from 'node:child_process';
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
function descendantPids(parentPid) {
  if (process.platform === 'win32') return [];
  try {
    const rows = execFileSync('ps', ['-eo', 'pid=,ppid='], { encoding: 'utf8' })
      .trim()
      .split('\n')
      .map((row) => row.trim().split(/\s+/).map(Number))
      .filter(([pid, ppid]) => Number.isInteger(pid) && Number.isInteger(ppid));
    const childrenByParent = new Map();
    for (const [pid, ppid] of rows) {
      const children = childrenByParent.get(ppid) ?? [];
      children.push(pid);
      childrenByParent.set(ppid, children);
    }
    const descendants = [];
    const visit = (pid) => {
      for (const childPid of childrenByParent.get(pid) ?? []) {
        visit(childPid);
        descendants.push(childPid);
      }
    };
    visit(parentPid);
    return descendants;
  } catch {
    return [];
  }
}

function stopChild(child, signal) {
  if (child.pid === undefined) return;
  for (const pid of descendantPids(child.pid)) {
    try {
      process.kill(pid, signal);
    } catch (error) {
      if (error.code !== 'ESRCH') throw error;
    }
  }
  child.kill(signal);
}

function shutdown(signal) {
  if (shuttingDown) return;
  shuttingDown = true;
  for (const child of children) stopChild(child, signal);
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
