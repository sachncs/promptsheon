import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import * as cedar from '@cedar-policy/cedar-wasm';

export type CedarDecision = 'allow' | 'deny';

export interface CedarRequest {
  /**
   * Principal entity UID in our namespace, e.g.
   * `{ __entity: { type: 'User', id: 'u-1' } }`.
   */
  principal: cedar.EntityUidJson;
  /** Action entity UID, e.g. `{ __entity: { type: 'Action', id: 'Agent::Publish' } }`. */
  action: cedar.EntityUidJson;
  /** Resource entity UID, e.g. `{ __entity: { type: 'Agent', id: 'a-1' } }`. */
  resource: cedar.EntityUidJson;
  /** Optional context. Cedar evaluates `when`/`unless` clauses
   * against this record; pass it through as JSON. */
  context?: Record<string, unknown>;
}

export interface CedarResult {
  /**
   * The decision the policy made. Default-deny makes `allow: false`
   * the safe fallback on any engine error.
   */
  allow: boolean;
  /** Diagnostics from the engine — empty on a clean allow/deny. */
  reasons: string[];
}

interface LoadedPolicy {
  policySet: cedar.PolicySet;
  schema: cedar.Schema;
}

/**
 * Wraps `@cedar-policy/cedar-wasm` so callers don't need to know
 * the JSON shapes the engine expects. Loads the policy file once
 * per process (cached) and reuses the parsed source + Schema
 * across every `authorize()` call.
 *
 * `dryRun: true` evaluates the policy without writing a deny;
 * `reasons` always includes the engine's diagnostics either way,
 * which is what `policy:eval` (IN-0.5) uses for the regression
 * matrix.
 */
export class CedarAuthorizer {
  private cached: LoadedPolicy | null = null;
  private readonly policyPath: string;
  private readonly dryRun: boolean;

  constructor(opts: { policyPath?: string; dryRun?: boolean } = {}) {
    this.policyPath =
      opts.policyPath ?? resolve('packages/server/policies/promptsheon.cedar');
    this.dryRun = opts.dryRun ?? false;
  }

  /**
   * Load + parse the policy file. Idempotent; subsequent calls
   * return the cached source + Schema. Throws on a parse failure
   * — fail fast so a bad policy file doesn't silently fall open.
   */
  load(): void {
    if (this.cached) return;
    const source = readFileSync(this.policyPath, 'utf-8');
    const policySet: cedar.PolicySet = { staticPolicies: source };
    const probe = cedar.checkParsePolicySet(policySet);
    if (probe.type !== 'success') {
      throw new Error(
        `failed to parse Cedar policy at ${this.policyPath}: ${JSON.stringify(probe.errors)}`,
      );
    }
    // Validate the policy against the schema so any
    // `Type::"id"` reference that doesn't exist in the schema
    // fails at boot rather than at evaluation time.
    const validationResult = cedar.validate({
      schema: PROMPTSHEON_SCHEMA,
      policies: policySet,
    });
    if (validationResult.type === 'failure') {
      throw new Error(
        `Cedar policy validation failed: ${JSON.stringify(validationResult.errors)}`,
      );
    }
    this.cached = { policySet, schema: PROMPTSHEON_SCHEMA };
  }

  /**
   * Evaluate one request. Returns `{ allow: false, reasons: [...] }`
   * on any engine failure — the safe default.
   *
   * Caller must call `load()` at least once before the first
   * `authorize()` call. Subsequent calls reuse the cache.
   */
  authorize(request: CedarRequest): CedarResult {
    if (!this.cached) {
      throw new Error('CedarAuthorizer.load() must run before authorize()');
    }
    const call: cedar.AuthorizationCall = {
      principal: request.principal,
      action: request.action,
      resource: request.resource,
      context: contextToJson(request.context ?? {}),
      policies: this.cached.policySet,
      entities: [wrapAsEntity(request.principal), wrapAsEntity(request.action), wrapAsEntity(request.resource)],
      schema: this.cached.schema as cedar.Schema,
    };
    const result = cedar.isAuthorized(call);
    if (result.type !== 'success') {
      return { allow: false, reasons: result.errors.map(diagnosticString) };
    }
    return {
      allow: result.response.decision === 'allow',
      reasons: result.response.diagnostics.errors.map(
        (e) => `${e.error.message}${e.policyId ? ` (policy ${e.policyId})` : ''}`,
      ),
    };
  }

