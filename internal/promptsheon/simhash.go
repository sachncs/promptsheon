package promptsheon

import (
	"hash/fnv"
	"math/bits"
)

// SimHash returns a 64-bit SimHash of content. SimHash is a
// locality-sensitive hash: similar inputs produce fingerprints
// with small Hamming distance. We use it in the API to find
// near-duplicate prompts without an embedding model.
//
// The implementation character-shingles the input (3-grams
// padded with NUL bytes) and hashes each shingle with FNV-64.
// For each bit position, the sum of +1 / -1 contributions across
// all shingles determines the final bit: positive → 1, negative
// → 0. The procedure is the standard SimHash algorithm described
// by Charikar (2002).
func SimHash(content string) uint64 {
	if content == "" {
		return 0
	}
	const shingle = 3
	padded := make([]byte, len(content)+shingle-1)
	copy(padded, content)
	// Trailing bytes are zero, so the last shingles are stable
	// across inputs of different lengths.

	var sums [64]int
	count := 0
	for i := 0; i+shingle <= len(padded); i++ {
		h := fnvHash64(padded[i : i+shingle])
		for bit := 0; bit < 64; bit++ {
			if h&(1<<bit) != 0 {
				sums[bit]++
			} else {
				sums[bit]--
			}
		}
		count++
	}
	if count == 0 {
		return 0
	}
	var fp uint64
	for bit := 0; bit < 64; bit++ {
		if sums[bit] > 0 {
			fp |= 1 << bit
		}
	}
	return fp
}

// SimilarityScore returns a similarity score in [0, 1] for two
// SimHash fingerprints. The score is 1 minus the normalised
// Hamming distance; 1.0 means the fingerprints are identical and
// 0.0 means they differ in every bit.
func SimilarityScore(a, b uint64) float64 {
	xor := a ^ b
	// PopCount returns the number of set bits; that's the
	// Hamming distance.
	dist := bits.OnesCount64(xor)
	return 1.0 - float64(dist)/64.0
}

// fnvHash64 returns the FNV-64 hash of b. Using FNV-64 keeps the
// implementation pure Go (no SHA-256 setup cost per shingle).
func fnvHash64(b []byte) uint64 {
	h := fnv.New64a()
	_, _ = h.Write(b)
	return h.Sum64()
}
