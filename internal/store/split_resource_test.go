package store

import "testing"

func TestSplitResource(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantKind string
		wantID   string
	}{
		{"workspace", "workspace:abc", "workspace", "abc"},
		{"release", "release:r1", "release", "r1"},
		{"user", "user:u1@example.com", "user", "u1@example.com"},
		{"empty", "", "", ""},
		{"no colon", "no_colon_here", "", ""},
		{"colons inside id", "exec:cmd:arg", "exec", "cmd:arg"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotKind, gotID := splitResource(c.input)
			if gotKind != c.wantKind || gotID != c.wantID {
				t.Errorf("splitResource(%q) = (%q, %q); want (%q, %q)",
					c.input, gotKind, gotID, c.wantKind, c.wantID)
			}
		})
	}
}
