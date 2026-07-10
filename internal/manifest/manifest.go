// Package manifest implements the PROMPTSHEON_PLUGINS_FILE manifest
// parser. The manifest is a YAML document listing the plugin
// binaries the supervisor should launch at boot. Each entry carries
// the plugin's binary path, environment, advertised services, and
// (for gRPC over UDS) the UDS socket path. The supervisor reads the
// manifest at boot and spawns one process per entry.
//
// This is Tier 2.32 of the architecture review board. The
// subprocess-execution path (gRPC over UDS, health gate, restart
// budget) is the M3 follow-on; today's commit ships the manifest
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

// Validate enforces the closed-set Name, a non-empty Binary, and
// (if set) a UDS path that points under /tmp.
func (e Entry) Validate() error {
	if !namePattern.MatchString(e.Name) {
		return fmt.Errorf("%w: %q", ErrBadName, e.Name)
	}
	if e.Binary == "" {
		return fmt.Errorf("manifest: empty binary path for %q", e.Name)
	}
	return nil
}

var namePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// DefaultUDS returns the UDS path the supervisor would use when an
// entry does not specify one. The path is namespaced under
// /tmp/promptsheon/ to keep it namespaced.
func (e Entry) DefaultUDS() string {
	if e.UDS != "" {
		return e.UDS
	}
	return "/tmp/promptsheon/" + e.Name + ".sock"
}
