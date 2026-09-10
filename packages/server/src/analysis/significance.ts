/**
 * Statistical significance for A/B experiment pass-rates.
 *
 * Two methods are provided so callers can pick the one that fits
 * their audience:
 *
 *   - **Frequentist** (two-proportion z-test). Cheap, well-known,
 *     easy to defend. Returns a p-value against the null hypothesis
 *     that the two variants have identical pass-rates.
 *
 *   - **Bayesian** (beta-binomial Monte Carlo). Each variant's
 *     pass-rate is given a Beta(1+passes, 1+fails) posterior;
 *     the probability that variant A beats variant B is the
 *     fraction of posterior samples where p_A > p_B, with a
 *     95% credible interval around each variant's pass-rate.
 *
 * The frequentist p-value is appropriate when the auditor needs a
 * null-hypothesis test ("would we see this delta by chance?"); the
 * Bayesian summary is more useful for product-side decisions
 * ("how confident are we that A > B?"). Both ship; the route layer
 * returns both.
 */

/**
 * Standard normal CDF via the Abramowitz & Stegun approximation
 * to erf. CDF(z) = 0.5 * (1 + erf(z / sqrt(2))). Accurate to
 * ~1.5e-7 across the full real line, no special functions needed.
 */
export function normalCdf(z: number): number {
  const sign = z < 0 ? -1 : 1;
  const abs = Math.abs(z) / Math.SQRT2;
  const t = 1 / (1 + 0.3275911 * abs);
  const a1 = 0.254829592;
  const a2 = -0.284496736;
  const a3 = 1.421413741;
  const a4 = -1.453152027;
  const a5 = 1.061405429;
  const y =
    1 -
    (((((a5 * t + a4) * t) + a3) * t + a2) * t + a1) *
      t *
      Math.exp(-abs * abs);
  const erf = sign * y;
  return 0.5 * (1 + erf);
}

export interface VariantStats {
  label: string;
  cases: number;
  passes: number;
  fails: number;
  passRate: number;
}

export interface PairwiseSignificance {
  a: string;
  b: string;
  /** Two-proportion z-test. Two-sided. */
  zScore: number;
  /** P-value that the two pass-rates are equal. */
  pValue: number;
  /** 95% Wald interval for `p_a - p_b`. */
  meanDiff: number;
  diffCi95: [number, number];
  /** Significant at α=0.05? */
  significant: boolean;
}

export interface BayesianSummary {
  /** P(A > B) estimated from 10k Beta(1+passes,1+fails) draws. */
  probABeatsB: number;
  /** 95% credible interval on pass-rate for variant A. */
  credibleIntervalA: [number, number];
  /** 95% credible interval on pass-rate for variant B. */
  credibleIntervalB: [number, number];
}

export interface SignificanceReport {
  variants: VariantStats[];
  /** Pairs (i, j) for i<j. */
  pairwise: PairwiseSignificance[];
  bayesian: Array<BayesianSummary & { a: string; b: string }>;
  winner: string | null;
  /** Variant stats ranked by pass-rate desc. */
  ranking: string[];
  /** True when at least one pair is significant at α=0.05. */
  anySignificant: boolean;
}

/**
 * Two-proportion z-test (pooled variance). Two-sided p-value.
 *
 * Returns `null` when either sample has zero observations — the
 * caller should treat the result as "insufficient data".
 */
export function twoProportionZTest(
  a: { passes: number; total: number },
  b: { passes: number; total: number },
): { zScore: number; pValue: number; meanDiff: number; diffCi95: [number, number] } | null {
  if (a.total === 0 || b.total === 0) return null;
  const pA = a.passes / a.total;
  const pB = b.passes / b.total;
  const pooled = (a.passes + b.passes) / (a.total + b.total);
  const se = Math.sqrt(pooled * (1 - pooled) * (1 / a.total + 1 / b.total));
  if (se === 0) return null;
  const z = (pA - pB) / se;
  // Two-sided p-value.
  const pValue = 2 * (1 - normalCdf(Math.abs(z)));
  const meanDiff = pA - pB;
  // Wald CI for the difference (unpooled SE).
  const seDiff = Math.sqrt((pA * (1 - pA)) / a.total + (pB * (1 - pB)) / b.total);
  const diffCi95: [number, number] = [meanDiff - 1.96 * seDiff, meanDiff + 1.96 * seDiff];
  return { zScore: z, pValue, meanDiff, diffCi95 };
}

