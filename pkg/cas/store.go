package cas

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
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
	// ErrInvalidRefName is returned when a reference name is malformed.
	ErrInvalidRefName = errors.New("invalid ref name")
	// ErrObjectTooLarge is returned when an object exceeds the size cap.
	ErrObjectTooLarge = errors.New("object exceeds maximum size")
)

// size limits applied to the on-disk and decompressed object streams.
// Caps prevent decompression-bomb and disk-fill denial-of-service.
const (
	// maxObjectOnDiskBytes is the largest file we will read from the objects dir.
	maxObjectOnDiskBytes = 64 << 20 // 64 MiB
	// maxObjectInflatedBytes is the largest uncompressed payload we will hold.
	maxObjectInflatedBytes = 256 << 20 // 256 MiB
)

// lockFile is the path (relative to PromptsheonDir) of the flock used to
// serialise mutation paths across processes. flock(2) is mandatory; without
// it, two daemons writing the same ref can race and corrupt state.
const lockFile = "lock"

// loggerPtr holds the package-wide structured logger as an atomic pointer so
// that SetLogger and concurrent reads from Commit/Init/etc. are race-free.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	loggerPtr.Store(defaultLogger())
}

// logger returns the current package logger. Reads are race-free via
// atomic.Pointer; writes go through SetLogger.
func logger() *slog.Logger { return loggerPtr.Load() }

