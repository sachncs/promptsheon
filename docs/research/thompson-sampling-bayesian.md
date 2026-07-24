# Thompson Sampling is already Bayesian — replacement criteria

Research note. The current bandit selector in
`internal/bandit/selector.go` uses Thompson Sampling with
a Beta(`α`, `β`) posterior per arm. This is the textbook
Bayesian multi-armed-bandit policy: each arm carries a
posterior over its success rate, every decision draws one
sample from each posterior, and the chosen arm is the one
with the highest sample. The exploration is the natural
posterior uncertainty, not an added ε-greedy term.

This note exists to acknowledge that the existing
implementation is already Bayesian, and to give the
maintainers a clear criterion for when a replacement
selector would be worth the migration cost.

## 1. Why "Bayesian" is the right default

The current selector:

- uses a Beta(1, 1) prior (a uniform prior over `[0, 1]`);
- updates the posterior with each observation
  (`Observe(success)` increments `α` or `β`);
- draws a per-decision sample from the posterior
  (`Sample(rng)` runs two Gamma draws + Marsaglia-Tsang);
- chooses the arm with the highest sample.

This is the Bayesian answer to the explore-vs-exploit
problem. The Beta-Bernoulli conjugate pair is the simplest
conjugate posterior that fits the "binary outcome" use
case (success / failure). For multi-objective or
heterogeneous outcomes, the policy extends to a
Gamma-Poisson or Dirichlet-Multinomial pair without
changing the algorithm.

## 2. When a replacement is justified

A replacement selector is justified if, and only if, one
of the following is true:

1. **Conjugate posterior limit is binding.** The
   Beta-Bernoulli pair is only a good fit when the
   outcome is binary and the observations are i.i.d.
   If the outcome is "reward in `ℝ`" (e.g. latency
   savings, dollars saved) the Beta-Bernoulli pair is
   the wrong shape. *Replacement candidate*: a Gaussian
   posterior with a Normal-Inverse-Gamma prior.

2. **Cold-start cost is too high.** With a uniform
   Beta(1, 1) prior and arms that have not been
   observed, the first N observations per arm are
   effectively uniform draws. If the operator's SLO
   requires a smarter warm-start (e.g. "bias the
   recommendation toward the most recent successful
   release"), the prior needs to be informative.
   *Replacement candidate*: a Thompson sampling with a
   prior seeded from a labelled dataset or a curated
   "default arm" with weight 2.

3. **The reward is delayed or compound.** If the
   outcome cannot be observed within the same request
   (e.g. capability quality is measured weeks later),
   the Beta-Bernoulli pair is too local. *Replacement
   candidate*: a contextual bandit with a posterior
   conditioned on context features (e.g. capability
   embeddings, deployment region).

4. **The cost of a wrong arm is asymmetric.** Thompson
   Sampling assumes the cost of a wrong arm is the same
   as the cost of a right arm not chosen. If a wrong
   arm is dramatically more expensive (e.g. an
   out-of-budget capability), the algorithm should
   minimise tail risk, not expected regret. *Replacement
   candidate*: a CVaR-aware bandit or a constrained
   MDP.

## 3. When a replacement is NOT justified

A replacement is NOT justified if the goal is "more
exploration" or "less regret". The existing Thompson
Sampling selector already explores; the regret upper
bound is the standard `O(√(KT log T))` for the Beta-
Bernoulli pair. Adding ε-greedy on top of Thompson
Sampling is a regression, not an improvement.

A replacement is also NOT justified if the goal is
"smarter bandits" without a measurement of how the
existing selector is misbehaving. The replacement needs
a baseline and a target metric; otherwise the migration
is a rewrite for its own sake.

## 4. Status

- **Shipped in v0.2.0**: this research note. No code.
- **NOT shipped in v0.2.0**: any replacement selector.
- **Tracking**: the criteria above are the only accepted
  triggers for a replacement PR. A PR that wants to
  replace the selector without meeting one of the four
  criteria is going to be rejected.
