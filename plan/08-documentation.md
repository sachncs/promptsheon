# Phase 8 — Developer and operator documentation

## Objective

Make the product self-explanatory, executable, and maintainable without
redirecting users to source code for normal usage.

## Responsible

- Developer experience: information architecture and examples.
- Backend/SDK/CLI owners: API accuracy and generated references.
- SRE/security: deployment, operations, and incident procedures.
- Product/frontend: in-product links and terminology consistency.

## Implementation

- Create documentation sections for:
  - quickstart,
  - product concepts,
  - agent specifications,
  - content addressing,
  - executions,
  - evidence,
  - evaluations,
  - mutations,
  - releases,
  - API,
  - SDK,
  - CLI,
  - architecture,
  - local development,
  - testing,
  - deployment,
  - operations,
  - security,
  - troubleshooting,
  - contributing.
- Write end-to-end examples for the critical product loop.
- Keep API examples synchronized with schemas and error envelopes.
- Add copy-pasteable curl, TypeScript SDK, and CLI examples.
- Document configuration, secrets, persistence, backups, migrations, and
  rollback.
- Add architecture diagrams showing package boundaries and data flow.
- Add troubleshooting by symptom, diagnosis, and remediation.
- Add release notes and migration guidance for public changes.
- Validate links, code examples, OpenAPI, and generated references in CI.

## Reliability and throughput

- Version documentation with product/API versions.
- Prefer tested examples over prose-only instructions.
- Avoid documenting unsupported or experimental features as stable.
- Include operational limits, quotas, timeouts, and capacity guidance.
- Document failure modes and degraded behaviour, not only happy paths.
- Keep docs builds deterministic and cacheable.

## Testing

- Build the documentation site in CI.
- Run Astro/content checks and markdown linting.
- Check internal and external links.
- Execute shell, curl, SDK, and CLI examples in isolated CI fixtures.
- Validate OpenAPI examples against route schemas.
- Test versioned migration and deployment instructions.
- Review every public release for stale screenshots, terminology, and claims.

## Deliverables

- Public product site with coherent narrative.
- Developer documentation hub.
- Operator runbooks.
- Executable quickstart and integration examples.
- Documentation quality gates.

## Exit criteria

- A developer can integrate without reading source code.
- An operator can deploy, back up, restore, monitor, and roll back.
- Documentation examples are tested and remain synchronized with the product.

