package main

import (
	"context"
	"fmt"

	"github.com/sachncs/promptsheon/pkg/cas"
)

// loadCASPrompt reads a prompt blob by content hash from
// the process's local CAS at
// .promptsheon/objects/<hash[0:2]>/<hash[2:]>. The blob
// is gzipped JSON (per pkg/cas/store.go); we return the
// raw prompt text. Respects ctx.Err before disk I/O.
func loadCASPrompt(ctx context.Context, hash string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	obj, err := cas.ReadObject(hash)
	if err != nil {
		return nil, fmt.Errorf("evolver.cas: read %s: %w", hash, err)
	}
	return []byte(obj.Data), nil
}

// writeCASPrompt writes the prompt text to CAS and
// returns the content hash. Identical text always
// produces the same hash, so an unchanged revision is
// a no-op on the CAS side.
func writeCASPrompt(ctx context.Context, text string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	hash, err := cas.WriteObject(cas.NewBlobObject(text))
	if err != nil {
		return "", fmt.Errorf("evolver.cas: write: %w", err)
	}
	return hash, nil
}
