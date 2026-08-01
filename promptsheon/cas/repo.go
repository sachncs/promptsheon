package cas

import (
	"github.com/sachncs/promptsheon/errf"
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

// nowUnixNano returns the current wall-clock time in nanoseconds
// since the Unix epoch. Extracted into a helper so tests can
// override the clock via a package-private variable.
func nowUnixNano() int64 {
	return time.Now().UnixNano()
}

// Init creates a fresh .promptsheon directory in the current
// working directory. It is idempotent: directories that already
// exist are left in place, and HEAD/refs are only (re)written when
// the repository has just been created. Re-running Init against
// an existing repo with a HEAD pointing at a non-default branch
// will NOT clobber the user's branch.
//
// Returns an error if .promptsheon already contains content but
// does not look like a repository, or if any of the directory
// creations fail.
func Init() error {
	if _, err := os.Stat(PromptsheonDir); err == nil {
		// Directory exists. Validate it.
		if !IsInitialized() {
			return errf.Errorf("%s exists but is not a promptsheon repository", PromptsheonDir)
		}
		// Already initialised: leave HEAD and refs alone.
		logger().Debug("repository already initialised", "dir", PromptsheonDir)
		return nil
	} else if !isNotExist(err) {
		return errf.Errorf("stat %s: %w", PromptsheonDir, err)
	}

	// Create the layout.
	dirs := []string{
		filepath.Join(PromptsheonDir, objectsDir),
		filepath.Join(PromptsheonDir, headsDir),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o750); err != nil {
			return errf.Errorf("mkdir %s: %w", d, err)
		}
	}

	// First-time init: write HEAD pointing at the default branch and
	// create an empty "main" ref file so ListRefs and the CLI both
	// report a branch even before the first commit.
	if err := WriteHEAD("ref: refs/heads/main"); err != nil {
		return err
	}
	if err := WriteRef("main", ""); err != nil {
		return err
	}
	logger().Debug("repository initialised", "dir", PromptsheonDir)
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
