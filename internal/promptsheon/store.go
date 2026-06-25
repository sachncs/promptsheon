package promptsheon

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Sentinel errors returned by the store package.
var (
	// ErrObjectNotFound is returned when an object hash does not exist in the store.
	ErrObjectNotFound = errors.New("object not found in CAS store")
	// ErrObjectCorrupted is returned when the content of a stored object does not
	// match its hash, indicating data corruption.
	ErrObjectCorrupted = errors.New("object hash does not match content")
	// ErrRepoNotInitialized is returned when an operation requires a repository
	// but .promptsheon does not exist.
	ErrRepoNotInitialized = errors.New("not a promptsheon repository (no .promptsheon)")
	// ErrInvalidHash is returned when a hash string is malformed (wrong length
	// or non-hex characters).
	ErrInvalidHash = errors.New("invalid object hash")
	// ErrRefNotFound is returned when no reference file exists for the given name.
	ErrRefNotFound = errors.New("ref not found")
)

// ObjectHash returns the SHA-256 hash that identifies the content of obj.
// The hash is derived from the canonical JSON serialization and is guaranteed
// to be deterministic: identical objects always hash to the same value.
// L-11 fix: returns an error so embedded callers (daemon, tests)
// can fail loudly on serialization failures rather than relying
// on the panicking behaviour of the previous implementation.
func ObjectHash(obj *Object) (string, error) {
	return canonicalHash(obj)
}

// canonicalSerialize returns the canonical JSON encoding of obj.
// The encoding uses Go's json.Marshal with struct fields in declaration order,
// which combined with sorted slices in the caller produces deterministic output.
func canonicalSerialize(obj *Object) ([]byte, error) {
	return json.Marshal(obj)
}

// canonicalHash computes the SHA-256 hash of the canonical JSON encoding of obj.
// L-11 fix: returns an error instead of panicking on marshal
// failure. The previous implementation panicked, which is
// appropriate for a CLI that owns its process but inappropriate for
// a library that may be embedded in a long-running daemon. Callers
// that genuinely cannot recover can wrap the result with a panic
// at the call site.
func canonicalHash(obj *Object) (string, error) {
	data, err := canonicalSerialize(obj)
	if err != nil {
		return "", fmt.Errorf("canonical hash serialize: %w", err)
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

// WriteObject serializes obj to canonical JSON, gzip-compresses it, and writes
// it to the content-addressable store under .promptsheon/objects/.
//
// If an object with the same hash already exists, WriteObject is a no-op and
// returns the existing hash. Written objects are set to read-only (0444)
// permissions to prevent accidental mutation.
//
// WriteObject returns the SHA-256 hash of the stored object.
func WriteObject(obj *Object) (string, error) {
	hash, err := canonicalHash(obj)
	if err != nil {
		return "", err
	}

	relPath := filepath.Join(objectsDir, hash[:2], hash[2:])
	fullPath := filepath.Join(PromptsheonDir, relPath)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	// Try to create the file atomically. O_EXCL ensures we fail if the file
	// already exists, eliminating the TOCTOU race between stat and write.
	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0444)
	if err != nil {
		if os.IsExist(err) {
			logger.Debug("object deduplicated", "hash", hash[:12])
			return hash, nil
		}
		return "", fmt.Errorf("create: %w", err)
	}

	logger.Debug("writing object", "hash", hash[:12])

	jsonData, err := canonicalSerialize(obj)
	if err != nil {
		f.Close()
		os.Remove(fullPath)
		return "", fmt.Errorf("serialize: %w", err)
	}

	var compressed bytes.Buffer
	gw := gzip.NewWriter(&compressed)
	if _, err := gw.Write(jsonData); err != nil {
		f.Close()
		os.Remove(fullPath)
		return "", fmt.Errorf("gzip write: %w", err)
	}
	if err := gw.Close(); err != nil {
		f.Close()
		os.Remove(fullPath)
		return "", fmt.Errorf("gzip close: %w", err)
	}

	if _, err := f.Write(compressed.Bytes()); err != nil {
		f.Close()
		os.Remove(fullPath)
		return "", fmt.Errorf("write: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(fullPath)
		return "", fmt.Errorf("close: %w", err)
	}

	logger.Debug("object written", "hash", hash[:12], "bytes", len(compressed.Bytes()))
	return hash, nil
}

// ReadObject reads, decompresses, and deserializes the object identified by
// hash from the content-addressable store. It verifies that the content's hash
// matches the requested hash and returns ErrObjectCorrupted if it does not.
func ReadObject(hash string) (*Object, error) {
	hash = sanitizeHash(hash)
	if len(hash) != 64 {
		return nil, fmt.Errorf("%w: hash must be 64 hex characters", ErrInvalidHash)
	}

	relPath := filepath.Join(objectsDir, hash[:2], hash[2:])
	fullPath := filepath.Join(PromptsheonDir, relPath)

	compressed, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrObjectNotFound, hash)
		}
		return nil, fmt.Errorf("read file: %w", err)
	}

	gr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer gr.Close()

	jsonData, err := io.ReadAll(gr)
	if err != nil {
		return nil, fmt.Errorf("decompress: %w", err)
	}

	var obj Object
	if err := json.Unmarshal(jsonData, &obj); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	computed, err := canonicalHash(&obj)
	if err != nil {
		return nil, fmt.Errorf("verify hash: %w", err)
	}
	if computed != hash {
		logger.Error("object corruption detected", "expected", hash[:12], "computed", computed[:12])
		return nil, fmt.Errorf("%w: expected %s, computed %s", ErrObjectCorrupted, hash, computed)
	}

	logger.Debug("object read", "hash", hash[:12], "compressed_bytes", len(compressed))
	return &obj, nil
}

