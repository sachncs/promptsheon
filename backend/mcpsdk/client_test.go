package mcpsdk_test

import (
	"context"
	"testing"

	"github.com/sachncs/promptsheon/internal/mcplist"
	"github.com/sachncs/promptsheon/internal/mcpsdk"
)

func TestDialRejectsBadEntry(t *testing.T) {
	t.Parallel()
	if _, err := mcpsdk.Dial(context.Background(), mcplist.Entry{Name: "bad name"}); err == nil {
		t.Error("expected error for invalid entry")
	}
}

func TestDialRejectsUnsupportedHTTPScheme(t *testing.T) {
	t.Parallel()
	e := mcplist.Entry{Name: "remote", URL: "https://example.com/mcp"}
	if _, err := mcpsdk.Dial(context.Background(), e); err == nil {
		t.Error("expected error for http(s) URL until upstream streamable client ships")
	}
}

func TestDialRejectsUnsupportedUnixScheme(t *testing.T) {
	t.Parallel()
	e := mcplist.Entry{Name: "local", URL: "unix:///var/run/mcp.sock"}
	if _, err := mcpsdk.Dial(context.Background(), e); err == nil {
		t.Error("expected error for unix:// URL (future UDS support)")
	}
}

func TestDialRejectsCommandMissingPath(t *testing.T) {
	t.Parallel()
	e := mcplist.Entry{Name: "cmd", URL: "command:"}
	if _, err := mcpsdk.Dial(context.Background(), e); err == nil {
		t.Error("expected error for command: without path")
	}
}

func TestDialRejectsMissingBinary(t *testing.T) {
	t.Parallel()
	e := mcplist.Entry{Name: "cmd", URL: "command:/nonexistent/binary/path"}
	_, err := mcpsdk.Dial(context.Background(), e)
	if err == nil {
		t.Fatal("expected dial failure for nonexistent binary")
	}
}

func TestHTTPHealthCheckRejectsNonHTTP(t *testing.T) {
	t.Parallel()
	if _, err := mcpsdk.HTTPHealthCheck(context.Background(), "command:/foo"); err == nil {
		t.Error("expected error for non-http URL")
	}
}
