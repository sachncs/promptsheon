// Package settings — pure LWW register CRDT types.
//
// A settings row is replicated between daemon processes as a
// last-write-wins register keyed by config key. The merge
// operator:
//
//  1. Picks the record with the larger version vector on the
//     local replica (dominance).
//  2. On a concurrent tie (vectors are incomparable), breaks
//     the tie deterministically: higher WriteTS wins; if
//     timestamps are equal, the lexicographically smaller
//     ReplicaID wins.
//  3. Treats a tombstone as a regular record — a Set on a
//     tombstoned key re-asserts the row. The settings layer
//     hides tombstones from Get/List; a future resurrecting
//     write would have to dominate the tombstone's vector
//     and timestamp.
//
// The algebra — dominance + deterministic tie-break — is
// associative, commutative, and idempotent. The test file
// (crdt_test.go) pins every property.
package settings

import (
	"sort"
	"time"
)

// CRDTRecord is one replica's view of a settings row. The
// fields mirror models.SystemConfig but the merge function
// operates on this struct so callers don't need to import the
// model package.
type CRDTRecord struct {
	Key           string
	Value         string
	UpdatedBy     string
	UpdatedAt     time.Time
	WriteTS       int64
	ReplicaID     string
	VersionVector map[string]uint64
	Tombstone     bool
}

// Merge picks the dominant record under LWW semantics:
// dominance first, then WriteTS, then ReplicaID.
//
// The boolean return is true if `a` wins (caller usually
// discards b); false if `b` wins. Callers that need to keep
// both can ignore the return — the chosen record is
// `winner`.
//
// Documented ceiling: a 64-bit WriteTS overflows after ~584
// years of single-replica 1ns writes. Settings writes are
// infrequent (operator-driven), so this is comfortable.
// ponytail: tie-break is timestamp then replica id; document
// this in settings resolution docs because it affects
// observable ordering across replicas.
//
//nolint:gocritic // ponytail: value semantics keep merge inputs immutable.
func Merge(a, b CRDTRecord) CRDTRecord {
	return mergeWithWinner(a, b).winner
}

// MergeRecords is the multi-record fold over a slice. The
// fold order is irrelevant (associativity).
func MergeRecords(records ...CRDTRecord) CRDTRecord {
	if len(records) == 0 {
		return CRDTRecord{}
	}
	cur := records[0]
	for _, r := range records[1:] {
		cur = mergeWithWinner(cur, r).winner
	}
	return cur
}

// VectorDominated reports whether a dominates b under the
// vector-clock partial order: a's count for every replica is
// at least b's, and at least one is strictly greater. Equal
// vectors are not dominated (incomparable, hence tie-break).
func VectorDominated(a, b map[string]uint64) bool {
	if len(a) == 0 && len(b) == 0 {
		return false
	}
	strictly := false
	allKeys := unionKeys(a, b)
	for _, k := range allKeys {
		av, bv := a[k], b[k]
		if av < bv {
			return false
		}
		if av > bv {
			strictly = true
		}
	}
	return strictly
}

// mergeResult is the (winner, aWon) pair. Tests use aWon to
// assert the tie-break path independently of the returned
// record.
type mergeResult struct {
	winner CRDTRecord
	aWon   bool
}

