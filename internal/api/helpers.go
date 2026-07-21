package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// generateID produces a collision-resistant identifier. The
// UnixNano timestamp alone would let an attacker predict the next
// ID; we mix it with 8 random bytes so successive calls produce
// uncorrelated values. The format is "api-<unixnano>-<hex>" so
// it remains human-grep-able in logs.
//
// BUG-7: the previous form (api-<UnixNano>) was time-only; two
// back-to-back callers in the same nanosecond would have
// collided (and any external attacker could enumerate resources
// by guessing the next nanosecond timestamp).
func generateID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand is documented to never fail on Linux/macOS
		// unless the kernel itself is broken. Fall back to the
		// nanosecond timestamp so callers always get a value.
		return fmt.Sprintf("api-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("api-%d-%s", time.Now().UnixNano(), hex.EncodeToString(b[:]))
}