// defaultLogger returns a JSON slog handler writing to stderr at the level
// requested by the PROMPTSHEON_LOG_LEVEL environment variable.
func defaultLogger() *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(os.Getenv("PROMPTSHEON_LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

// SetLogger replaces the package logger. Useful for tests and for host
// applications that want to route promptsheon's structured logs through their
// own logger. Pass nil to restore the default logger.
func SetLogger(l *slog.Logger) {
	if l == nil {
		loggerPtr.Store(defaultLogger())
		return
	}
	loggerPtr.Store(l)
}

// withRepoLock takes the repository-wide flock for the duration of fn. The
// lock is released even on panic. The lock file is created lazily under
// PromptsheonDir with mode 0600.
//
// All mutation paths (Commit, WriteRef, WriteHEAD, Checkout, CreateBranch,
// DeleteBranch) MUST run under this lock to keep concurrent writers from
// clobbering each other.
func withRepoLock(fn func() error) error {
	root, err := os.OpenRoot(PromptsheonDir)
	if err != nil {
		return fmt.Errorf("open repo: %w", err)
	}
	defer func() { _ = root.Close() }()

	f, err := root.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open lock: %w", err)
	}
	defer func() { _ = f.Close() }()

	if err := flockAcquire(f); err != nil {
		return fmt.Errorf("flock: %w", err)
	}
	defer func() { _ = flockRelease(f) }()

	return fn()
}

// ObjectHash returns the SHA-256 hash that identifies the content of obj.
// The hash is derived from the canonical JSON serialization and is guaranteed
// to be deterministic: identical objects always hash to the same value.
// L-11 fix: returns an error so embedded callers (daemon, tests)
// can fail loudly on serialization failures rather than relying
// on the panicking behaviour of the previous implementation.
func ObjectHash(obj *Object) (string, error) {
	return canonicalHash(obj)
}

// WriteObject serializes obj to canonical JSON, gzip-compresses it, and writes
// it to the content-addressable store under .promptsheon/objects/.
//
// If an object with the same hash already exists, WriteObject is a no-op and
// returns the existing hash. Written objects are set to read-only (0400)
// permissions to prevent accidental mutation.
//
// WriteObject is crash-safe: the bytes are written to a temp file in the same
// directory, fsynced, and atomically renamed over the final path. A crash
// before the rename leaves the prior (or no) object on disk, never a partial.
//
// WriteObject returns the SHA-256 hash of the stored object.
func WriteObject(obj *Object) (string, error) {
	hash, err := canonicalHash(obj)
	if err != nil {
		return "", err
	}

	relPath := filepath.Join(objectsDir, hash[:2], hash[2:])

	if e := os.MkdirAll(filepath.Join(PromptsheonDir, filepath.Dir(relPath)), 0750); e != nil {
		return "", fmt.Errorf("mkdir: %w", e)
	}

	root, err := os.OpenRoot(PromptsheonDir)
	if err != nil {
		return "", fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = root.Close() }()

	// Try to create the file atomically. O_EXCL ensures we fail if the file
	// already exists, eliminating the TOCTOU race between stat and write.
	f, err := root.OpenFile(relPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0400)
	if err != nil {
		if isExist(err) {
			logger().Debug("object deduplicated", "hash", hash[:12])
			return hash, nil
		}
		return "", fmt.Errorf("create: %w", err)
	}

	logger().Debug("writing object", "hash", hash[:12])

	jsonData, err := canonicalSerialize(obj)
	if err != nil {
		if cerr := f.Close(); cerr != nil {
			logger().Warn("close after serialize failure", "err", cerr)
		}
		if rerr := root.Remove(relPath); rerr != nil && !isNotExist(rerr) {
			logger().Warn("cleanup after serialize failure", "err", rerr)
		}
		return "", fmt.Errorf("serialize: %w", err)
	}

	var compressed bytes.Buffer
	gw := gzip.NewWriter(&compressed)
	if _, err := gw.Write(jsonData); err != nil {
		if cerr := f.Close(); cerr != nil {
			logger().Warn("close after gzip write failure", "err", cerr)
		}
		if rerr := root.Remove(relPath); rerr != nil && !isNotExist(rerr) {
			logger().Warn("cleanup after gzip failure", "err", rerr)
		}
		return "", fmt.Errorf("gzip write: %w", err)
	}
	if err := gw.Close(); err != nil {
		if cerr := f.Close(); cerr != nil {
			logger().Warn("close after gzip close failure", "err", cerr)
		}
		if rerr := root.Remove(relPath); rerr != nil && !isNotExist(rerr) {
			logger().Warn("cleanup after gzip close failure", "err", rerr)
		}
		return "", fmt.Errorf("gzip close: %w", err)
	}

	if _, err := f.Write(compressed.Bytes()); err != nil {
		if cerr := f.Close(); cerr != nil {
			logger().Warn("close after write failure", "err", cerr)
		}
		if rerr := root.Remove(relPath); rerr != nil && !isNotExist(rerr) {
			logger().Warn("cleanup after write failure", "err", rerr)
		}
		return "", fmt.Errorf("write: %w", err)
	}
	// fsync before close so the bytes survive a crash.
	if err := f.Sync(); err != nil {
		if cerr := f.Close(); cerr != nil {
			logger().Warn("close after fsync failure", "err", cerr)
		}
		if rerr := root.Remove(relPath); rerr != nil && !isNotExist(rerr) {
			logger().Warn("cleanup after fsync failure", "err", rerr)
		}
		return "", fmt.Errorf("fsync: %w", err)
	}
	if err := f.Close(); err != nil {
		if rerr := root.Remove(relPath); rerr != nil && !isNotExist(rerr) {
			logger().Warn("cleanup after close failure", "err", rerr)
		}
		return "", fmt.Errorf("close: %w", err)
	}

	logger().Debug("object written", "hash", hash[:12], "bytes", len(compressed.Bytes()))
	return hash, nil
}

// ReadObject reads, decompresses, and deserializes the object identified by
// hash from the content-addressable store. It verifies that the content's hash
// matches the requested hash and returns ErrObjectCorrupted if it does not.
//
// ReadObject enforces two size caps to defend against decompression bombs:
//   - the on-disk file must not exceed maxObjectOnDiskBytes, and
//   - the decompressed payload must not exceed maxObjectInflatedBytes.
//
// Exceeding either cap returns ErrObjectTooLarge wrapped with detail.
func ReadObject(hash string) (*Object, error) {
	if err := validateHash(hash); err != nil {
		return nil, err
	}

	relPath := filepath.Join(objectsDir, hash[:2], hash[2:])

	root, err := os.OpenRoot(PromptsheonDir)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = root.Close() }()

	f, err := root.Open(relPath)
	if err != nil {
		if isNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrObjectNotFound, hash)
		}
		return nil, fmt.Errorf("open: %w", err)
	}
	defer func() { _ = f.Close() }()

	// Cap the on-disk read so a corrupt object cannot OOM the process.
	compressed, err := io.ReadAll(io.LimitReader(f, maxObjectOnDiskBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read object: %w", err)
	}
	if int64(len(compressed)) > maxObjectOnDiskBytes {
		return nil, fmt.Errorf("%w: on-disk size > %d bytes", ErrObjectTooLarge, maxObjectOnDiskBytes)
	}

	gr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer func() { _ = gr.Close() }()

	jsonData, err := io.ReadAll(io.LimitReader(gr, maxObjectInflatedBytes+1))
	if err != nil {
		return nil, fmt.Errorf("decompress: %w", err)
	}
	if int64(len(jsonData)) > maxObjectInflatedBytes {
		return nil, fmt.Errorf("%w: inflated size > %d bytes", ErrObjectTooLarge, maxObjectInflatedBytes)
	}

	var obj Object
	if e := json.Unmarshal(jsonData, &obj); e != nil {
		return nil, fmt.Errorf("unmarshal: %w", e)
	}

	computed, err := canonicalHash(&obj)
	if err != nil {
		return nil, fmt.Errorf("verify hash: %w", err)
	}
	if computed != hash {
		logger().Error("object corruption detected", "expected", hash[:12], "computed", computed[:12])
		return nil, fmt.Errorf("%w: expected %s, computed %s", ErrObjectCorrupted, hash, computed)
	}

	logger().Debug("object read", "hash", hash[:12], "compressed_bytes", len(compressed))
	return &obj, nil
}

