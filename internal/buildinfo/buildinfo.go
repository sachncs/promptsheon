// Package buildinfo exposes the version, commit, and build time
// of the running binary. The values are set at link time by the
// GoReleaser ldflags:
//
//	-X github.com/sachncs/promptsheon/internal/buildinfo.Version=...
//	-X github.com/sachncs/promptsheon/internal/buildinfo.Commit=...
//	-X github.com/sachncs/promptsheon/internal/buildinfo.BuildTime=...
//
// When the binary is built from a plain 'go build' (no ldflags)
// the variables fall back to "dev" / "unknown" / "unknown". This
// lets `go test ./...` and local development work without
// setting any flags.
package buildinfo

import "runtime"

// These are set via -ldflags="-X ..." at release time. Treat
// them as immutable after process start.
var (
	// Version is the semantic version of the running binary.
	// Examples: "0.0.6", "1.0.0-rc1".
	Version = "dev"

	// Commit is the git commit hash the binary was built from.
	Commit = "unknown"

	// BuildTime is the RFC3339 timestamp of the build.
	BuildTime = "unknown"
)

// Info is the structured form returned by the /api/v1/version
// endpoint and the --version flag.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// Get returns the current build info. The Go runtime fields are
// captured at call time so they always reflect the actual
// interpreter/architecture the binary is running on.
func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildTime: BuildTime,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}
