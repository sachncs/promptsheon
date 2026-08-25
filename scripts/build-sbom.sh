#!/usr/bin/env bash
# Build a Software Bill of Materials (SPDX-style JSON) for promptsheon.
# Combines npm/pnpm lockfiles from frontend, packages/server, and
# packages/shared into a single document. Run from repo root.

set -euo pipefail
cd "$(dirname "$0")/.."

OUT=docs/compliance/sbom.json

# Collect packages from each lockfile
node - <<'JS' > "$OUT"
const fs = require('fs');
const path = require('path');

/**
 * Minimal pnpm v9 yaml parser. We don't pull in a YAML library —
 * just grep the lines we need: `name@version:`, `version: x.y.z`,
 * `resolution: {integrity: ...}`. This is enough for an SBOM.
 */
function readPnpmYaml(p) {
  if (!fs.existsSync(p)) return { packages: {} };
  const text = fs.readFileSync(p, 'utf-8');
  const lines = text.split('\n');
  const packages = {};
  let curName = null;
  let inPackages = false;
  for (const line of lines) {
    if (line.match(/^packages:/)) {
      inPackages = true;
      continue;
    }
    if (inPackages && /^[a-z]/.test(line) && !line.startsWith('  ')) {
      // We left the packages: section.
      inPackages = false;
    }
    if (!inPackages) continue;
    const nameMatch = line.match(/^ {2}'?([^:]+?)'?:\s*$/);
    if (nameMatch) {
      curName = nameMatch[1].replace(/\/$/, '');
      continue;
    }
    const verMatch = line.match(/^\s+version:\s*['"]?([^'"]+)['"]?\s*$/);
    if (verMatch && curName) {
      packages[curName] = { ...packages[curName], version: verMatch[1] };
    }
    const resMatch = line.match(/^\s+(?:resolution|resolved):\s*['"]?([^'"\s]+)['"]?\s*$/);
    if (resMatch && curName) {
      packages[curName] = { ...packages[curName], resolved: resMatch[1].replace(/integrity:/, '') };
    }
  }
  return { packages };
}

function readLockfile(p) {
  if (!fs.existsSync(p)) return { packages: {} };
  if (p.endsWith('.yaml')) return readPnpmYaml(p);
  try {
    const lock = JSON.parse(fs.readFileSync(p, 'utf-8'));
    const out = { packages: {} };
    if (lock.packages) {
      // pnpm v9+ lockfile: lock.packages['node_modules/<name>']
      for (const [k, v] of Object.entries(lock.packages)) {
        const m = k.match(/^node_modules\/(.+?)(?:@[^/]+)?$/);
        if (m) out.packages[m[1]] = { version: v.version || '', resolved: v.resolved || '' };
      }
    }
    return out;
  } catch (e) {
    return { packages: {}, error: e.message };
  }
}

const sources = [
  'frontend/package.json',
  'packages/server/package.json',
  'packages/shared/package.json',
  'packages/cli/package.json',
  'packages/sdk/package.json',
].map((rel) => {
  const pkgPath = path.join(process.cwd(), rel);
  if (!fs.existsSync(pkgPath)) return { rel, pkg: null };
  const pkg = JSON.parse(fs.readFileSync(pkgPath, 'utf-8'));
  return { rel, pkg };
});

const lockfiles = [
  'pnpm-lock.yaml',
  'frontend/pnpm-lock.yaml',
  'packages/server/pnpm-lock.yaml',
  'packages/shared/pnpm-lock.yaml',
].filter((p) => fs.existsSync(p)).map((p) => ({
  rel: p,
  ...readLockfile(p),
}));

const out = {
  spdxVersion: 'SPDX-2.3',
  dataLicense: 'CC0-1.0',
  SPDXID: 'SPDXRef-DOCUMENT',
  name: 'promptsheon',
  documentNamespace: 'https://github.com/sachncs/promptsheon',
  creationInfo: {
    created: new Date().toISOString(),
    creators: ['Tool: scripts/build-sbom.sh'],
  },
  packages: lockfiles.reduce((acc, lock) => {
    Object.assign(acc, lock.packages);
    return acc;
  }, {}),
  directDependencies: sources
    .filter((s) => s.pkg)
    .map((s) => ({
      component: s.rel,
      dependencies: Object.entries(s.pkg.dependencies ?? {}).map(([name, version]) => ({ name, version })),
      devDependencies: Object.entries(s.pkg.devDependencies ?? {}).map(([name, version]) => ({ name, version })),
    })),
  lockfiles: lockfiles.map((l) => ({ rel: l.rel, packages: Object.keys(l.packages).length })),
};

process.stdout.write(JSON.stringify(out, null, 2));
JS

echo
echo "SBOM written to $OUT ($(jq '.packages | length' < "$OUT") packages, $(wc -l < "$OUT") lines)"