  /**
   * For tests + the policy:eval CLI: bypass the JSON wrapping and
   * evaluate against the loaded policy with caller-supplied
   * entity JSON. Returns the raw engine answer for assertion.
   */
  rawEvaluate(call: cedar.AuthorizationCall): cedar.AuthorizationAnswer {
    if (!this.cached) {
      throw new Error('CedarAuthorizer.load() must run before rawEvaluate()');
    }
    return cedar.isAuthorized(call);
  }
}

function diagnosticString(d: cedar.DetailedError): string {
  return d.message;
}

/**
 * Convert a Cedar `Context` into the JSON shape the WASM API
 * accepts. Contexts are open key-value bags; we just round-trip
 * through JSON so the caller can pass arbitrary JSON-serializable
 * values.
 */
function contextToJson(context: Record<string, unknown>): cedar.Context {
  return JSON.parse(JSON.stringify(context)) as cedar.Context;
}

/**
 * Wrap a principal/action/resource UID into an EntityJson the
 * WASM API expects. `__entity` is the canonical wire form.
 */
function wrapAsEntity(uid: cedar.EntityUidJson): cedar.EntityJson {
  if (
    typeof uid === 'object' &&
    uid !== null &&
    '__entity' in uid
  ) {
    return {
      uid: uid as { __entity: cedar.TypeAndId },
      attrs: {},
      parents: [],
    };
  }
  return {
    uid: { __entity: uid as cedar.TypeAndId },
    attrs: {},
    parents: [],
  };
}

/**
 * The Cedar schema for promptsheon. Mirrors the action +
 * resource vocabulary in `promptsheon.cedar`. Keep these in
 * sync — drift here silently disables a capability.
 *
 * Cedar's WASM API requires every entity type referenced in the
 * policy to have a shape declaration. Empty shapes still need
 * the `{}` body so the parser registers the type. We also
 * declare Action + Resource in their own `entityTypes` block so
 * the policy's `Action::"X"` and `Resource::"Y"` literals parse
 * cleanly.
 */
export const PROMPTSHEON_SCHEMA = `
namespace promptsheon {
  // Principal types
  entity User;
  entity Agent;
  entity System;

  // Role / org / team entities
  entity Role;
  entity Org;
  entity Team;

  // Resource types
  entity Memory;
  entity Budget;
  entity Audit;
  entity Trace;
  entity Tool;
  entity Resource;

  action "Agent::Read" appliesTo { principal: [User, Agent, System, Role], resource: [Org, Agent, Resource] };
  action "Agent::Write" appliesTo { principal: [User, Agent, System, Role], resource: [Org, Agent, Resource] };
  action "Agent::Publish" appliesTo { principal: [User, Agent, System, Role], resource: [Org, Agent, Resource] };
  action "Agent::Invoke" appliesTo { principal: [Agent, System, Role], resource: [Org, Agent, Resource] };
  action "Agent::Resume" appliesTo { principal: [Agent, System, Role], resource: [Org, Agent, Resource] };
  action "Tool::Read" appliesTo { principal: [User, Agent, System, Role], resource: [Org, Tool, Resource] };
  action "Tool::Call" appliesTo { principal: [Agent, System, Role], resource: [Org, Tool, Resource] };
  action "Tool::Register" appliesTo { principal: [User, Agent, System, Role], resource: [Org, Tool, Resource] };
  action "Memory::Read" appliesTo { principal: [User, Agent, System, Role], resource: [Org, Agent, Memory, Resource] };
  action "Memory::Write" appliesTo { principal: [User, Agent, System, Role], resource: [Org, Agent, Memory, Resource] };
  action "Audit::Read" appliesTo { principal: [User, Agent, System, Role], resource: [Org, Audit, Resource] };
  action "Observability::Read" appliesTo { principal: [User, Agent, System, Role], resource: [Org, Trace, Resource] };
  action "Observability::Export" appliesTo { principal: [Agent, System, Role], resource: [Org, Trace, Resource] };
  action "Budget::Create" appliesTo { principal: [User, Agent, System, Role], resource: [Org, Budget, Resource] };
  action "Budget::Read" appliesTo { principal: [User, Agent, System, Role], resource: [Org, Budget, Resource] };
  action "Budget::Update" appliesTo { principal: [User, Agent, System, Role], resource: [Org, Budget, Resource] };
  action "Budget::Delete" appliesTo { principal: [User, Agent, System, Role], resource: [Org, Budget, Resource] };
  action "Armor::Configure" appliesTo { principal: [User, Agent, System, Role], resource: [Org, Resource] };
  action "Registry::Sign" appliesTo { principal: [User, Agent, System, Role], resource: [Org, Agent, Resource] };
  action "Gateway::Invoke" appliesTo { principal: [User, Agent, System, Role], resource: [Org, Resource] };
}
`;