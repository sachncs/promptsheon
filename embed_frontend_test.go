package main

import (
	"errors"
	"io/fs"
	"testing"
)

func TestFrontendEmbedSub(t *testing.T) {
	sub, err := fs.Sub(frontendDist, "frontend/dist")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	f, err := fs.Stat(sub, "index.html")
	if err != nil {
		t.Fatalf("index.html missing under frontend/dist: %v", err)
	}
	if f.IsDir() {
		t.Fatalf("index.html is a directory")
	}
}

func TestFrontendEmbedSubNoLeakage(t *testing.T) {
	sub, err := fs.Sub(frontendDist, "frontend/dist")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	_, err = fs.Stat(sub, "frontend/dist/index.html")
	if err == nil {
		t.Fatal("expected 'frontend/dist/...' to be unreachable after fs.Sub, got nil")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected fs.ErrNotExist, got %v", err)
	}
}