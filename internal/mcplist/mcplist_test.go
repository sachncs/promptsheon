package mcplist

import (
	"errors"
	"testing"
)

func TestEntryValidateEmpty(t *testing.T) {
	t.Parallel()
	if err := (Entry{}).Validate(); !errors.Is(err, ErrEmptyName) {
		t.Fatalf("expected ErrEmptyName, got %v", err)
	}
}

func TestEntryValidateBadChars(t *testing.T) {
	t.Parallel()
	bad := []string{
		"server one",   // space
		"server/two",   // slash
		"server!three", // bang
		"server$four",  // dollar
		"server\nfive", // newline
		"",             // empty
	}
	for _, n := range bad {
		if err := (Entry{Name: n}).Validate(); err == nil {
			t.Errorf("expected error for %q", n)
		}
	}
}

func TestEntryValidateGood(t *testing.T) {
	t.Parallel()
	good := []string{
		"server-one",
		"server.two",
		"server_three",
		"ServerFour",
		"a",
		repeat64(),
	}
	for _, n := range good {
		e := Entry{Name: n, URL: "unix:///tmp/" + n + ".sock"}
		if err := e.Validate(); err != nil {
			t.Errorf("expected ok for %q, got %v", n, err)
		}
	}
}

func TestEntryValidateRejectsBadURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		url  string
	}{
		{"empty", ""},
		{"file scheme", "file:///etc/passwd"},
		{"no host", "http://"},
		{"relative unix", "unix://tmp/foo.sock"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := Entry{Name: "good", URL: tc.url}
			if err := e.Validate(); err == nil {
				t.Errorf("expected ErrBadURL for %q", tc.url)
			}
		})
	}
}

func TestEntryValidateAcceptsHTTPS(t *testing.T) {
	t.Parallel()
	for _, u := range []string{
		"http://localhost:7700",
		"https://mcp.example.com/v1",
		"unix:///var/run/mcp.sock",
	} {
		e := Entry{Name: "good", URL: u}
		if err := e.Validate(); err != nil {
			t.Errorf("URL %q should validate: %v", u, err)
		}
	}
}

func TestListAddRemoveAllows(t *testing.T) {
	t.Parallel()
	l := NewList("ws-1")
	if err := l.Add(Entry{Name: "server-a", URL: "unix:///tmp/a.sock", WorkspaceID: "ws-1"}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if !l.Allows("server-a") {
		t.Fatalf("expected Allows(server-a) true")
	}
	if l.Allows("server-b") {
		t.Fatalf("expected Allows(server-b) false")
	}
	if err := l.Remove("server-a"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if l.Allows("server-a") {
		t.Fatalf("expected Allows(server-a) false after remove")
	}
	if err := l.Remove("server-a"); !errors.Is(err, ErrUnknownName) {
		t.Fatalf("expected ErrUnknownName, got %v", err)
	}
}

func TestListAddDeduplicates(t *testing.T) {
	t.Parallel()
	l := NewList("ws-1")
	_ = l.Add(Entry{Name: "a", URL: "unix:///tmp/a.sock", WorkspaceID: "ws-1"})
	_ = l.Add(Entry{Name: "a", URL: "unix:///tmp/a2.sock", WorkspaceID: "ws-1"})
	if len(l.Entries) != 1 {
		t.Fatalf("expected 1 entry after duplicate add, got %d", len(l.Entries))
	}
	if l.Entries[0].URL != "unix:///tmp/a2.sock" {
		t.Fatalf("expected URL to be updated, got %s", l.Entries[0].URL)
	}
}

func TestListIsSorted(t *testing.T) {
	t.Parallel()
	l := NewList("ws-1")
	_ = l.Add(Entry{Name: "zeta", URL: "unix:///tmp/z.sock", WorkspaceID: "ws-1"})
	_ = l.Add(Entry{Name: "alpha", URL: "unix:///tmp/a.sock", WorkspaceID: "ws-1"})
	_ = l.Add(Entry{Name: "mu", URL: "unix:///tmp/m.sock", WorkspaceID: "ws-1"})
	if l.Entries[0].Name != "alpha" || l.Entries[1].Name != "mu" || l.Entries[2].Name != "zeta" {
		t.Fatalf("expected alphabetical sort, got %+v", l.Entries)
	}
}

func TestListValidateRejectsBadEntry(t *testing.T) {
	t.Parallel()
	l := NewList("ws-1")
	_ = l.Add(Entry{Name: "good", URL: "unix:///tmp/g.sock", WorkspaceID: "ws-1"})
	l.Entries = append(l.Entries, Entry{Name: "bad name", WorkspaceID: "ws-1"})
	if err := l.Validate(); err == nil {
		t.Fatalf("expected validation error on bad entry")
	}
}

// repeat64 returns a string of 64 'a' characters. Used to verify
// the 64-char upper bound on the Name field.
func repeat64() string {
	out := make([]byte, 64)
	for i := range out {
		out[i] = 'a'
	}
	return string(out)
}
