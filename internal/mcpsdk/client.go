// Package mcpsdk bridges the per-Workspace mcplist allowlist with
// the official modelcontextprotocol/go-sdk client. The wire
// formats supported are:
//   - http://, https:// : HTTP streamable transport (client-side
//     support lands in a future go-sdk release; today the SDK
//     only ships the server side of streamable HTTP)
//   - unix://           : reserved for future UDS support
//   - command:/path     : spawn the plugin binary; not part of
//     mcplist URL syntax today; documented here so the runtime
//     can dispatch when the manifest entry points at a binary
//
// Today's path covers the common case: a command binary is
// launched and JSON-RPC over stdio is used. Production tenants
// running pure HTTP MCP servers should pin to a future go-sdk
// release with streamable HTTP client support.
package mcpsdk

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sachncs/promptsheon/internal/mcplist"
)

// Dial connects to a single MCP server entry and returns a
// ClientSession ready for tool calls. The session is owned by the
// caller; Close it when done.
func Dial(ctx context.Context, entry mcplist.Entry) (*mcp.ClientSession, error) {
	if err := entry.Validate(); err != nil {
		return nil, fmt.Errorf("mcpsdk: %w", err)
	}
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "promptsheon",
		Version: "v0.1.0",
	}, &mcp.ClientOptions{})

	transport, err := buildTransport(entry)
	if err != nil {
		return nil, err
	}

	dialCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	session, err := client.Connect(dialCtx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("mcpsdk: connect %q: %w", entry.Name, err)
	}
	return session, nil
}

// buildTransport constructs the right mcp.Transport for the
// entry's URL. http(s):// is rejected today because the upstream
// SDK only ships server-side streamable HTTP; unix:// and
// command: forms are supported.
func buildTransport(entry mcplist.Entry) (mcp.Transport, error) {
	switch {
	case strings.HasPrefix(entry.URL, "http://"), strings.HasPrefix(entry.URL, "https://"):
		return nil, errors.New("mcpsdk: streamable http client transport not yet shipped by upstream go-sdk; pin to command: or unix: URLs")
	case strings.HasPrefix(entry.URL, "unix://"):
		// Production tenants typically run an MCP server behind a
		// reverse proxy exposing http. The UDS path is reserved
		// for air-gapped deployments; future versions wire it
		// through a streamable listener that the client dials.
		return nil, errors.New("mcpsdk: unix:// MCP transport not yet wired; use http(s):// via a future go-sdk release or command: with a binary")
	case strings.HasPrefix(entry.URL, "command:"):
		path := strings.TrimPrefix(entry.URL, "command:")
		if path == "" {
			return nil, errors.New("mcpsdk: command: url missing path")
		}
		return &mcp.CommandTransport{Command: exec.Command(path)}, nil
	default:
		return nil, fmt.Errorf("mcpsdk: unsupported url scheme for %q", entry.URL)
	}
}

// ListTools dials the entry, returns the advertised tool list,
// and closes the session. Convenience wrapper for capability
// discovery.
func ListTools(ctx context.Context, entry mcplist.Entry) (*mcp.ListToolsResult, error) {
	session, err := Dial(ctx, entry)
	if err != nil {
		return nil, err
	}
	defer func() { _ = session.Close() }()
	return session.ListTools(ctx, nil)
}

// HTTPHealthCheck returns the HTTP status code from a GET to the
// entry's URL. Used by the alerting path to confirm a remote MCP
// server is reachable; the value is informational, not part of
// the wire protocol.
func HTTPHealthCheck(ctx context.Context, url string) (int, error) {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return 0, errors.New("mcpsdk: HTTPHealthCheck only valid for http(s) URLs")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode, nil
}
