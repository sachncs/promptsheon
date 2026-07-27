package capability

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// ComputeManifestHash returns the canonical SHA-256 hex of a
// Manifest in its JSON serialisation. The hash is the Version's
// primary identity: two Versions with the same Manifest share the
// same hash. The handler package also computes this for HTTP
// request validation; both call sites must agree on the encoding,
// which is the stdlib json.Marshal of the value (no field reordering,
// no whitespace stripping). The two implementations MUST stay in
// sync — if you change one, change both.
func ComputeManifestHash(m Manifest) (string, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
