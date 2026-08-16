package main

import (
	"io/fs"
	"testing"
)

// TestFrontendEmbed verifies that the //go:embed directive in
// frontend.go picks up the dashboard assets. The previous
// regression was a missing //go:embed directive that left
// frontendDist as a zero-value embed.FS, causing fs.Sub to return
// an empty FS and every static path to 404.
func TestFrontendEmbed(t *testing.T) {
	sub, err := fs.Sub(frontendDist, "frontend/dist")
	if err != nil {
		t.Fatalf("fs.Sub frontend/dist: %v", err)
	}
	if sub == nil {
		t.Fatal("expected non-nil sub FS")
	}
	entries, err := fs.ReadDir(sub, ".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected embedded dashboard to contain entries")
	}
	hasIndex := false
	for _, e := range entries {
		if e.Name() == "index.html" {
			hasIndex = true
			break
		}
	}
	if !hasIndex {
		t.Fatalf("expected index.html in embedded dashboard, got %v", entries)
	}
}
