# ADR 0016: Plugins over gRPC (loopback only)
#
# - **Status:** Accepted
# - **Date:** 2026-07-10
# - **Supersedes:** —
# - **Superseded by:** —
#
# ## Context
#
# The Charter principle "Plugin First" declares that Models,
# Guardrails, Evaluators, Optimizers, Storage, Telemetry, MCP,
# Tools, Knowledge, Memory, and Policies are all independently
# replaceable through stable interfaces. The current code embeds
# the built-in Providers (openai/anthropic/ollama/azure/nvidia) and
# the built-in Guardrails (static rules) inside `internal/llm` and
# `internal/guardrail`; replacing them requires recompiling the
# server.
#
# A plugin SDK makes the boundary first-class so third-party
# developers can ship a Provider, Guardrail, or Evaluator without
# touching this codebase.
#
# ## Decision
#
# Plugins are standalone binaries that speak gRPC with the server
# over loopback (UDS or localhost TCP). The contract lives in
# `pkg/plugin`. Mechanisms:
#
#   - Each plugin advertises its descriptor (name, version,
#     services served, min_core_version) on a Handshake RPC.
#   - The server validates the descriptor against the consumers
#     that load it (e.g. the LLM Registry expects a Provider
#     service).
#   - Crashes are detected via gRPC health; the supervisor
#     restarts a crashed plugin subject to a per-plugin budget.
#   - Secrets never leave the server: API keys are passed per
#     call as gRPC metadata, encrypted at rest in the server's
#     vault, never echoed in logs.
#   - gRPC over UDS is the cheapest path with the strongest
#     isolation; we never enable gRPC over the public network.
#
# Each consumer package continues to define its own interface
# (Charter Principle 3, "interfaces belong to consumers"). The
# plugin SDK provides adapters that satisfy those interfaces by
# talking gRPC to a plugin. Adding a new plugin does not require
# changes to consumer code.
#
# ## Options considered
#
# 1. **Go plugins via `plugin.Open`.** Rejected: the load order
#    matters, plugin symbols are version-sensitive, and the
#    plugin inherits the host's address space so a bug in a
#    plugin can crash the entire server. The cost (a contract)
#    is too high for the benefit (a single binary).
#
# 2. **WASM for everything.** Rejected for now: provider SDKs in
#    every language are HTTP/gRPC, and a WASM sandbox for
#    network-bound plugins is more complexity than value for the
#    first iteration. WASM is added in M3 follow-on for untrusted
#    third-party Guardrails only.
#
# 3. **gRPC over loopback with strict consumer-owned
#    interfaces.** Chosen. Strongest isolation; smallest impact
#    on consumer code; the contract can be rev'd per plugin
#    descriptor without bumping the server.
#
# ## Consequences
#
# Positive:
#
# - A third party can publish a Provider, Guardrail, or
#   Evaluator plugin in any language without forking the server.
# - Built-in providers and guardrails can be refactored into
#   plugins in M3 follow-on commits without changing consumer
#   packages.
# - The contract versioning rules are explicit: a plugin
#   advertises `min_core_version` and is rejected if the server
#   is older.
#
# Negative:
#
# - One binary per plugin to deploy and supervise. The
#   supervisor and the lifecycle in pkg/plugin are simple to
#   reason about but they are not free.
# - We add gRPC tooling to the build. The toolchain in CI must
#   install protoc only when a plugin is shipped; out of scope
#   for core contributors who do not author plugins.
#
# ## References
#
# - pkg/plugin/ (this ADR's directory)
# - internal/llm/registry.go (Provider consumer uses plugin
#   adapters)
# - Charter Principle 10 ("Plugin First") and Principle 3
#   ("Interfaces Belong to Consumers")
