package postgres

import (
	"context"
	"testing"
)

func TestOpenRejectsEmptyDSN(t *testing.T) {
	t.Parallel()
	if _, err := Open(context.Background(), ""); err == nil {
		t.Fatalf("expected error for empty DSN")
	}
}

func TestCloseNilSafe(t *testing.T) {
	t.Parallel()
	var b *Backend
	if err := b.Close(); err != nil {
		t.Fatalf("Close on nil: %v", err)
	}
}
