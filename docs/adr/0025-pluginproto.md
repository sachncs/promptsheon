# ADR 0025: Plugin transport — net/rpc v0.1.x, gRPC M3.5

- **Status:** Accepted
- **Date:** 2026-07-10
- **Supersedes:** —
- **Superseded by:** —

## Context

The architecture review board (Tier 1.29 / 2.32) called for a
remote plugin transport that the supervisor uses to launch,
supervise, and health-check process plugins. The first cut of
this transport (commit `4ba80a5 feat: subprocess plugin path
(net/rpc over UDS, M3 follow-on)`) uses Go stdlib `net/rpc` over
a Unix domain socket. The contract is the `Plugin` service name
with three methods: `Ping`, `Health`, `Stop`.

The M3.5 follow-on per ADR-0019 introduces the gRPC transport.
The v0.1.x code path stays on net/rpc; the gRPC path ships
in M3.5 with codegen stubs and a generated pb.go client.

## Decision

The plugin wire contract is committed in
`internal/pluginproto/proto/plugin.proto`. The proto file is
the canonical source of truth:

```proto
syntax = "proto3";
package pluginv1.v0_1_0;

service Plugin {
  rpc Ping(PingRequest) returns (PingResponse);
  rpc Health(HealthRequest) returns (HealthResponse);
  rpc Stop(StopRequest) returns (StopResponse);
}

message PingRequest {}
message PingResponse { string name = 1; string version = 2; }
message HealthRequest {}
message HealthResponse { bool ok = 1; }
message StopRequest {}
message StopResponse {}
```

The v0.1.x transport is `net/rpc` over UDS (internal/subprocess).
A plugin binary that listens on a UDS path and implements the
three methods on a `*pluginRPC` receiver can be loaded by the
production supervisor. The `internal/pluginproto/doc.go` Go file
is a doc-only placeholder; the M3.5 commit adds the generated
pb.go stubs and the `grpc.Dial` call sites that replace
`rpc.Dial`.

The net/rpc and gRPC transports share the same three methods
(Ping, Health, Stop) so a v0.1.x plugin that implements one can
be ported to the other by registering the same three methods on
a `pluginv1.PluginServer`.

## Options considered

1. **gRPC from day one.** Rejected for v0.1.x: the .proto
   codegen adds a real toolchain dependency (protoc, the Go
   gRPC plugins, generated .pb.go). The v0.1.x baseline keeps
   the toolchain simple with `net/rpc` and a 0-dependency
   transport. gRPC is the M3.5 follow-on.
2. **Custom binary protocol.** Rejected: `net/rpc` is the Go
   stdlib's battle-tested equivalent. Custom protocols are
   maintenance overhead without functional benefit at v0.1.x's
   scale.
3. **net/rpc v0.1.x, gRPC M3.5 (chosen).** Today's v0.1.x
   ships net/rpc (commit `4ba80a5`); the M3.5 commit imports
   the generated `pluginv1` package and replaces `rpc.Dial`
   with `grpc.Dial` at the supervisor's subprocess call sites.
   The .proto file is the canonical contract.

## Consequences

Positive:

- The wire format is stable. Plugins written against the
  v0.1.x net/rpc path can be ported to gRPC by registering
  the same three methods on a generated `pluginv1.PluginServer`.
- The .proto file is committed; production tenants
  generate stubs in their build pipeline (M3.5).
- The net/rpc transport remains the production path for
  v0.1.x. The gRPC adapter is a follow-on.

Negative:

- Plugin binaries written against the v0.1.x net/rpc path
  must be ported to gRPC in M3.5. The wire format is
  forward-only (no deprecation period).
- The gRPC codegen requires protoc + protoc-gen-go. Plugin
  authors will need to add these to their build pipelines.

## References

- `internal/subprocess/subprocess.go` — net/rpc implementation
- `internal/pluginproto/proto/plugin.proto` — gRPC contract
- `internal/pluginproto/doc.go` — package doc
- ADR-0016 (Plugins over gRPC) — the architectural decision for
  the M3.5 path
- ADR-0024 (Plugin transport — net/rpc v0.1.x, gRPC M3.5) —
  earlier decision on the v0.1.x transport
- Architecture Review §21 Tier 1.29
