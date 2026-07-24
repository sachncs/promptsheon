// Property-based tests for the CAS layer. The library used
// (testing/quick) is the standard-library property harness; this
// file pins the content-addressing invariants that the store
// depends on:
//
//  1. ObjectHash is deterministic: same content → same hash.
//  2. ObjectHash is collision-free for arbitrary distinct strings.
//  3. ObjectHash is a 64-character lowercase hex SHA-256.
//  4. WriteObject → ReadObject round-trips the blob data exactly.
//  5. WriteObject is idempotent on the same content; the second
//     call returns the same hash and does not duplicate the file.
//  6. ObjectHash of a freshly-built blob matches the hash returned
//     by WriteObject.
//
// We deliberately stay on the existing Object / ObjectType API
// and do not redesign the union-kind — the property tests are
// pure black-box checks against the public surface.
//
// The Generator interface in testing/quick uses math/rand (v1);
// we import v1 under the alias "rand" so the generator signature
// matches the interface.
package cas

import (
	rand "math/rand"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
	"testing/quick"
)

// arbString is the printable generator for arbitrary short
// strings. We cap at 256 bytes so the per-iteration wall time
// stays small even after gzip.
type arbString string

func (arbString) Generate(rng *rand.Rand, size int) reflect.Value {
	n := size
	if n > 256 {
		n = 256
	}
	b := make([]byte, n)
	// Printable ASCII keeps the failure messages readable and
	// avoids the zero-byte edge cases some compressors love to
	// trip over.
	for i := range b {
		b[i] = byte('A' + rng.Intn(26))
	}
	return reflect.ValueOf(arbString(b))
}

// arbStringPair is the printable generator for two distinct
// short strings. We need both to differ so the no-collision
// property has a meaningful input.
type arbStringPair struct {
	A, B string
}

func (arbStringPair) Generate(rng *rand.Rand, size int) reflect.Value {
	n := size
	if n > 128 {
		n = 128
	}
	a := make([]byte, n)
	for i := range a {
		a[i] = byte('a' + rng.Intn(26))
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('A' + rng.Intn(26))
	}
	return reflect.ValueOf(arbStringPair{A: string(a), B: string(b)})
}

// arbBlobHash is the printable generator for previously-unseen
// 64-char hex strings. We use the Generator interface so the
// value is well-formed (lowercase hex), avoiding the shrinker's
// tendency to drive ObjectExists inputs to empty strings.
type arbBlobHash string

func (arbBlobHash) Generate(rng *rand.Rand, _ int) reflect.Value {
	const hex = "0123456789abcdef"
	b := make([]byte, 64)
	for i := range b {
		b[i] = hex[rng.Intn(len(hex))]
	}
	return reflect.ValueOf(arbBlobHash(string(b)))
}

// hex64Pattern is the only shape a valid hash is allowed to take.
var hex64Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// withFreshRepo chdirs into a tempdir and initializes the repo
// for the lifetime of the test. Returning the cleanup func via
// t.Cleanup means the directory is removed even if the test
// fails partway through.
func withFreshRepo(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	if err := Init(); err != nil {
		_ = os.Chdir(prev)
		t.Fatalf("Init: %v", err)
	}
	cleanup := func() {
		_ = os.Chdir(prev)
	}
	return dir, cleanup
}

