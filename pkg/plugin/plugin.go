// Package plugin defines the gRPC plugin surface for Promptsheon.
//
// Plugins are standalone binaries that implement one or more of the
// service definitions exposed here as Go interfaces. The server
// discovers them through a configuration file
// (PROMPTSHEON_PLUGINS_FILE) and connects over loopback gRPC (UDS /
// localhost TCP) — never over the public network — to invoke them
// from inside the request path.
//
// Plugins replace built-ins without server recompilation. A Provider
// plugin adds a new model vendor; a Guardrail plugin adds a new
// safety check. The consumer packages (capability, policy,
// recommendation, optimizer) declare the interface they consume;
// this package is the gRPC contract that lets any language
// implement the interface. A Go plugin is shown in
// plugins/providers/openai; anything that can speak gRPC can
// publish a plugin of its own.
//
// The mechanism chosen is gRPC because plugins are run in their own
// process and gRPC over UDS is the cheapest way to get typed,
// versioned, code-gen-friendly contracts across process boundaries.
// A WASM path for untrusted third-party Guardrails is a M3 follow-on.
//
// Each plugin is launched once at server startup, supervised for
// crashes, and asked to report health on a heartbeat. The plugin
// lifecycle is owned by the server's plugin supervisor; consumers
// only see an interface.
package plugin

import (
	"context"
	"errors"
	"fmt"
)

// PluginVersion is the semantic version of the plugin contract the
// plugin implements. Bumps break the contract; the server enforces
// a min_core_version per plugin descriptor.
type PluginVersion string

// PluginDescriptor is the static metadata a plugin publishes at
// registration time. The server uses it to validate capabilities
// against expected consumers.
type PluginDescriptor struct {
	Name           string
	Version        PluginVersion
	Services       []string
	MinCoreVersion PluginVersion
}

// Handshake is the registration message a plugin sends on its
// first stream. The server replies with HandshakeAck carrying the
// enabled boolean and any error.
type Handshake struct {
	Descriptor PluginDescriptor
}

// Plugin is the lifecycle interface every plugin binary satisfies
// at its top level. Implementations are responsible for spawning
// their gRPC server (typically via a generated stub).
type Plugin interface {
	// Handshake returns the plugin's descriptor. The server
	// invokes this once before opening any streams and refuses
	// the plugin if Descriptor.Services don't match the
	// registered services.
	Handshake(ctx context.Context) (PluginDescriptor, error)

	// Shutdown is called by the server on graceful shutdown. It
	// must drain any in-flight calls within the supplied context.
	Shutdown(ctx context.Context) error
}

// Errors are sentinels consumers may errors.Is against.
var (
	ErrServiceNotDeclared = errors.New("plugin: service not declared in descriptor")
	ErrVersionTooOld      = errors.New("plugin: plugin version older than min_core_version")
)

// validateDescriptor checks a descriptor against the services the
// consumer expects. Used by the supervisor when binding a plugin to
// a consumer.
func validateDescriptor(d PluginDescriptor, expectedServices []string) error {
	if d.Name == "" {
		return fmt.Errorf("plugin: descriptor missing Name")
	}
	declared := map[string]struct{}{}
	for _, s := range d.Services {
		declared[s] = struct{}{}
	}
	for _, want := range expectedServices {
		if _, ok := declared[want]; !ok {
			return fmt.Errorf("%w: %s", ErrServiceNotDeclared, want)
		}
	}
	return nil
}
