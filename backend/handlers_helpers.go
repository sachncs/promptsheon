package backend

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/sachncs/promptsheon/backend/errs"
	"github.com/sachncs/promptsheon/backend/capability"
)

// Auto-split from handlers_capabilities.go

func computeManifestHash(m capability.Manifest) (string, error) {
	return capability.ComputeManifestHash(m)
}
func manifestHash(versionID, model, provider string) string {
	h := sha256.New()
	h.Write([]byte(versionID))
	h.Write([]byte{0x1f})
	h.Write([]byte(model))
	h.Write([]byte{0x1f})
	h.Write([]byte(provider))
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// inputHash returns the SHA-256 hex of the JSON-encoded inputs.
func inputHash(input []byte) string {
	if len(input) == 0 {
		return ""
	}
	h := sha256.Sum256(input)
	return "sha256:" + hex.EncodeToString(h[:])
}

func modelRevision(model, provider string) string {
	return time.Now().UTC().Format("2006-01-02") + ":" + model + ":" + provider
}
var errProviderMissing = errs.ErrorExecutorProviderMissing
