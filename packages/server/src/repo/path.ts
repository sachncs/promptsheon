import { RepoTreeEntry } from '@promptsheon/shared';

export function normalizePath(input: string): string {
  let p = input.replace(/^\/+/, '').replace(/\/+$/, '');
  if (p === '' || p === '.') return '';
  const parts: string[] = [];
  for (const seg of p.split('/')) {
    if (seg === '' || seg === '.') continue;
    if (seg === '..') {
      if (parts.length === 0) throw new Error(`path escapes the repository: ${input}`);
      parts.pop();
      continue;
    }
    parts.push(seg);
  }
  return parts.join('/');
}

export function treeToEntries(
  tree: { entries: RepoTreeEntry[] },
): Array<{ path: string; blobOid: string; size: number }> {
  return tree.entries.map((e) => ({ path: e.path, blobOid: e.blob.oid, size: e.blob.size }));
}
