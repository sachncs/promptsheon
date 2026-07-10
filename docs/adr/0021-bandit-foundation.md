# ADR 0021: Multi-armed bandit recommender foundation (Thompson Sampling)

- **Status:** Accepted
- **Date:** 2026-07-10
- **Supersedes:** —
- **Superseded by:** —

## Context

The architecture review board (Tier 2.35, M4 follow-on)
called for a multi-armed bandit recommender that complements the
deterministic rules engine (`internal/optimizer/rules`).
Bandit policies are the right tool when the Recommendation system
must choose between competing Release variants without knowing
in advance which is best; the bandit explores while the rules
engine acts on observable thresholds.

The existing `internal/abtesting.Engine` uses static
traffic-weighted random selection. That is fine for
producer-side A/B testing but it is not a learning algorithm
— a Release variant that is failing 90% of the time still gets
10% traffic until a human notices.

## Decision

The bandit package ships the Thompson Sampling selection
algorithm: each arm maintains a Beta(alpha, beta) posterior over
its true success rate; the next arm is the one with the highest
sample drawn from its posterior. Arms that have been pulled many
times have tight posteriors (low variance); arms that have not
been pulled have wide posteriors (high variance) which is the
natural exploration mechanism.

The full bandit recommender — Bayesian Thompson arms keyed
on Recommendation acceptance, persistent posteriors across
restarts, bandit-aware Recommendations surfaced to operators —
ships in a follow-on commit. Today's commit is the algorithm
core: a `Selector` type that takes arm IDs, records outcomes,
and returns the highest-posterior arm. It is concurrency-safe.

The arm-selection policy is intentionally simple because it
ships alongside the deterministic rules engine. Production
operators can run both: rules trigger Recommendations on
threshold breaches; bandit arms surface "this version is being
underused despite a high success rate" Recommendations.

## Options considered

1. **Static traffic weighting (today's Engine).** Rejected: a
   non-learning algorithm doesn't close the optimisation loop.
2. **UCB1 (deterministic upper-confidence bound).** Rejected as
   primary: UCB1 is deterministic given enough trials, which
   makes it harder to debug. Thompson Sampling is Bayesian and
   produces posterior distributions, not point estimates, which
   is more useful for the recommendation engine's downstream
   consumers.
3. **Thompson Sampling (chosen).** Standard bandit policy;
   simple to implement (one Beta sample per arm); exploration
   emerges from the Bayesian posterior; supported by decades of
   applied research and shipped in production at companies
   like Spotify, Netflix, and Booking.com.

## Consequences

Positive:

- The bandit arm is a real learning algorithm; the abtesting
  Engine can swap its static selection for bandit selection
  by calling `bandit.NewSelector(armIDs)` and updating on
  each outcome.
- The Recommendation engine v2 (M4 follow-on) can read
  `Selector.PosteriorMean(armID)` to emit "this variant is
  outperforming" Recommendations.
- Tests cover the exploitation/exploration trade-off
  deterministically by seeding the RNG.

Negative:

- The Selector is in-memory; the bandit recommender that ships
  in M4 follow-on will need a persistent arm-posterior store.
- The Beta(1, 1) uniform prior is the standard "no-information"
  choice; a Jeffreys prior (Beta(0.5, 0.5)) is sometimes
  preferred for binary rewards. The follow-on M4 commit can
  switch priors behind a config.

## References

- `internal/bandit/selector.go` — Thompson Sampling selector
- `internal/bandit/selector_test.go` — table-driven suite
- Architecture Review §21 Tier 2.35.
- ADR-0019: deferred items include the M4 bandit recommender
  and the LLM-judge engine; today's commit is the algorithm
  foundation; the production engine ships in a follow-on.
