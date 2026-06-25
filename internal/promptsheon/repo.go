package promptsheon

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// Filesystem layout for a promptsheon repository. These constants
// are used everywhere; keep them lowercase ASCII so they survive
// cross-platform case-insensitive filesystems (macOS HFS+,
// Windows NTFS) unchanged.
const (
	// PromptsheonDir is the per-repository metadata directory,
	// analogous to .git. All other directories are nested inside it.
	PromptsheonDir = ".promptsheon"

	// objectsDir holds the content-addressable object store. Objects
	// are sharded by the first two hex characters of their hash to
	// keep directory fan-out bounded.
	objectsDir = "objects"

	// headsDir holds local branch references. Each file inside it
	// contains the 64-character hash of the commit the branch
	// points to.
	headsDir = "refs/heads"

	// headFile is the symbolic HEAD pointer. Its content is either
	// "ref: refs/heads/<name>" for a branch checkout or a raw
	// 64-character hash for a detached HEAD.
	headFile = "HEAD"

	// maxBranchLength bounds the size of a branch name. Git uses
	// 255; we follow suit.
	maxBranchLength = 255
)

// logger is the package-wide structured logger. It is configured
// at init() to write JSON records to stderr at the level requested
// by the PROMPTSHEON_LOG_LEVEL environment variable. Library
// consumers can override it via SetLogger.
var logger *slog.Logger

func init() {
	level := slog.LevelInfo
	switch os.Getenv("PROMPTSHEON_LOG_LEVEL") {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

// SetLogger replaces the package logger. Useful for tests and for
// host applications that want to route promptsheon's structured
// logs through their own logger.
func SetLogger(l *slog.Logger) {
	if l == nil {
		return
	}
	logger = l
}

// nowUnixNano returns the current wall-clock time in nanoseconds
// since the Unix epoch. Extracted into a helper so tests can
// override the clock via a package-private variable in the future.
func nowUnixNano() int64 {
	return time.Now().UnixNano()
}

// Init creates a fresh .promptsheon directory in the current
// working directory. It is idempotent: directories that already
// exist are left in place, and HEAD is rewritten to point at the
// default branch.
//
// Returns an error if .promptsheon already contains content but
// does not look like a repository, or if any of the directory
// creations fail.
func Init() error {
	if _, err := os.Stat(PromptsheonDir); err == nil {
		// Directory exists. Validate it.
		if !IsInitialized() {
			return fmt.Errorf("%s exists but is not a promptsheon repository", PromptsheonDir)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", PromptsheonDir, err)
	} else {
		// Create the layout.
		dirs := []string{
			filepath.Join(PromptsheonDir, objectsDir),
			filepath.Join(PromptsheonDir, headsDir),
		}
		for _, d := range dirs {
			if err := os.MkdirAll(d, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", d, err)
			}
		}
	}

	// Always rewrite HEAD so a half-initialized directory can be
	// rescued by re-running init. We also create an empty "main"
	// ref file so ListRefs and the CLI both report a branch even
	// before the first commit; the empty hash signals "no commits
	// yet" without breaking the invariant that HEAD always points
	// at an existing ref.
	if err := WriteHEAD("ref: refs/heads/main"); err != nil {
		return err
	}
	if err := WriteRef("main", ""); err != nil {
		return err
	}
	logger.Debug("repository initialised", "dir", PromptsheonDir)
	return nil
}

// IsInitialized reports whether the current working directory
// contains a usable .promptsheon repository. The check is the
// existence of both the .promptsheon/objects/ directory and the
// .promptsheon/HEAD file. Either alone is not enough: a stale
// objects/ without HEAD would be a corrupt partial init.
func IsInitialized() bool {
	if _, err := os.Stat(PromptsheonDir); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(PromptsheonDir, objectsDir)); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(PromptsheonDir, headFile)); err != nil {
		return false
	}
	return true
}

// copyFile copies src to dst with the given mode. It is used by
// the test helpers and by Init; isolating it keeps the test code
// short and makes the intent obvious.
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}
