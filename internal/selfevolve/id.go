package selfevolve

import (
	"crypto/rand"
	"encoding/hex"
)

// generateID returns a 16-char hex string with a prefix
// (e.g. "v-0123abcd..."). crypto/rand doesn't fail on
// Linux/macOS, so the call is not checked.
func generateID(prefix string) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return prefix + "-" + hex.EncodeToString(b[:])
}
