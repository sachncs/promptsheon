// Package mcplist is the per-Workspace MCP (Model Context Protocol)
// allowlist. Each Workspace declares the set of trusted MCP servers
// its Releases may call; Releases whose Manifest references an
// MCP server outside the allowlist are rejected at validation
// time.
//
// The scope here is the value type, the closed-set validation, and the
// Repository interface. Runtime enforcement is in the invoke path;
// the MCP server SDK (gRPC over UDS plus the server manifest) ships in a
// follow-on.
package mcplist

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// Entry is one allowlisted MCP server.
//
// The Name is the canonical identifier (the manifest's
// "name" field). URL is the server's UDS or TCP endpoint;
// Validate rejects obvious injection (non-http schemes, missing
// host) so a malformed entry fails at allowlist time rather than
// at invoke time. WorkspaceID and CreatedBy are the audit-trail
// fields.
type Entry struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	WorkspaceID string `json:"workspace_id"`
	CreatedAt   string `json:"created_at"` // RFC3339
	CreatedBy   string `json:"created_by"`
}

// ErrEmptyName is returned by Validate when the Name is empty.
var ErrEmptyName = errors.New("mcplist: empty name")

// ErrBadName is returned when the Name contains characters
// outside the allowed set (alnum, dash, dot, underscore).
var ErrBadName = errors.New("mcplist: bad name")

// ErrBadURL is returned when the URL cannot be parsed as an
// absolute http(s) URL or unix UDS path.
var ErrBadURL = errors.New("mcplist: bad url")

// Validate enforces the closed-set Name format and a syntactically
// valid URL (http(s)://host[:port][/path] or unix:///abs/path).
func (e Entry) Validate() error {
	if strings.TrimSpace(e.Name) == "" {
		return ErrEmptyName
	}
	if !namePattern.MatchString(e.Name) {
		return fmt.Errorf("%w: %q", ErrBadName, e.Name)
	}
	if err := validateURL(e.URL); err != nil {
		return fmt.Errorf("%w: %q: %w", ErrBadURL, e.URL, err)
	}
	return nil
}

// validateURL accepts absolute http(s) URLs with a non-empty host
// and unix:///absolute/path UDS endpoints. Anything else is
// rejected; runtime SSRF checks happen at invoke time.
func validateURL(raw string) error {
	if raw == "" {
		return errors.New("empty url")
	}
	if strings.HasPrefix(raw, "unix://") {
		path := strings.TrimPrefix(raw, "unix://")
		if path == "" || path[0] != '/' {
			return errors.New("unix url must be absolute")
		}
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme %q not http(s)", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("missing host")
	}
	return nil
}

// namePattern is the closed set of characters allowed in an MCP
// server name: alnum, dash, dot, underscore. Length 1-64.
var namePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// List is the consumer-defined collection: a sorted, de-duplicated
// set of allowlisted MCP servers for one Workspace.
type List struct {
	WorkspaceID string  `json:"workspace_id"`
	Entries     []Entry `json:"entries"`
}

// NewList constructs an empty List for the supplied workspace.
func NewList(workspaceID string) *List {
	return &List{WorkspaceID: workspaceID}
}

// Add inserts an entry after Validate; duplicate Names overwrite
// the existing entry (allowing URL updates).
func (l *List) Add(e Entry) error {
	if err := e.Validate(); err != nil {
		return err
	}
	for i, existing := range l.Entries {
		if existing.Name == e.Name {
			l.Entries[i] = e
			l.sort()
			return nil
		}
	}
	l.Entries = append(l.Entries, e)
	l.sort()
	return nil
}

// Remove deletes the entry with the supplied Name. Returns
// ErrUnknownName if no entry matches.
func (l *List) Remove(name string) error {
	for i, e := range l.Entries {
		if e.Name == name {
			l.Entries = append(l.Entries[:i], l.Entries[i+1:]...)
			return nil
		}
	}
	return ErrUnknownName
}

// Allows reports whether the supplied name is on the list. The
// empty list allows nothing; the closed-set semantics: only
// explicitly listed servers may be called.
func (l *List) Allows(name string) bool {
	for _, e := range l.Entries {
		if e.Name == name {
			return true
		}
	}
	return false
}

// Validate walks every entry and returns the first failure.
func (l *List) Validate() error {
	for _, e := range l.Entries {
		if err := e.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (l *List) sort() {
	sort.Slice(l.Entries, func(i, j int) bool {
		return l.Entries[i].Name < l.Entries[j].Name
	})
}

// ErrUnknownName is returned by Remove when the supplied Name
// is not on the list.
var ErrUnknownName = errors.New("mcplist: unknown name")

// Repository is the consumer-defined persistence interface.
// Production wiring supplies a backend-backed implementation;
// tests use an in-memory map.
type Repository interface {
	Load(ctx context.Context, workspaceID string) (*List, error)
	Save(ctx context.Context, l *List) error
}
