package release

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"
)

// generateReleaseID produces a short, content-derived ID. Format is
// "rel-" followed by 16 hex characters: sha256(capabilityID || nanos).
// Capability-scoped lookups use ListReleasesForCapability, so the ID
// only needs to be unique enough to avoid collisions within a single
// capability at creation time.
func generateReleaseID(capabilityID string) string {
	h := sha256.New()
	h.Write([]byte(capabilityID))
	h.Write([]byte{0x1f})
	h.Write([]byte(strconv.FormatInt(time.Now().UnixNano(), 10)))
	return "rel-" + hex.EncodeToString(h.Sum(nil)[:8])
}
