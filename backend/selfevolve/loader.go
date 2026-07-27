package selfevolve

import (
	"context"
	"fmt"

	"github.com/sachncs/promptsheon/backend/cas"
)

// CasPromptLoader reads and writes prompt blobs in the local
// CAS at .promptsheon/objects/<hash[0:2]>/<hash[2:]>. The
// evolver uses it to fetch the active prompt and to write
// the candidate prompt. The CAS format is gzipped JSON
// (per pkg/cas/store.go) so callers do not need to know the
// on-disk layout.
type CasPromptLoader struct{}

// NewCasPromptLoader returns a loader. The CAS is process-global;
// the loader is stateless.
func NewCasPromptLoader() *CasPromptLoader { return &CasPromptLoader{} }

// LoadPrompt reads a prompt blob by content hash. The bytes
// returned are the raw prompt text (the CAS blob's data
// field, post-decompression and post-JSON-parse).
func (l *CasPromptLoader) LoadPrompt(ctx context.Context, hash string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	obj, err := cas.ReadObject(hash)
	if err != nil {
		return nil, fmt.Errorf("selfevolve.cas: read %s: %w", hash, err)
	}
	return []byte(obj.Data), nil
}

// WritePrompt writes the prompt text to CAS as a blob and
// returns the content hash. The hash is the canonical
// SHA-256 of the blob's JSON encoding — identical text always
// produces the same hash, so an unchanged revision is a
// no-op on the CAS side.
func (l *CasPromptLoader) WritePrompt(ctx context.Context, text string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	hash, err := cas.WriteObject(cas.NewBlobObject(text))
	if err != nil {
		return "", fmt.Errorf("selfevolve.cas: write: %w", err)
	}
	return hash, nil
}
