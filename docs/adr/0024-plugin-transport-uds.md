# ADR 0024: Plugin transport — net/rpc over UDS for v0.1.x, gRPC for M3.5

- **Status:** Accepted
- **Date:** 2026-07-10
- **Supersedes:** —
- **Superseded by:** —

## Context

The architecture review board (Tier 2.32) called for a plugin
manifest that the daemon reads at boot to know which remote
plugins to launch. The prior commit landed
`internal/pluginsup` with a `Remote` stub that satisfied the
Plugin interface but did not actually exec anything.

The M3 follow-on plan per ADR-0019 includes "subprocess
supervisor with gRPC over UDS". gRPC codegen with .proto files
adds a real toolchain dependency (protoc, the Go gRPC plugins,
and a generated .pb.go file per release). The forward-only v0.1.0
release ships net/rpc over UDS instead, then upgrades to gRPC
in M3.5.

## Decision

`internal/subprocess.Binary` is the production plugin transport
for v0.1.x. The plugin binary listens on a UDS path declared in
the manifest; the supervisor exec's the binary, waits up to 5s
for the UDS endpoint, dials it, and proxies the Plugin wire
protocol (Plugin.Ping, Plugin.Health, Plugin.Stop) over
net/rpc. On crash, the supervisor restarts the binary subject to
RestartPolicy.

The net/rpc choice is the v0.1.x transport. It is the Go stdlib
binary protocol, has zero new dependencies, and is well-trodden
in the Go ecosystem. The v0.1.x ABI is:

```
service Plugin
  Ping    (PingArgs,    PingReply)    (string Name, string Version)
  Health  (HealthArgs,  HealthReply)  ()                    ()
  Stop    (StopArgs,    StopReply)    ()                    ()
```

The wire format is the standard Go net/rpc gob encoding. Plugin
binaries implement these methods on a `*PluginRPC` receiver and
register via `subprocess.ServeUnix(socket, name)`. The supervisor
is the only client.

## Options considered

1. **gRPC over UDS from day one.** Rejected as v0.1.x: the .proto
   codegen adds a real toolchain dependency and a generated
   .pb.go file that must be regenerated on every schema change.
   The gRPC + grpc-gateway + protoc-gen-go toolchain is a separate
   M3.5 milestone; the v0.1.x baseline keeps the toolchain
   simple.
2. **Custom binary protocol (custom length-prefixed framing
   over a net.Conn).** Rejected: net/rpc is the Go stdlib's
   battle-tested equivalent; writing a custom protocol is
   maintenance overhead with no functional benefit at v0.1.x's
   scale.
3. **net/rpc over UDS (chosen).** Zero new dependencies. The
   net/rpc package is the Go stdlib's RPC framework. UDS
   transport is the production pattern for same-host IPC; the
   kernel handles the connection lifecycle.

## Consequences

Positive:

- v0.1.x ships with a real subprocess plugin path; production
  tenants can author a manifest with `binary: /opt/foo` and
  the supervisor exec's it.
- The Plugin wire protocol is the net/rpc Plugin service name;
  plugins can use the standard net/rpc.Server.Register call.
- gRPC migration in M3.5 is a server-side change (replace
  net/rpc with grpc.NewServer) and a client-side change
  (replace rpc.Dial with grpc.Dial). The supervisor's lifecycle
  and the manifest format do not change.

Negative:

- Plugins written today against the v0.1.x net/rpc protocol
  must be ported to gRPC in M3.5. The ABI is different
  (gRPC uses protobuf; net/rpc uses gob).
- The wire format is Go-specific. A plugin written in another
  language would need a net/rpc-compatible codec. The gRPC
  M3.5 milestone fixes this.

## References

- internal/subprocess/subprocess.go — Binary lifecycle,
  ServeUnix, PluginRPC contract
- internal/pluginsup/supervisor.go — LoadFromEnv dispatches
  on binary field
- internal/subprocess/subprocess_test.go — table-driven suite
  for the wire protocol
- ADR-0015 (Postgres+RLS), 0016 (Plugins over gRPC) — the
  M3.5 follow-on wires gRPC codegen with .proto files; today's
  net/rpc is the v0.1.x transport
- Architecture Review §21 Tier 2.32
