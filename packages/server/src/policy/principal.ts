import * as cedar from '@cedar-policy/cedar-wasm';

/**
 * The Cedar principal the authorizer evaluates against.
 *
 * - `User` — a human in an org, identified by the authenticated
 *   request context. The `role` field is the user's
 *   promptsheon role and gates which Cedar policy clauses match.
 * - `Agent` — a workload identity (SVID or API key). The
 *   `classification` field is the data sensitivity clearance
 *   the agent runs at; cross-agent actions require target.classification
 *   ≤ agent.classification (enforced at the gateway, not in Cedar).
 * - `System` — a bootstrap or replication principal established by
 *   an explicit internal request context.
 *
 * Cedar's `entity { ... }` types must be declared in the schema;
 * see PROMPTSHEON_SCHEMA in `./authorizer.js`.
 */
export type Principal =
  | { type: 'User'; id: string; orgId: string; role: string }
  | { type: 'Role'; id: string; orgId: string; role: string }
  | { type: 'Agent'; id: string; orgId: string; classification: string }
  | { type: 'System'; id: string };

const NAMESPACE = 'promptsheon';

/**
 * Build the Cedar UID shape the engine expects for a principal.
 * `__entity: { type, id }` is the canonical wire form.
 */
function uid(principal: Principal): cedar.EntityUidJson {
  switch (principal.type) {
    case 'User':
      return { __entity: { type: `${NAMESPACE}::User`, id: principal.id } };
    case 'Role':
      return { __entity: { type: `${NAMESPACE}::Role`, id: principal.id } };
    case 'Agent':
      return { __entity: { type: `${NAMESPACE}::Agent`, id: principal.id } };
    case 'System':
      return { __entity: { type: `${NAMESPACE}::System`, id: principal.id } };
  }
}

/**
 * Convert the typed principal into the Cedar UID shape the engine
 * expects. Re-exported so the authorizer and the policy:eval CLI
 * don't have to repeat the namespace prefix logic.
 */
export function principalToUid(principal: Principal): cedar.EntityUidJson {
  return uid(principal);
}
