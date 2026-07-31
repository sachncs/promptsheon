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
	return ComputeManifestHash(m)
}

// ComputeManifestHash wraps capability.ComputeManifestHash so
// non-backend packages (notably package main's release invoker)
// can use the same canonical manifest hash without importing
// the capability package's internals.
func ComputeManifestHash(m capability.Manifest) (string, error) {
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

// InputHash returns the SHA-256 hex of the JSON-encoded inputs.
// Empty input returns "" so callers can distinguish "no inputs"
// from "input that happened to hash to empty bytes".
func InputHash(input []byte) string {
	if len(input) == 0 {
		return ""
	}
	h := sha256.Sum256(input)
	return hex.EncodeToString(h[:])
}

// inputHash returns the SHA-256 hex of the JSON-encoded inputs.
// Deprecated: use InputHash; kept for in-package callers.
func inputHash(input []byte) string {
	return InputHash(input)
}

// ModelRevision returns the per-day revision stamp used by
// the invoke path. The date is UTC; an empty model/provider
// still produces a stable, distinguishable string.
func ModelRevision(model, provider string) string {
	return time.Now().UTC().Format("2006-01-02") + ":" + model + ":" + provider
}

func modelRevision(model, provider string) string {
	return ModelRevision(model, provider)
}
var errProviderMissing = errs.ErrorExecutorProviderMissing
