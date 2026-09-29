# Testing and flaky-test policy

Promptsheon treats a flaky test as a reliability defect, not as a reason to
retry CI until it passes.

## Required test behaviour

- CI does not automatically retry failed tests.
- A failed test blocks the workflow until the failure is understood or the test
  is explicitly quarantined.
- Every quarantined test must have an owner, a tracking issue, the observed
  failure mode, and a removal deadline.
- Quarantine is temporary and must not reduce the required quality gate without
  an explicit engineering-lead decision.
- New tests must cover the failure path they are intended to protect, not only
  the successful response.

## Quarantine process

1. Reproduce the failure locally and in a clean checkout.
2. Classify it as a product defect, test defect, environment defect, or genuine
   nondeterminism.
3. Fix the product or test defect immediately when possible.
4. If quarantine is unavoidable, add an entry to
   [`docs/baseline/flaky-tests.md`](baseline/flaky-tests.md).
5. Assign an owner and issue, then set a deadline no more than 14 days away.
6. Remove the quarantine entry when the test is deterministic and green.

## Failure budget

The current failure budget is zero: there are no approved quarantined tests.
Any exception must be visible in the register and reviewed before merge.

