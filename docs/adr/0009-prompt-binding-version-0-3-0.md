# ADR 0009: Adopt prompt-binding schema version 0.3.0

- **Status:** Accepted
- **Date:** 2024-01-01
- **Supersedes:** —
- **Superseded by:** —

## Context

A prompt is not useful on its own — it must be bound to a model. The binding needs to record the provider, the model, the parameters, and a reference to the API key. Earlier versions of the schema (in the original draft) baked the model name into the prompt definition, with no way to override per environment.

## Decision

We adopt a `binding` block on prompts with the structure:

```json
{
  "binding": {
    "provider": "openai",
    "model": "gpt-4o",
    "parameters": {"temperature": 0.7, "max_tokens": 1024},
    "api_key_ref": "openai-production"
  }
}
```

The schema version is recorded in the `prompts` table and the request body. The current version is `0.3.0`. The server rejects requests with an unknown `schema_version` with `400 Bad Request`.

A `system_prompt` field is a separate block (`internal/store/migrations/012_prompt_system_prompt.sql`) so that a system prompt and a user template can evolve independently.

## Options considered

1. **Explicit `binding` block, versioned (chosen).** Composable, testable, supports per-environment overrides.
2. **Implicit binding via the request body.** Easier to write a quick curl, harder to maintain, no audit trail.
3. **Tag-based binding.** Tags are flat, do not enforce shape.

## Consequences

Positive:

- The same prompt definition can be bound to different models in different environments (dev uses `gpt-4o-mini`, prod uses `gpt-4o`).
- The `api_key_ref` field references a key in the vault, so the key never appears in the prompt definition.
- The schema version is part of the persisted object, so old prompts can be re-hydrated even after a breaking change.

Negative:

- The schema is one more thing to keep in sync between the server, the SDK, and the CLI. The OpenAPI generator in `scripts/genopenapi` is the source of truth.
- We have not yet needed to roll forward. When we do, the migration plan is "add a new ADR, accept both versions for one release, deprecate the old one".

## References

- `internal/store/migrations/008_prompt_binding.sql` — initial binding columns
- `internal/store/migrations/012_prompt_system_prompt.sql` — system prompt split-out
- `internal/store/migrations/014_prompt_generation_config.sql` — generation parameters
- `docs/llm-providers.md` — provider and binding examples
