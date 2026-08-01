package capability

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
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

// InputHash returns the SHA-256 hex of the JSON-encoded inputs
// passed to an LLM invoke. Empty input returns "" so callers can
// distinguish "no inputs" from "input that happened to hash to
// empty bytes". Moved here from promptsheon/handlers_helpers.go in
// PLAN-49 c2.17 so callers outside the backend package can use it.
func InputHash(input []byte) string {
	if len(input) == 0 {
		return ""
	}
	h := sha256.Sum256(input)
	return hex.EncodeToString(h[:])
}

// ModelRevision returns the per-day revision stamp used by the
// invoke path. The date is UTC; an empty model/provider still
// produces a stable, distinguishable string. Moved here from
// promptsheon/handlers_helpers.go in PLAN-49 c2.17.
func ModelRevision(model, provider string) string {
	return time.Now().UTC().Format("2006-01-02") + ":" + model + ":" + provider
}

// ManifestHashPlaceholder is the placeholder used when a Version
// row has no stored ManifestHash. Kept here as the single source
// of truth; handler helpers used to expose this but the lowercase
// twins have been removed in PLAN-49 c2.17.
func ManifestHashPlaceholder(versionID, model, provider string) string {
	h := sha256.New()
	h.Write([]byte(versionID))
	h.Write([]byte{0x1f})
	h.Write([]byte(model))
	h.Write([]byte{0x1f})
	h.Write([]byte(provider))
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
