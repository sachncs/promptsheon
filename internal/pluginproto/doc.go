// Package pluginproto defines the wire contract for the M3.5
// plugin transport. Today's net/rpc implementation lives in
// internal/subprocess; the gRPC adapter replaces it via the
// codegen-driven stubs in pluginproto. The wire format is
// forward-only: a v0.1.x plugin that implements the
// internal/subprocess PluginRPC contract will need a thin gRPC
// adapter that wraps the same business logic.
//
// F-23 forward-only. The proto file is the canonical contract;
// production tenants generate stubs in their build pipelines
// (M3.5 follow-on). For v0.1.x the net/rpc path in
// internal/subprocess remains the production transport.

// Package-level doc: M3.5 gRPC follow-on per ADR-0019 wires the
// .proto file below to a generated .pb.go client. The M3.5 commit
// imports the generated package and replaces internal/subprocess's
// net/rpc call sites with grpc.Dial. The wire format is the
// canonical contract; v0.1.x plugin binaries can implement
// either the net/rpc PluginRPC or the gRPC pluginv1.PluginServer.
package pluginproto

// No exported Go code yet. The M3.5 commit will add generated
// stubs (file descriptor: protocolbuffers/protobuf, protoc-gen-go).
// The .proto file in this directory is the source of truth.
