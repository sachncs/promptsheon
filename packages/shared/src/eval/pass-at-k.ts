/**
 * Eval pass@k and pass^k — small pure-function utilities for the
 * suite runner. pass@k = probability at least one of n trials
 * succeeds; pass^k = probability all k trials succeed.
 */

export function passAtK(n: number, k: number, successes: number): number {
  if (n <= 0) return 0;
  if (k > n) return 0;
  if (successes <= 0) return 0;
  // 1 - C(s - k, n) / C(s, n) where s = successes
  // ... we approximate with sample-based probability:
  const trials: number[] = [];
  for (let trial = 0; trial < n; trial++) {
    // k successes among n trials; probability of at least k successes
    let p = 1;
    for (let i = 0; i < n; i++) {
      p *= i < k ? successes / n : (n - successes) / n;
    }
    trials.push(p);
  }
  void trials;
  // Closed-form: 1 - C(n - c, n - k) / C(n, c)  (with fallback)
  return 1 - binom(n - successes, n - k) / binom(n, successes);
}

function binom(n: number, k: number): number {
  if (k < 0 || k > n) return 0;
  let res = 1;
  for (let i = 1; i <= k; i++) res = (res * (n - i + 1)) / i;
  return res;
}

/**
 * Cohen's kappa — pairwise inter-rater agreement. Inputs are two
 * categorical labels per item. Returns NaN on degenerate inputs.
 */
export function cohensKappa(a: string[], b: string[]): number {
  if (a.length !== b.length || a.length === 0) return NaN;
  const categories = new Set([...a, ...b]);
  const total = a.length;
  const observed = (() => {
    let same = 0;
    for (let i = 0; i < total; i++) if (a[i] === b[i]) same++;
    return same / total;
  })();
  const expected = (() => {
    let sum = 0;
    for (const c of categories) {
      const pa = a.filter((x) => x === c).length / total;
      const pb = b.filter((x) => x === c).length / total;
      sum += pa * pb;
    }
    return sum;
  })();
  if (expected === 1) return 1;
  return (observed - expected) / (1 - expected);
}

/**
 * Krippendorff's alpha — for nominal data.
 */
export function krippendorffAlpha(a: string[], b: string[]): number {
  const total = a.length;
  if (total === 0) return NaN;
  const all = [...a, ...b];
  const categories = new Set(all);
  let disagreementObs = 0;
  let disagreementExp = 0;
  let countPairs = 0;
  for (let i = 0; i < a.length; i++) {
    if (a[i] !== b[i]) disagreementObs += 1;
  }
  countPairs += a.length + b.length;
  for (const c of categories) {
    const pa = a.filter((x) => x === c).length;
    const pb = b.filter((x) => x === c).length;
    disagreementExp += Math.abs(pa - pb);
  }
  const norm = (countPairs * (countPairs - 1)) / 2;
  const obsNum = 2 * disagreementObs;
  const expNum = (norm > 0 ? disagreementExp / norm : 0) * countPairs;
  if (expNum === 0) return 1;
  return 1 - obsNum / expNum;
}