// ObjectExists reports whether an object with the given hash exists in the store.
func ObjectExists(hash string) bool {
	hash = sanitizeHash(hash)
	if len(hash) != 64 {
		return false
	}
	relPath := filepath.Join(objectsDir, hash[:2], hash[2:])
	fullPath := filepath.Join(PromptsheonDir, relPath)
	_, err := os.Stat(fullPath)
	return err == nil
}

// ObjectFileSize returns the on-disk compressed size of the object identified
// by hash. Returns 0 and an error if the object does not exist.
func ObjectFileSize(hash string) (int64, error) {
	hash = sanitizeHash(hash)
	if len(hash) != 64 {
		return 0, fmt.Errorf("%w: hash must be 64 hex characters", ErrInvalidHash)
	}
	relPath := filepath.Join(objectsDir, hash[:2], hash[2:])
	fullPath := filepath.Join(PromptsheonDir, relPath)
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("%w: %s", ErrObjectNotFound, hash)
		}
		return 0, fmt.Errorf("stat: %w", err)
	}
	return info.Size(), nil
}

// WriteRef writes the branch reference name to point at targetHash.
// It creates the .promptsheon/refs/heads directory if it does not exist.
func WriteRef(name, targetHash string) error {
	refPath := filepath.Join(PromptsheonDir, headsDir, name)
	if err := os.MkdirAll(filepath.Dir(refPath), 0755); err != nil {
		return fmt.Errorf("mkdir refs: %w", err)
	}
	if err := os.WriteFile(refPath, []byte(targetHash), 0644); err != nil {
		return fmt.Errorf("write ref: %w", err)
	}
	return nil
}

// ReadRef reads the branch reference name and returns the commit hash it
// points to. Returns ErrRefNotFound if the reference does not exist.
func ReadRef(name string) (string, error) {
	refPath := filepath.Join(PromptsheonDir, headsDir, name)
	data, err := os.ReadFile(refPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s", ErrRefNotFound, name)
		}
		return "", fmt.Errorf("read ref: %w", err)
	}
	return sanitizeHash(string(bytes.TrimSpace(data))), nil
}

// ListRefs returns the names of all branch references in the repository.
func ListRefs() ([]string, error) {
	refsDir := filepath.Join(PromptsheonDir, headsDir)
	entries, err := os.ReadDir(refsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list refs: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// WriteHEAD sets the HEAD pointer. content should be either
// "ref: refs/heads/<name>" for a symbolic reference or a raw 64-char
// commit hash for a detached HEAD.
func WriteHEAD(content string) error {
	headPath := filepath.Join(PromptsheonDir, headFile)
	if err := os.WriteFile(headPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write HEAD: %w", err)
	}
	return nil
}

// ReadHEAD returns the raw content of the HEAD file (symbolic reference or
// commit hash). Returns ErrRepoNotInitialized if .promptsheon does not exist.
func ReadHEAD() (string, error) {
	headPath := filepath.Join(PromptsheonDir, headFile)
	data, err := os.ReadFile(headPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrRepoNotInitialized
		}
		return "", fmt.Errorf("read HEAD: %w", err)
	}
	return string(bytes.TrimSpace(data)), nil
}

// IsHEADDetached returns true if the HEAD content is a raw commit hash
// rather than a symbolic reference.
func IsHEADDetached(content string) bool {
	return !bytes.HasPrefix([]byte(content), []byte("ref: "))
}

// HEADRefName extracts the branch name from a symbolic HEAD reference.
// It expects content in the form "ref: refs/heads/<name>" and returns
// the <name> portion. Returns "" if HEAD is detached.
func HEADRefName(content string) string {
	if IsHEADDetached(content) {
		return ""
	}
	s := content[len("ref: "):]
	parts := strings.SplitN(s, "/", 3)
	if len(parts) == 3 && parts[0] == "refs" && parts[1] == "heads" {
		return parts[2]
	}
	return s
}
