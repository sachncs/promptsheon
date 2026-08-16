package cas

import (
	"os"
	"testing"
)

// BenchmarkWriteObject exercises the hot path of the CAS: write
// + dedup + read for a typical blob. Tracks gzip + IO + JSON
// encode allocations.
func BenchmarkWriteObject(b *testing.B) {
	dir := b.TempDir()
	oldWD := chdir(b, dir)
	defer chdir(b, oldWD)

	if err := Init(); err != nil {
		b.Fatalf("Init: %v", err)
	}
	obj := NewBlobObject(repeat("benchmark content ", 256))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := WriteObject(obj); err != nil {
			b.Fatalf("WriteObject: %v", err)
		}
	}
}

// BenchmarkWriteObjectDedup exercises the dedup hot path: every
// call hits the same hash so the store should detect the existing
// object without re-writing it.
func BenchmarkWriteObjectDedup(b *testing.B) {
	dir := b.TempDir()
	oldWD := chdir(b, dir)
	defer chdir(b, oldWD)

	if err := Init(); err != nil {
		b.Fatalf("Init: %v", err)
	}
	obj := NewBlobObject("stable content")

	if _, err := WriteObject(obj); err != nil {
		b.Fatalf("seed WriteObject: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := WriteObject(obj); err != nil {
			b.Fatalf("WriteObject: %v", err)
		}
	}
}

// BenchmarkCommit exercises a full commit + branch write. This is
// the most representative production write pattern: the seed
// blobs are written once, and each iteration commits a fresh
// tree that points at one of them.
func BenchmarkCommit(b *testing.B) {
	dir := b.TempDir()
	oldWD := chdir(b, dir)
	defer chdir(b, oldWD)

	if err := Init(); err != nil {
		b.Fatalf("Init: %v", err)
	}
	// Seed 50 blobs to give the commit a realistic tree to point at.
	for i := 0; i < 50; i++ {
		obj := NewBlobObject(repeat("seed-payload-", i+1))
		if _, err := WriteObject(obj); err != nil {
			b.Fatalf("seed WriteObject: %v", err)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Build a single-blob tree by reusing a seed blob's hash.
		seed := NewBlobObject(repeat("seed-payload-", i%50+1))
		seedHash, err := ObjectHash(seed)
		if err != nil {
			b.Fatalf("ObjectHash: %v", err)
		}
		tree := NewTreeObject([]TreeEntry{
			{Name: "file.txt", Hash: seedHash},
		})
		if _, err := WriteObject(tree); err != nil {
			b.Fatalf("write tree: %v", err)
		}
		treeHash, err := ObjectHash(tree)
		if err != nil {
			b.Fatalf("tree ObjectHash: %v", err)
		}
		if _, err = Commit(treeHash, nil, "tester", "bench commit", nil); err != nil {
			b.Fatalf("Commit: %v", err)
		}
	}
}

// BenchmarkVerify exercises the integrity verifier on a small
// repo. Verify is on the daemon's startup hot path.
func BenchmarkVerify(b *testing.B) {
	dir := b.TempDir()
	oldWD := chdir(b, dir)
	defer chdir(b, oldWD)

	if err := Init(); err != nil {
		b.Fatalf("Init: %v", err)
	}
	// Seed with a handful of commits so Verify has work.
	for i := 0; i < 10; i++ {
		blob := NewBlobObject(repeat("seed ", i+1))
		blobHash, err := ObjectHash(blob)
		if err != nil {
			b.Fatalf("ObjectHash: %v", err)
		}
		tree := NewTreeObject([]TreeEntry{{Name: "file.txt", Hash: blobHash}})
		if _, err := WriteObject(tree); err != nil {
			b.Fatalf("write tree: %v", err)
		}
		treeHash, err := ObjectHash(tree)
		if err != nil {
			b.Fatalf("tree ObjectHash: %v", err)
		}
		if _, err = Commit(treeHash, nil, "tester", "seed", nil); err != nil {
			b.Fatalf("Commit: %v", err)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Verify(); err != nil {
			b.Fatalf("Verify: %v", err)
		}
	}
}

// chdir flips the working directory for the duration of the
// test and returns the prior cwd for restoration. The public Init
// operates on the cwd; benchmarks need a clean repo per run so
// they don't leak state across runs.
func chdir(b *testing.B, dir string) string {
	b.Helper()
	old, err := os.Getwd()
	if err != nil {
		b.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		b.Fatalf("chdir: %v", err)
	}
	return old
}

// repeat concatenates s n times. ponytail: stdlib strings.Repeat
// is fine, but a custom one keeps the benchmark self-contained.
func repeat(s string, n int) string {
	if n <= 1 {
		return s
	}
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