/**
 * Beta quantile via the inverse regularized incomplete beta function
 * approximation. Used for credible-interval endpoints on a Beta
 * posterior.
 *
 * Good enough for credible intervals (4 decimal places is fine for
 * 95% endpoints).
 */
function betaInv(p: number, alpha: number, beta: number): number {
  if (p <= 0) return 0;
  if (p >= 1) return 1;
  // Initial guess from the normal approximation to the Beta mean.
  const mean = alpha / (alpha + beta);
  const variance = (alpha * beta) / ((alpha + beta) ** 2 * (alpha + beta + 1));
  const sd = Math.sqrt(variance);
  // Newton-Raphson on the regularized incomplete beta function.
  let x = mean + sd * (p - 0.5) * 2;
  x = Math.min(Math.max(x, 1e-6), 1 - 1e-6);
  for (let i = 0; i < 64; i += 1) {
    const fx = regularizedIncompleteBeta(x, alpha, beta) - p;
    if (Math.abs(fx) < 1e-9) return x;
    const dfx = betaPdf(x, alpha, beta);
    if (dfx === 0) break;
    x = x - fx / dfx;
    x = Math.min(Math.max(x, 1e-9), 1 - 1e-9);
  }
  return x;
}

function betaPdf(x: number, alpha: number, beta: number): number {
  if (x <= 0 || x >= 1) return 0;
  // ln Beta density; we use the log form to avoid overflow at large α, β.
  const log =
    (alpha - 1) * Math.log(x) +
    (beta - 1) * Math.log(1 - x) -
    (logGamma(alpha) + logGamma(beta) - logGamma(alpha + beta));
  return Math.exp(log);
}

function regularizedIncompleteBeta(x: number, a: number, b: number): number {
  // Continued-fraction expansion. Reference: Numerical Recipes §6.4.
  if (x <= 0) return 0;
  if (x >= 1) return 1;
  const bt = Math.exp(
    logGamma(a + b) - logGamma(a) - logGamma(b) + a * Math.log(x) + b * Math.log(1 - x),
  );
  if (x < (a + 1) / (a + b + 2)) {
    return (bt * betacf(x, a, b)) / a;
  }
  return 1 - (bt * betacf(1 - x, b, a)) / b;
}

function betacf(x: number, a: number, b: number): number {
  const maxIter = 200;
  const eps = 3e-7;
  const qab = a + b;
  const qap = a + 1;
  const qam = a - 1;
  let c = 1;
  let d = 1 - (qab * x) / qap;
  if (Math.abs(d) < 1e-30) d = 1e-30;
  d = 1 / d;
  let h = d;
  for (let m = 1; m <= maxIter; m += 1) {
    const m2 = 2 * m;
    let aa = (m * (b - m) * x) / ((qam + m2) * (a + m2));
    d = 1 + aa * d;
    if (Math.abs(d) < 1e-30) d = 1e-30;
    c = 1 + aa / c;
    if (Math.abs(c) < 1e-30) c = 1e-30;
    d = 1 / d;
    h *= d * c;
    aa = (-(a + m) * (qab + m) * x) / ((a + m2) * (qap + m2));
    d = 1 + aa * d;
    if (Math.abs(d) < 1e-30) d = 1e-30;
    c = 1 + aa / c;
    if (Math.abs(c) < 1e-30) c = 1e-30;
    d = 1 / d;
    const del = d * c;
    h *= del;
    if (Math.abs(del - 1) < eps) break;
  }
  return h;
}

function logGamma(x: number): number {
  // Lanczos approximation; accurate to ~10 sig figs for x > 0.
  const c = [
    76.18009172947146, -86.50532032941677, 24.01409824083091, -1.231739572450155,
    0.1208650973866179e-2, -0.5395239384953e-5,
  ];
  let y = x;
  let series = 1.000000000190015;
  for (let j = 0; j < 6; j += 1) series += c[j]! / ++y;
  const logSqrt2Pi = Math.log(Math.sqrt(2 * Math.PI));
  return (
    -logSqrt2Pi -
    (x + 0.5) * Math.log(x + 5.5) +
    Math.log(series) -
    (x + 0.5) +
    Math.log(x)
  );
}

/**
 * Beta(α, β) random sample via two Gamma samples. Marsaglia & Tsang.
 */
function betaSample(alpha: number, beta: number): number {
  const x = gammaSample(alpha);
  const y = gammaSample(beta);
  return x / (x + y);
}

