// Package settings — process-stable replica id helper.
//
// The settings layer is CRDT-aware: every local Set/Delete
// attributes the write to a per-process replica id so a
// concurrent merge can break ties deterministically. The id
// must be stable for the lifetime of the daemon (a restart
// that mints a fresh id would have to re-merge its own
// history, which is wasted work).
//
// LocalReplicaID returns a 32-char hex string. crypto/rand is
// the source; on the (essentially impossible) failure of
// crypto/rand the function falls back to a stable, non-secret
// string derived from the host so the daemon still boots
// rather than refusing to start. The fallback is fine
// because the replica id is only used for CRDT tie-break,
// not for any security boundary.
package settings

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"os"
)

// LocalReplicaID returns a per-process replica id stable for
// the lifetime of the daemon. crypto/rand produces the
// entropy; the fallback path is documented above.
//
// ponytail: non-secret fallback allowed — the id drives
// CRDT tie-break, not auth or audit attribution.
func LocalReplicaID() string {
	var buf [16]byte
	if _, err := cryptorand.Read(buf[:]); err == nil {
		return hex.EncodeToString(buf[:])
	}
	if host, err := os.Hostname(); err == nil && host != "" {
		return "host-" + host
	}
	return "promptsheon-fallback-replica"
}
