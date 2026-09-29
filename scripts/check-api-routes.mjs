#!/usr/bin/env node

const baseUrl = (process.env.PROMPTSHEON_API_BASE_URL ?? 'http://127.0.0.1:8080').replace(/\/$/, '');
const requestTimeoutMs = 15_000;
const safeUuid = '00000000-0000-0000-0000-000000000000';
const safeHash = '0'.repeat(64);
const mutatingMethods = new Set(['post', 'put', 'patch', 'delete']);

function resolvePath(path) {
  return path.replace(/\{([^}]+)\}/g, (_match, name) => {
    if (name.toLowerCase().includes('hash')) return safeHash;
    if (name.toLowerCase().includes('name') || name.toLowerCase().includes('slug')) return 'contract-smoke';
    return safeUuid;
  });
}

const openapiResponse = await fetch(`${baseUrl}/api/openapi.json`, {
  signal: AbortSignal.timeout(requestTimeoutMs),
});
if (!openapiResponse.ok) throw new Error(`OpenAPI request failed: ${openapiResponse.status}`);

const document = await openapiResponse.json();
const operations = Object.entries(document.paths ?? {}).flatMap(([path, methods]) =>
  Object.keys(methods)
    .filter((method) => ['get', 'post', 'put', 'patch', 'delete'].includes(method))
    .map((method) => ({ method, path })),
);

if (operations.length < 100) {
  throw new Error(`Expected at least 100 documented API operations, found ${operations.length}`);
}

const failures = [];
for (const operation of operations) {
  const url = `${baseUrl}${resolvePath(operation.path)}`;
  const init = {
    method: operation.method.toUpperCase(),
    headers: { accept: 'application/json' },
    signal: AbortSignal.timeout(requestTimeoutMs),
  };
  if (mutatingMethods.has(operation.method)) {
    init.headers['content-type'] = 'application/json';
    init.body = '{}';
  }

  try {
    const response = await fetch(url, init);
    if (response.status >= 500) failures.push(`${operation.method.toUpperCase()} ${operation.path} -> ${response.status}`);
  } catch (error) {
    failures.push(`${operation.method.toUpperCase()} ${operation.path} -> ${error instanceof Error ? error.message : String(error)}`);
  }
}

if (failures.length > 0) {
  console.error(`API contract smoke failed for ${failures.length} operation(s):`);
  for (const failure of failures) console.error(`- ${failure}`);
  process.exitCode = 1;
} else {
  console.log(`API contract smoke passed for ${operations.length} documented operations.`);
}
