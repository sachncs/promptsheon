# Phase 7 — Product UX, UI, and developer experience

## Objective

Make the platform understandable and efficient for builders, developers, and
operators without hiding important state or evidence.

## Responsible

- Product lead: user journeys, priorities, language, acceptance criteria.
- UX/UI lead: information architecture, interaction design, accessibility.
- Frontend: Next.js implementation, state, forms, performance, E2E.
- Backend/DX: API ergonomics and error contracts.
- Design systems owner: tokens, components, responsive rules, visual QA.

## Implementation

- Define primary navigation around Agents, Executions, Evaluations, Evidence,
  Releases, Policies, and Operations.
- Build the critical flows:
  - create and validate an agent,
  - execute and inspect evidence,
  - evaluate a candidate,
  - review a mutation proposal,
  - approve and canary a release,
  - monitor and roll back.
- Show agent hash, revision, lifecycle, and environment consistently.
- Add contextual empty states with one clear next action.
- Add comparison views for revisions, candidates, costs, latency, and quality.
- Keep advanced policies behind progressive disclosure.
- Standardize loading, error, retry, success, and optimistic states.
- Use TanStack Query for server state and typed forms with Zod.
- Preserve keyboard navigation, focus states, reduced motion, and screen-reader
  labels.
- Remove dead-end redirects to the repository for user workflows.
- Optimize bundles, data fetching, image sizes, and initial render paths.

## Reliability and throughput

- Make UI state derived from server state where possible.
- Prevent duplicate mutations with disabled states and idempotency keys.
- Cancel stale queries and long-running execution views.
- Paginate and virtualize large evidence and execution lists.
- Avoid loading full traces or datasets on initial page load.
- Add clear stale-data and refresh semantics.
- Use error boundaries and route-level recovery.
- Track frontend performance, API latency, failed mutations, and abandonment.

## Testing

- Component tests for forms, state transitions, errors, and accessibility.
- Playwright journeys for the six critical flows.
- Responsive tests at supported viewport sizes.
- Keyboard-only and screen-reader-oriented checks.
- Visual regression checks for key pages and themes.
- Network failure, timeout, stale data, and permission-denied tests.
- Performance tests for initial load, route transitions, and large evidence lists.
- Contract tests against API error envelopes and pagination.

## Deliverables

- Product information architecture.
- Design tokens and reusable component system.
- Agent, execution, evaluation, evidence, and release workflows.
- Responsive accessible UI.
- Frontend performance and UX telemetry.

## Exit criteria

- A new user can complete the critical path without repository knowledge.
- A developer can find an API/SDK/CLI integration path from the product UI.
- An operator can inspect, approve, promote, and roll back without ambiguity.
- Key workflows work on mobile, keyboard, and assistive technology.

