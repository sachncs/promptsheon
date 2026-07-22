// Package kmsbyok is the KMS-backed KeyProvider implementation.
// The Provider calls KMSClient.GenerateDataKey; tests supply a
// deterministic test double.
package kmsbyok

import (
	"crypto/sha256"
	"encoding/hex"
)

// sha256Hash is the real implementation used by deterministicTestKey.
// The name avoids shadowing the imported package identifier.
func sha256Hash(b []byte) []byte {
	sum := sha256.Sum256(b)
	out := make([]byte, 32)
	copy(out, sum[:])
	return out
}

// sha256Hex returns the hex-encoded sha256 of b. Used as the LRU
// key in Provider so a rotated wrapped blob maps to a different
// key (acceptance: rotated KMS keys are reflected on next read).
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
