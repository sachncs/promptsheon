# ADR 0022: Plugin manifest (PROMPTSHEON_PLUGINS_FILE)

- **Status:** Accepted
- **Date:** 2026-07-10
- **Supersedes:** —
- **Superseded by:** —

## Context

The architecture review board (Tier 2.32) called for a
declarative plugin manifest that production operators can use
to register remote plugins with the daemon at boot. Without
the manifest, the only path to add a plugin is recompile +
restart, which is unacceptable for a system that ships with
several built-ins and is expected to grow third-party plugins.

The supervisor (Tier 2.46) already manages the lifecycle. The
manifest is the configuration surface the supervisor reads at
boot to know which processes to launch.

## Decision

`internal/manifest` ships the YAML manifest format and parser:

- File is the top-level shape: `plugins: [Entry]`.
- Entry is one plugin descriptor: `name`, `version`, `binary`,
  `args`, `env`, `services`, `uds`, `min_core_version`.
- The Name field uses the same closed-set as the MCP allowlist
  (alnum, dash, dot, underscore; 1-64 chars). The reuse
  prevents operator confusion between two similar surfaces.
- The Binary field is required and non-empty. Validation rejects
  the empty case with a clear error.
- The UDS field defaults to `/tmp/promptsheon/<name>.sock` when
  the operator does not specify one. The path is namespaced
  under /tmp/promptsheon/ to keep the supervisor's socket
  directory contained.
- The Services field is the list of pkg/plugin services the
  plugin implements ("Provider", "Guardrail", etc.). The
  supervisor's Handshake checks that the list matches the
  declared services.
- min_core_version is the version envelope; the supervisor
  refuses to load a plugin whose min_core_version exceeds the
  daemon's build version (an M3 follow-on enforces this).

## Options considered

1. **Single big config file with all subsystems.** Rejected:
   violates single-responsibility; the manifest is the only
   shape that needs to be hot-reloadable.
2. **Per-plugin config file (one file per plugin).** Rejected:
   ordering and dependencies between plugins become implicit;
   the manifest is the explicit ordering surface.
3. **YAML manifest at PROMPTSHEON_PLUGINS_FILE (chosen).** The
   operator authors one file; the supervisor reads it; the
   manifest is the contract. Per-plugin configs are out of
   scope for M3 follow-on.

## Consequences

Positive:

- Production operators add a plugin by appending one entry
  to PROMPTSHEON_PLUGINS_FILE and restarting the daemon (or
  SIGHUP for a hot reload in M3 follow-on).
- The closed-set Name and the UDS path make the supervisor
  safe: the UDS namespace is namespaced, the Name format is
  the same as the MCP allowlist, and the Services list is
  validated against the registered consumers.

Negative:

- The M3 follow-on that wires subprocess execution is the
  bigger piece; today's commit is the manifest format only.
- The manifest is read once at boot. Hot-reload via SIGHUP
  ships in M3 follow-on.

## References

- `internal/manifest/manifest.go` — File, Entry, Load, Validate
- `internal/manifest/manifest_test.go` — table-driven coverage
- Architecture Review §21 Tier 2.32.
- ADR-0019: deferred items include the M3 subprocess
  supervisor that consumes this manifest.
