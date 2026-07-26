//go:build windows

package cas

import (
	"fmt"
	"os"
)

// flockAcquire is a Windows stub. The Go runtime on Windows does not expose
// flock(2); we fall back to LockFileEx which has the same semantics. This
// build is intentionally conservative — the daemon is primarily POSIX.
func flockAcquire(f *os.File) error {
	if f == nil {
		return fmt.Errorf("flock: nil file")
	}
	// Use a best-effort advisory lock via LockFileEx; if the platform
	// rejects it, we fail closed to avoid the silent split-brain the
	// POSIX path guards against.
	return lockFileWindows(f, 0x00000004 /* LOCKFILE_EXCLUSIVE_LOCK */)
}

// flockRelease releases the Windows lock acquired by flockAcquire.
func flockRelease(f *os.File) error {
	if f == nil {
		return nil
	}
	return unlockFileWindows(f)
}