// mergeWithWinner is the heart of the LWW merge: dominance +
// tie-break. The tie-break is documented as `ponytail:` only
// where the ceiling matters; here it does (concurrent writes
// across replicas must converge deterministically).
//
// Every branch folds the version vectors (component-wise
// max) into the winner so the merged record always carries
// the union of both sides' history. The fold is what makes
// the merge a total order on (folded vector, payload) —
// a downstream merge sees the same vector regardless of the
// order the operands were combined. Combined with the
// canonical payload tie-break below it makes Merge
// commutative, idempotent, and associative for arbitrary
// records.
//
//nolint:gocritic // ponytail: value semantics keep merge inputs immutable.
func mergeWithWinner(a, b CRDTRecord) mergeResult {
	if a.Key != "" && b.Key != "" && a.Key != b.Key {
		// Different keys: a Merge call is per-key, so this
		// is a programmer error. Return a to avoid data
		// loss; the settings layer only calls Merge inside
		// a per-key loop.
		return mergeResult{winner: a, aWon: true}
	}
	foldedVec := foldVectors(a.VersionVector, b.VersionVector)
	// Dominance: one vector strictly dominates the other.
	// The dominant record picks the payload, but the
	// version vector is always folded so causality is
	// preserved across merge sequences.
	if VectorDominated(a.VersionVector, b.VersionVector) {
		winner := a
		winner.VersionVector = foldedVec
		return mergeResult{winner: winner, aWon: true}
	}
	if VectorDominated(b.VersionVector, a.VersionVector) {
		winner := b
		winner.VersionVector = foldedVec
		return mergeResult{winner: winner, aWon: false}
	}
	// Tie-break: timestamp then replica id. Equal timestamps
	// and identical replica ids are the "same write replayed"
	// case — Merge is idempotent under those conditions.
	if a.WriteTS != b.WriteTS {
		if a.WriteTS > b.WriteTS {
			winner := a
			winner.VersionVector = foldedVec
			return mergeResult{winner: winner, aWon: true}
		}
		winner := b
		winner.VersionVector = foldedVec
		return mergeResult{winner: winner, aWon: false}
	}
	if a.ReplicaID != b.ReplicaID {
		if a.ReplicaID < b.ReplicaID {
			winner := a
			winner.VersionVector = foldedVec
			return mergeResult{winner: winner, aWon: true}
		}
		winner := b
		winner.VersionVector = foldedVec
		return mergeResult{winner: winner, aWon: false}
	}
	// Identical metadata on both sides — the payloads may
	// still differ. Apply the canonical payload-order
	// tie-break so the merge is commutative and idempotent
	// even when the records carry divergent payloads:
	//   1. Tombstone beats live record (prevents resurrection
	//      of a deleted key under a tombstoned metadata tie).
	//   2. Otherwise: lexicographic Value, UpdatedBy, then
	//      UpdatedAt nanoseconds — the order is fixed so the
	//      function is direction-independent.
	winner, aWon := canonicalPayloadWinner(a, b)
	winner.VersionVector = foldedVec
	return mergeResult{winner: winner, aWon: aWon}
}

// canonicalPayloadWinner picks the winner record under the
// canonical tie-break: tombstone wins over live record, then
// lexicographic Value, UpdatedBy, then UpdatedAt. Returns
// the winner record and a boolean indicating whether the
// winner came from the left operand (a).
//
//nolint:gocritic // ponytail: value semantics keep merge inputs immutable.
func canonicalPayloadWinner(a, b CRDTRecord) (CRDTRecord, bool) {
	if a.Tombstone != b.Tombstone {
		if a.Tombstone {
			return a, true
		}
		return b, false
	}
	if c := compareCanonical(a.Value, b.Value); c != 0 {
		if c < 0 {
			return a, true
		}
		return b, false
	}
	if c := compareCanonical(a.UpdatedBy, b.UpdatedBy); c != 0 {
		if c < 0 {
			return a, true
		}
		return b, false
	}
	if !a.UpdatedAt.Equal(b.UpdatedAt) {
		if a.UpdatedAt.Before(b.UpdatedAt) {
			return a, true
		}
		return b, false
	}
	// Fully equal payload: a is canonical.
	return a, true
}

// foldVectors returns the component-wise max of two version
// vectors. Used by every merge branch so the merged record
// carries the union of both sides' history; the dominance
// check stays a fast path for the "one side fully dominates"
// case but the folded vector is what survives to the next
// merge.
func foldVectors(a, b map[string]uint64) map[string]uint64 {
	if len(a) == 0 && len(b) == 0 {
		return map[string]uint64{}
	}
	out := make(map[string]uint64, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if cur, ok := out[k]; !ok || v > cur {
			out[k] = v
		}
	}
	return out
}

// compareCanonical is the canonical lexicographic compare
// used by the payload-order tie-break. It exists as a helper
// so the four-field ordering is a single, easy-to-grep
// sequence; production callers should not need it directly.
func compareCanonical(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// unionKeys returns the sorted union of two maps' keys. The
// sort is for deterministic test output and has no semantic
// effect on the merge.
func unionKeys(a, b map[string]uint64) []string {
	seen := map[string]struct{}{}
	for k := range a {
		seen[k] = struct{}{}
	}
	for k := range b {
		seen[k] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// IncVector returns a copy of v with the local replica's
// counter bumped by 1. The original map is not mutated.
func IncVector(v map[string]uint64, replicaID string) map[string]uint64 {
	out := make(map[string]uint64, len(v)+1)
	for k, n := range v {
		out[k] = n
	}
	out[replicaID]++
	return out
}

// NextWriteTS is the monotonic timestamp the local replica
// uses for the next Set/Delete. It is based on wall-clock
// time so two replicas writing at the same nanosecond tie-
// break on ReplicaID. Callers should use this instead of
// time.Now().UnixNano() so the contract is centralised.
func NextWriteTS() int64 {
	return time.Now().UnixNano()
}
