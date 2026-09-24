/** A release candidate with its traffic weight. */
export interface CanaryRelease {
  id: string;
  canaryPercent: number;
}

/**
 * Select a release from an active pool using its canary weights.
 * A single release always wins, including when its weight is zero.
 */
export function selectByCanary(
  pool: CanaryRelease[],
  rng: () => number = Math.random,
): string | null {
  if (pool.length === 0) return null;
  if (pool.length === 1) return pool[0].id;
  const total = pool.reduce((sum, release) => sum + release.canaryPercent, 0);
  if (total <= 0) return pool[0].id;
  const target = rng() * total;
  let accumulated = 0;
  for (const release of pool) {
    accumulated += release.canaryPercent;
    if (target < accumulated) return release.id;
  }
  return pool[pool.length - 1].id;
}
