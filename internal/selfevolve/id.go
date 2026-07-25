package selfevolve

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// generateID returns a 16-char hex string with a prefix
// (e.g. "v-0123abcd..."). The random bytes are sourced
// from crypto/rand; on the rare read failure we fall
// back to a timestamp so the evolver never panics.
func generateID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s-ts-%d", prefix, fallbackTS())
	}
	return prefix + "-" + hex.EncodeToString(b[:])
}
