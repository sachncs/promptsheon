# API compatibility policy

This policy defines how Promptsheon evolves its HTTP API, SDK, CLI, and
persisted data. It applies to supported product surfaces; internal modules may
change when all first-party callers and tests are updated in the same change.

## Compatibility promise

- The current API contract is the unversioned `/api` surface.
- Additive response fields and new endpoints are backwards-compatible changes.
- Existing fields retain their meaning and types during a supported lifecycle.
- Error responses keep the shape `{ "error": { "code": string, "message": string } }`.
- Published agent specifications, manifests, evaluation results, evidence, and
  releases are immutable. New schema versions use explicit migrations.
- SDK and CLI commands must remain compatible with the supported API contract,
  or document a minimum server version before release.

## Breaking changes

A change is breaking when it removes or renames an endpoint, field, command, or
behaviour; changes a field type; changes authentication requirements; changes
the meaning of a persisted value; or invalidates a previously valid agent
specification.

Breaking changes require:

1. An entry in `CHANGELOG.md` under the unreleased version.
2. Migration guidance in the relevant developer documentation.
3. Updated first-party clients, UI flows, fixtures, and contract tests.
4. An architectural decision record when the change affects a package boundary,
   persistence model, or production lifecycle.
5. A release note identifying affected versions and rollback considerations.

Do not add permanent compatibility wrappers to avoid updating first-party
callers. Keep a wrapper only when an external consumer needs a documented
deprecation window and the wrapper has an owner and removal version.

## Deprecation process

- Mark the contract as deprecated in the API, SDK, CLI, and documentation.
- Emit a structured deprecation signal where the protocol allows it.
- Keep the old path tested during the deprecation window.
- Document the replacement and migration example.
- Remove the path only after the removal version is announced.

## Review checklist

- Does the change preserve the error envelope?
- Does it preserve organization and workspace scoping?
- Does it preserve idempotency and pagination semantics?
- Does it preserve immutable hash lineage?
- Are migrations forward-compatible and rollback-aware?
- Are API, SDK, CLI, frontend, public site, and examples synchronized?
- Are contract, integration, and failure-path tests included?

