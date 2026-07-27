// Package manifest implements the PROMPTSHEON_PLUGINS_FILE manifest
// parser. The manifest is a YAML document listing the plugin
// binaries the supervisor should launch at boot. Each entry carries
// the plugin's binary path, environment, advertised services, and
// (for gRPC over UDS) the UDS socket path. The supervisor reads the
// manifest at boot and spawns one process per entry.
//
// The subprocess-execution path (gRPC over UDS, health gate,
// restart budget) is the follow-on; this package ships the manifest
// parser and the Manifest / Entry value types.
package manifest

import (
	"errors"
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// File is the top-level shape of PROMPTSHEON_PLUGINS_FILE.
type File struct {
	Plugins []Entry `yaml:"plugins"`
}

// Entry is one plugin descriptor.
type Entry struct {
	Name           string   `yaml:"name"`
	Version        string   `yaml:"version"`
	Binary         string   `yaml:"binary"`
	Args           []string `yaml:"args"`
	Env            []string `yaml:"env"`      // KEY=VALUE form
	Services       []string `yaml:"services"` // e.g. ["Provider", "Guardrail"]
	UDS            string   `yaml:"uds"`      // optional, default /tmp/promptsheon/<name>.sock
	MinCoreVersion string   `yaml:"min_core_version"`
}

// ErrEmpty is returned when a manifest is empty.
var ErrEmpty = errors.New("manifest: no plugins")

// ErrBadName is returned when a plugin Name is empty or fails
// the same closed-set as the MCP allowlist.
var ErrBadName = errors.New("manifest: bad plugin name")

// Load reads the manifest from path and validates each entry.
func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("manifest: read: %w", err)
	}
	var f File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("manifest: parse: %w", err)
	}
	if len(f.Plugins) == 0 {
		return nil, ErrEmpty
	}
	for i := range f.Plugins {
		if err := f.Plugins[i].Validate(); err != nil {
			return nil, fmt.Errorf("manifest: plugins[%d]: %w", i, err)
		}
	}
	return &f, nil
}

// ErrBadUDS is returned when a manifest entry's UDS path does
// not live under /tmp/promptsheon/. The path namespace avoids
// collisions with system sockets and keeps plugin lifecycles
// scoped to a single tenant.
var ErrBadUDS = errors.New("manifest: UDS path must be under /tmp/promptsheon/")

// Validate enforces the closed-set Name, a non-empty Binary, and
// (if set) a UDS path that points under /tmp/promptsheon/.
func (e Entry) Validate() error {
	if !namePattern.MatchString(e.Name) {
		return fmt.Errorf("%w: %q", ErrBadName, e.Name)
	}
	if e.Binary == "" {
		return fmt.Errorf("manifest: empty binary path for %q", e.Name)
	}
	if e.UDS != "" && !udsPattern.MatchString(e.UDS) {
		return fmt.Errorf("%w: got %q", ErrBadUDS, e.UDS)
	}
	return nil
}

var namePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// udsPattern accepts any absolute path that resolves under
// /tmp/promptsheon/. Realpath normalization happens at the
// supervisor when it binds the listener; this validator only
// rejects paths that are clearly out of bounds.
var udsPattern = regexp.MustCompile(`^/tmp/promptsheon(/[A-Za-z0-9._-]+)*\.sock$`)

// DefaultUDS returns the UDS path the supervisor would use when an
// entry does not specify one. The path is namespaced under
// /tmp/promptsheon/ to keep it namespaced.
func (e Entry) DefaultUDS() string {
	if e.UDS != "" {
		return e.UDS
	}
	return "/tmp/promptsheon/" + e.Name + ".sock"
}