function gammaSample(shape: number): number {
  if (shape < 1) {
    const u = Math.random();
    return gammaSample(shape + 1) * Math.pow(u, 1 / shape);
  }
  const d = shape - 1 / 3;
  const c = 1 / Math.sqrt(9 * d);
  // eslint-disable-next-line no-constant-condition
  while (true) {
    let v: number;
    let x: number;
    do {
      x = randn();
      v = 1 + c * x;
    } while (v <= 0);
    v = v * v * v;
    const u = Math.random();
    if (u < 1 - 0.0331 * x * x * x * x) return d * v;
    if (Math.log(u) < 0.5 * x * x + d * (1 - v + Math.log(v))) return d * v;
  }
}

// Box-Muller; we use a single draw and discard the second.
function randn(): number {
  const u = Math.random() || 1e-12;
  const v = Math.random();
  return Math.sqrt(-2 * Math.log(u)) * Math.cos(2 * Math.PI * v);
}

/**
 * Bayesian summary: P(A > B) and credible intervals via Monte Carlo.
 */
export function bayesianCompare(
  a: { passes: number; fails: number },
  b: { passes: number; fails: number },
  options: { samples?: number } = {},
): BayesianSummary {
  const samples = options.samples ?? 10_000;
  const alphaA = 1 + a.passes;
  const betaA = 1 + a.fails;
  const alphaB = 1 + b.passes;
  const betaB = 1 + b.fails;
  let aWins = 0;
  const drawsA: number[] = [];
  const drawsB: number[] = [];
  for (let i = 0; i < samples; i += 1) {
    const pA = betaSample(alphaA, betaA);
    const pB = betaSample(alphaB, betaB);
    if (pA > pB) aWins += 1;
    drawsA.push(pA);
    drawsB.push(pB);
  }
  drawsA.sort();
  drawsB.sort();
  const q025 = (xs: number[]) => xs[Math.floor(xs.length * 0.025)]!;
  const q975 = (xs: number[]) => xs[Math.floor(xs.length * 0.975)]!;
  return {
    probABeatsB: aWins / samples,
    credibleIntervalA: [q025(drawsA), q975(drawsA)],
    credibleIntervalB: [q025(drawsB), q975(drawsB)],
  };
}

/**
 * Build the full significance report for an experiment. Caller
 * supplies per-variant counts (label + passes + total). Returns
 * null when there is fewer than one variant with at least one
 * observation — the caller should treat that as "no signal".
 */
export function buildSignificanceReport(
  variants: VariantStats[],
  options: { alpha?: number; bayesSamples?: number } = {},
): SignificanceReport | null {
  const observed = variants.filter((v) => v.cases > 0);
  if (observed.length === 0) return null;

  const alpha = options.alpha ?? 0.05;
  const bayesSamples = options.bayesSamples ?? 10_000;

  const pairwise: PairwiseSignificance[] = [];
  const bayesian: Array<BayesianSummary & { a: string; b: string }> = [];
  for (let i = 0; i < observed.length; i += 1) {
    for (let j = i + 1; j < observed.length; j += 1) {
      const va = observed[i]!;
      const vb = observed[j]!;
      const ft = twoProportionZTest(
        { passes: va.passes, total: va.cases },
        { passes: vb.passes, total: vb.cases },
      );
      if (ft) {
        pairwise.push({
          a: va.label,
          b: vb.label,
          zScore: ft.zScore,
          pValue: ft.pValue,
          meanDiff: ft.meanDiff,
          diffCi95: ft.diffCi95,
          significant: ft.pValue < alpha,
        });
      }
      bayesian.push({
        a: va.label,
        b: vb.label,
        ...bayesianCompare(
          { passes: va.passes, fails: va.fails },
          { passes: vb.passes, fails: vb.fails },
          { samples: bayesSamples },
        ),
      });
    }
  }

  // Winner: the variant with the highest pass-rate whose pairwise
  // test against the runner-up is significant. If no pair is
  // significant, the experiment is inconclusive — winner is null.
  const sorted = [...observed].sort((a, b) => b.passRate - a.passRate);
  const ranking = sorted.map((v) => v.label);
  let winner: string | null = null;
  if (pairwise.length > 0) {
    const top = sorted[0]!;
    const runnerUp = sorted[1] ?? null;
    if (runnerUp) {
      const sig = pairwise.find(
        (p) => (p.a === top.label && p.b === runnerUp.label) || (p.b === top.label && p.a === runnerUp.label),
      );
      if (sig && sig.significant) winner = top.label;
    }
  }

  return {
    variants: observed,
    pairwise,
    bayesian,
    winner,
    ranking,
    anySignificant: pairwise.some((p) => p.significant),
  };
}