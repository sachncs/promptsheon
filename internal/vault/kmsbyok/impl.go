// Package kmsbyok is the KMS-backed KeyProvider implementation.
// The Provider calls KMSClient.GenerateDataKey; tests supply a
// deterministic test double.
package kmsbyok

import (
	"crypto/sha256"
)

// sha256Hash is the real implementation used by deterministicTestKey.
// The name avoids shadowing the imported package identifier.
func sha256Hash(b []byte) []byte {
	sum := sha256.Sum256(b)
	out := make([]byte, 32)
	copy(out, sum[:])
	return out
}
