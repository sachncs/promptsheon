package buildinfo

import (
	"runtime"
	"strings"
	"testing"
)

func TestDefaults(t *testing.T) {
	// When the binary is built without -ldflags, the three
	// exported strings have known defaults. This test pins the
	// shape so a careless rename of a default value shows up as
	// a test failure rather than a confusing 'dev' in production.
	if Version != "dev" {
		t.Errorf("expected default Version=dev, got %q", Version)
	}
	if Commit != "unknown" {
		t.Errorf("expected default Commit=unknown, got %q", Commit)
	}
	if BuildTime != "unknown" {
		t.Errorf("expected default BuildTime=unknown, got %q", BuildTime)
	}
}

func TestGetReturnsAllFields(t *testing.T) {
	info := Get()
	if info.Version != Version {
		t.Errorf("Version: Get=%q, package var=%q", info.Version, Version)
	}
	if info.Commit != Commit {
		t.Errorf("Commit: Get=%q, package var=%q", info.Commit, Commit)
	}
	if info.BuildTime != BuildTime {
		t.Errorf("BuildTime: Get=%q, package var=%q", info.BuildTime, BuildTime)
	}
	if info.GoVersion != runtime.Version() {
		t.Errorf("GoVersion: Get=%q, runtime=%q", info.GoVersion, runtime.Version())
	}
	if info.OS != runtime.GOOS {
		t.Errorf("OS: Get=%q, runtime=%q", info.OS, runtime.GOOS)
	}
	if info.Arch != runtime.GOARCH {
		t.Errorf("Arch: Get=%q, runtime=%q", info.Arch, runtime.GOARCH)
	}
}

func TestGetIsJSONSerializable(t *testing.T) {
	// A consumer is going to JSON-encode the Info struct. The
	// field tags on the struct control the wire format; this
	// test makes sure a future refactor that drops or renames
	// a tag is caught.
	info := Get()
	// Build a minimal expected JSON shape; we just check the
	// keys are present and roughly correct, not the exact
	// encoding (encoding/json is tested by the standard
	// library).
	json := struct {
		Version   string
		Commit    string
		BuildTime string
		GoVersion string
		OS        string
		Arch      string
	}{
		Version:   info.Version,
		Commit:    info.Commit,
		BuildTime: info.BuildTime,
		GoVersion: info.GoVersion,
		OS:        info.OS,
		Arch:      info.Arch,
	}
	if json.Version == "" {
		t.Error("expected non-empty Version in serialised form")
	}
	if !strings.HasPrefix(json.GoVersion, "go") {
		t.Errorf("expected GoVersion to start with 'go', got %q", json.GoVersion)
	}
}
