import type { Manifest } from '@promptsheon/shared';

/**
 * Validate the structural integrity of a Manifest DAG.
 * Returns a list of human-readable errors. Empty array means valid.
 *
 * Checks:
 * - All node ids are unique
 * - All edges reference existing node ids (no dangling)
 * - DAG is acyclic (iterative DFS with white/gray/black coloring)
 * - No self-loops
 */
export function validateDag(manifest: Manifest): { valid: boolean; errors: string[] } {
  const errors: string[] = [];

  const ids = new Set<string>();
  for (const node of manifest.nodes) {
    if (ids.has(node.id)) {
      errors.push(`duplicate node id: ${node.id}`);
    }
    ids.add(node.id);
  }

  for (const edge of manifest.edges) {
    if (edge.from === edge.to) {
      errors.push(`self-loop on node ${edge.from}`);
    }
    if (!ids.has(edge.from)) {
      errors.push(`edge ${edge.from}->${edge.to} references missing source ${edge.from}`);
    }
    if (!ids.has(edge.to)) {
      errors.push(`edge ${edge.from}->${edge.to} references missing target ${edge.to}`);
    }
  }

  const adj = new Map<string, string[]>();
  for (const id of ids) adj.set(id, []);
  for (const edge of manifest.edges) {
    if (ids.has(edge.from) && ids.has(edge.to)) {
      adj.get(edge.from)!.push(edge.to);
    }
  }

  const color = new Map<string, 0 | 1 | 2>();
  for (const id of ids) color.set(id, 0);

  const stack: string[] = [];
  function visit(node: string): void {
    const c = color.get(node);
    if (c === 1) {
      errors.push(`cycle detected: ${[...stack, node].join(' -> ')}`);
      return;
    }
    if (c === 2) return;
    color.set(node, 1);
    stack.push(node);
    for (const next of adj.get(node) ?? []) {
      visit(next);
    }
    stack.pop();
    color.set(node, 2);
  }
  for (const id of ids) {
    if (color.get(id) === 0) visit(id);
  }

  return { valid: errors.length === 0, errors };
}