// ObjectExists reports whether an object with the given hash exists in the store.
func ObjectExists(hash string) (bool, error) {
	if err := validateHash(hash); err != nil {
		return false, err
	}
	relPath := filepath.Join(objectsDir, hash[:2], hash[2:])

	root, err := os.OpenRoot(PromptsheonDir)
	if err != nil {
		return false, fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = root.Close() }()

	_, err = root.Stat(relPath)
	if err != nil {
		if isNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat object: %w", err)
	}
	return true, nil
}

// ObjectFileSize returns the on-disk compressed size of the object identified
// by hash. Returns 0 and an error if the object does not exist.
func ObjectFileSize(hash string) (int64, error) {
	if err := validateHash(hash); err != nil {
		return 0, err
	}
	relPath := filepath.Join(objectsDir, hash[:2], hash[2:])

	root, err := os.OpenRoot(PromptsheonDir)
	if err != nil {
		return 0, fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = root.Close() }()

	info, err := root.Stat(relPath)
	if err != nil {
		if isNotExist(err) {
			return 0, fmt.Errorf("%w: %s", ErrObjectNotFound, hash)
		}
		return 0, fmt.Errorf("stat: %w", err)
	}
	return info.Size(), nil
}

// WriteRef writes the branch reference name to point at targetHash.
// It creates the .promptsheon/refs/heads directory if it does not exist.
//
// WriteRef is crash-safe: bytes go to a temp file in the same directory,
// are fsynced, and then atomically renamed over the final path. A crash
// before the rename leaves either the prior ref or no ref; never a partial.
//
// WriteRef must be called under withRepoLock when other writers may be
// active.
func WriteRef(name, targetHash string) error {
	if err := validateRefName(name); err != nil {
		return err
	}
	cleaned := sanitizeHash(targetHash)
	if cleaned != "" {
		if err := validateHash(cleaned); err != nil {
			return fmt.Errorf("ref target: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Join(PromptsheonDir, headsDir, filepath.Dir(name)), 0750); err != nil {
		return fmt.Errorf("mkdir refs: %w", err)
	}

	return writeFileAtomic(filepath.Join(headsDir, name), []byte(cleaned), 0o600)
}

// ReadRef reads the branch reference name and returns the commit hash it
// points to. Returns ErrRefNotFound if the reference does not exist.
func ReadRef(name string) (string, error) {
	if err := validateRefName(name); err != nil {
		return "", err
	}

	root, err := os.OpenRoot(PromptsheonDir)
	if err != nil {
		return "", fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = root.Close() }()

	f, err := root.Open(filepath.Join(headsDir, name))
	if err != nil {
		if isNotExist(err) {
			return "", fmt.Errorf("%w: %s", ErrRefNotFound, name)
		}
		return "", fmt.Errorf("read ref: %w", err)
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("read ref: %w", err)
	}
	return sanitizeHash(string(bytes.TrimSpace(data))), nil
}

// ListRefs returns the names of all branch references in the repository.
func ListRefs() ([]string, error) {
	refsDir := filepath.Join(PromptsheonDir, headsDir)
	entries, err := os.ReadDir(refsDir)
	if err != nil {
		if isNotExist(err) {
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
//
// WriteHEAD is crash-safe: bytes go to a temp file, are fsynced, and
// then atomically renamed. Must be called under withRepoLock when
// other writers may be active.
func WriteHEAD(content string) error {
	return writeFileAtomic(headFile, []byte(content), 0o600)
}

// ReadHEAD returns the raw content of the HEAD file (symbolic reference or
// commit hash). Returns ErrRepoNotInitialized if .promptsheon does not exist.
func ReadHEAD() (string, error) {
	root, err := os.OpenRoot(PromptsheonDir)
	if err != nil {
		if isNotExist(err) {
			return "", ErrRepoNotInitialized
		}
		return "", fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = root.Close() }()

	f, err := root.Open(headFile)
	if err != nil {
		if isNotExist(err) {
			return "", ErrRepoNotInitialized
		}
		return "", fmt.Errorf("read HEAD: %w", err)
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(f)
	if err != nil {
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
// the <name> portion. Returns "" if HEAD is detached or if the content
// does not match the expected shape — callers should treat "" as
// detached/invalid rather than as a usable branch name.
func HEADRefName(content string) string {
	if IsHEADDetached(content) {
		return ""
	}
	s := content[len("ref: "):]
	parts := strings.SplitN(s, "/", 3)
	if len(parts) == 3 && parts[0] == "refs" && parts[1] == "heads" && parts[2] != "" {
		return parts[2]
	}
	return ""
}

// validateRefName checks that name is a safe branch name without path
// traversal components. Returns ErrInvalidRefName (wrapped) if the name is
// empty or contains dangerous characters.
func validateRefName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: must not be empty", ErrInvalidRefName)
	}
	if strings.Contains(name, "\x00") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return fmt.Errorf("%w: %q", ErrInvalidRefName, name)
	}
	return nil
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
// a library that may be embedded in a long-running daemon.
func canonicalHash(obj *Object) (string, error) {
	data, err := canonicalSerialize(obj)
	if err != nil {
		return "", fmt.Errorf("canonical hash serialize: %w", err)
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

// writeFileAtomic writes data to relPath (relative to PromptsheonDir) using
// the standard "write to temp + fsync + rename" pattern. The temp file is
// created in the same directory so the rename is atomic on POSIX. Mode is
// applied to the final file. If the target exists, it is replaced.
func writeFileAtomic(relPath string, data []byte, mode os.FileMode) error {
	root, err := os.OpenRoot(PromptsheonDir)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = root.Close() }()

	if err := os.MkdirAll(filepath.Join(PromptsheonDir, filepath.Dir(relPath)), 0o750); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Join(PromptsheonDir, filepath.Dir(relPath)), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		if cerr := os.Remove(tmpName); cerr != nil && !isNotExist(cerr) {
			logger().Warn("cleanup temp", "path", tmpName, "err", cerr)
		}
	}

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("fsync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		cleanup()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := renameAcrossRoot(root, relPath, tmpName); err != nil {
		cleanup()
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// renameAcrossRoot performs an atomic rename from tmpAbs (an absolute path
// inside the repo) to relPath (relative to the OpenRoot). os.OpenRoot
// does not expose Rename, so we fall back to os.Rename when both paths
// are inside the same directory.
func renameAcrossRoot(_ *os.Root, relPath, tmpAbs string) error {
	// Sanity: ensure the temp file is actually inside the repo.
	absRepo, err := filepath.Abs(PromptsheonDir)
	if err != nil {
		return fmt.Errorf("abs repo: %w", err)
	}
	absTmp, err := filepath.Abs(tmpAbs)
	if err != nil {
		return fmt.Errorf("abs tmp: %w", err)
	}
	if !strings.HasPrefix(absTmp, absRepo+string(filepath.Separator)) {
		return fmt.Errorf("temp file %q is outside repo %q", absTmp, absRepo)
	}
	target := filepath.Join(PromptsheonDir, relPath)
	if err := os.Rename(tmpAbs, target); err != nil {
		return err
	}
	// fsync the directory so the rename is durable.
	if d, err := os.Open(filepath.Dir(target)); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

// isExist reports whether err corresponds to a file-exists error, on any OS.
func isExist(err error) bool {
	return errors.Is(err, fs.ErrExist)
}

// isNotExist reports whether err corresponds to a file-does-not-exist error.
func isNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}
