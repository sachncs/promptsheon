// Package backend — handler helpers that don't belong to any
// single handler file. These were split out when handlers_*.go
// files were getting too large. PLAN-49 c2.17 expanded
// capability/hash.go with the public InputHash/ModelRevision
// and dropped the lowercase twins and the errs.ErrProviderMissing
// alias. The file is now empty of public symbols except for
// ManifestHash which moved to capability/.
package backend

import (
	"github.com/sachncs/promptsheon/backend/capability"
)

// ManifestHash wraps capability.ComputeManifestHash so callers
// outside the backend package can use the same canonical hash
// without importing capability internals.
func ManifestHash(m capability.Manifest) (string, error) {
	return capability.ComputeManifestHash(m)
}