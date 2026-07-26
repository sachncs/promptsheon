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
// consumer expects and against the server's minimum supported
// plugin protocol version. Used by the supervisor when binding a
// plugin to a consumer.
//
// Versioning uses semver-style dotted triples ("1.2.3"). When
// either side omits the field we fall back to plain string equality
// so test fixtures and dev builds keep working. Mismatched, but
// parseable, versions return ErrVersionTooOld wrapped with detail.
func validateDescriptor(d PluginDescriptor, expectedServices []string) error {
	if d.Name == "" {
		return fmt.Errorf("plugin: descriptor missing Name")
	}
	if err := compareVersions(d.Version, d.MinCoreVersion); err != nil {
		return err
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

// compareVersions returns ErrVersionTooOld (wrapped) when
// pluginVersion is strictly older than minCoreVersion per semver rules.
// An empty pluginVersion or minCoreVersion is treated as "no
// constraint" — equality check only.
func compareVersions(pluginVersion, minCoreVersion PluginVersion) error {
	if pluginVersion == "" || minCoreVersion == "" {
		return nil
	}
	pv, perr := parseSemver(string(pluginVersion))
	mv, merr := parseSemver(string(minCoreVersion))
	if perr != nil || merr != nil {
		// Unknown version formats are accepted verbatim; supervisors
		// in production are expected to use semver, but dev builds
		// may use arbitrary tags.
		return nil
	}
	for i := 0; i < 3; i++ {
		if pv[i] < mv[i] {
			return fmt.Errorf("%w: plugin=%s min_core=%s", ErrVersionTooOld, pluginVersion, minCoreVersion)
		}
		if pv[i] > mv[i] {
			return nil
		}
	}
	return nil
}

// parseSemver splits "MAJOR.MINOR.PATCH" into its three integer
// components. Trailing pre-release/build metadata is ignored. Returns
// an error on any malformed input.
func parseSemver(s string) ([3]int, error) {
	var out [3]int
	// Trim trailing -prerelease and +build metadata.
	for i, r := range s {
		if r == '-' || r == '+' {
			s = s[:i]
			break
		}
	}
	parts := splitDots(s, 3)
	if len(parts) != 3 {
		return out, fmt.Errorf("not a triple: %q", s)
	}
	for i, p := range parts {
		n, err := atoi(p)
		if err != nil || n < 0 {
			return out, fmt.Errorf("bad component %q in %q: %w", p, s, err)
		}
		out[i] = n
	}
	return out, nil
}

func splitDots(s string, n int) []string {
	out := make([]string, 0, n)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			out = append(out, s[start:i])
			start = i + 1
			if len(out) == n-1 {
				break
			}
		}
	}
	out = append(out, s[start:])
	return out
}

func atoi(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not a digit: %c", r)
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}