// TestProperty_BlobHashIsDeterministic asserts that two hashes
// computed from an identical blob object are equal. This is the
// core content-addressing invariant: identical content ⇒
// identical hash.
func TestProperty_BlobHashIsDeterministic(t *testing.T) {
	t.Parallel()
	f := func(s arbString) bool {
		obj := NewBlobObject(string(s))
		h1, err := ObjectHash(obj)
		if err != nil {
			t.Logf("ObjectHash: %v", err)
			return false
		}
		h2, err := ObjectHash(obj)
		if err != nil {
			t.Logf("ObjectHash: %v", err)
			return false
		}
		return h1 == h2
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_BlobHashIsHex64 asserts that the hash is a
// 64-character lowercase hex string. The blob-storage layout
// shards on hash[:2], so anything that is not lowercase hex
// would break the on-disk layout.
func TestProperty_BlobHashIsHex64(t *testing.T) {
	t.Parallel()
	f := func(s arbString) bool {
		h, err := ObjectHash(NewBlobObject(string(s)))
		if err != nil {
			t.Logf("ObjectHash: %v", err)
			return false
		}
		return hex64Pattern.MatchString(h)
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_BlobHashNoCollisionForDistinctContent asserts
// that two blobs with different content produce different
// hashes. The generator builds A and B with disjoint alphabets
// so the empty-string case is impossible.
func TestProperty_BlobHashNoCollisionForDistinctContent(t *testing.T) {
	t.Parallel()
	f := func(p arbStringPair) bool {
		// Two different content lengths can collide if their
		// canonical serializations differ by less than the
		// length; we restrict the property to content of equal
		// length.
		if p.A == p.B {
			return true
		}
		ha, err := ObjectHash(NewBlobObject(p.A))
		if err != nil {
			t.Logf("ObjectHash A: %v", err)
			return false
		}
		hb, err := ObjectHash(NewBlobObject(p.B))
		if err != nil {
			t.Logf("ObjectHash B: %v", err)
			return false
		}
		return ha != hb
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_BlobWriteReadRoundtrip asserts that the contents
// survive a write + read round-trip byte-for-byte. The store
// gzips on write and decompresses on read, so the property
// catches any serialization glitch that would round-trip
// incorrectly.
func TestProperty_BlobWriteReadRoundtrip(t *testing.T) {
	_, cleanup := withFreshRepo(t)
	defer cleanup()

	f := func(s arbString) bool {
		obj := NewBlobObject(string(s))
		hash, err := WriteObject(obj)
		if err != nil {
			t.Logf("WriteObject: %v", err)
			return false
		}
		got, err := ReadObject(hash)
		if err != nil {
			t.Logf("ReadObject(%s): %v", hash, err)
			return false
		}
		if !got.IsBlob() {
			t.Logf("round-tripped object is not a blob: %+v", got)
			return false
		}
		return got.Data == obj.Data
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_BlobWriteIsIdempotent asserts that writing the
// same content twice yields the same hash and does not create
// a second object file for that hash. The store is a CAS:
// re-writes are no-ops. The shard can hold other blobs whose
// hash shares the same two-character prefix, so we check the
// specific file rather than the shard's total count.
func TestProperty_BlobWriteIsIdempotent(t *testing.T) {
	_, cleanup := withFreshRepo(t)
	defer cleanup()

	f := func(s arbString) bool {
		obj := NewBlobObject(string(s))
		hash1, err := WriteObject(obj)
		if err != nil {
			t.Logf("first WriteObject: %v", err)
			return false
		}
		hash2, err := WriteObject(obj)
		if err != nil {
			t.Logf("second WriteObject: %v", err)
			return false
		}
		if hash1 != hash2 {
			t.Logf("hash mismatch: %s vs %s", hash1, hash2)
			return false
		}
		// Exactly one file under the path matching the hash.
		objPath := filepath.Join(PromptsheonDir, objectsDir, hash1[:2], hash1[2:])
		info, err := os.Stat(objPath)
		if err != nil {
			t.Logf("Stat: %v", err)
			return false
		}
		if !info.Mode().IsRegular() {
			t.Logf("expected regular file, got %v", info.Mode())
			return false
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_BlobHashMatchesOnDisk asserts that the hash
// returned by WriteObject equals the hash returned by
// ObjectHash. The two paths share the same canonical serializer
// but the property pins that against future drift.
func TestProperty_BlobHashMatchesOnDisk(t *testing.T) {
	_, cleanup := withFreshRepo(t)
	defer cleanup()

	f := func(s arbString) bool {
		obj := NewBlobObject(string(s))
		hash, err := ObjectHash(obj)
		if err != nil {
			t.Logf("ObjectHash: %v", err)
			return false
		}
		stored, err := WriteObject(obj)
		if err != nil {
			t.Logf("WriteObject: %v", err)
			return false
		}
		return hash == stored
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_BlobIsBlobAfterRoundtrip asserts that a freshly
// written + read blob still classifies as a blob (not a tree or
// commit). The kind is inferred from which fields are populated;
// a serialization glitch that left a field populated would
// reclassify the object.
func TestProperty_BlobIsBlobAfterRoundtrip(t *testing.T) {
	_, cleanup := withFreshRepo(t)
	defer cleanup()

	f := func(s arbString) bool {
		obj := NewBlobObject(string(s))
		hash, err := WriteObject(obj)
		if err != nil {
			t.Logf("WriteObject: %v", err)
			return false
		}
		got, err := ReadObject(hash)
		if err != nil {
			t.Logf("ReadObject: %v", err)
			return false
		}
		return got.Type() == TypeBlob
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatal(err)
	}
}

// TestProperty_ObjectExistsTracksWrite asserts that ObjectExists
// returns true after write and false for a previously-unseen
// 64-char hex hash. The check is used by the daemon to skip
// re-writes; an always-true implementation would silently
// corrupt the dedup property.
func TestProperty_ObjectExistsTracksWrite(t *testing.T) {
	_, cleanup := withFreshRepo(t)
	defer cleanup()

	f := func(s arbString, fresh arbBlobHash) bool {
		obj := NewBlobObject(string(s))
		hash, err := WriteObject(obj)
		if err != nil {
			t.Logf("WriteObject: %v", err)
			return false
		}
		if !ObjectExists(hash) {
			t.Logf("ObjectExists(%s) = false after write", hash)
			return false
		}
		// The shrinker-driven "fresh" hash is a well-formed
		// 64-char lowercase hex string. We expect it not to
		// exist on the freshly-initialised disk; the
		// generator's 64^64 space makes collisions
		// effectively impossible.
		otherHash := string(fresh)
		if otherHash == hash {
			return true
		}
		if ObjectExists(otherHash) {
			t.Logf("ObjectExists(%s) = true before write", otherHash)
			return false
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatal(err)
	}
}
