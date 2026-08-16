//go:build windows

// Package cas file-locking on Windows. The standard library
// removed the `syscall.LockFileEx` and friends in Go 1.20+; the
// documented replacement is `golang.org/x/sys/windows`. This
// file is the Windows implementation of the lock helpers in
// `lock.go`; see `lock_unix_impl.go` for the POSIX implementation.
package cas

import (
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// lockRetryInterval is the delay between successive LockFileEx
// attempts when the requested region is already held by another
// process. 10 ms keeps the worst-case wait near 1 s for the
// default retry budget (100 iterations).
const lockRetryInterval = 10 * time.Millisecond

// lockFileWindows takes an exclusive lock on the file handle.
// `flags` is the LockFileEx flag set: pass `windows.LOCKFILE_FAIL_IMMEDIATELY`
// to avoid blocking on contention, or `windows.LOCKFILE_EXCLUSIVE_LOCK`
// (the implicit default for `flags == 0`) for an exclusive lock.
func lockFileWindows(f *os.File, flags uint32) error {
	h := windows.Handle(f.Fd())
	var overlapped windows.Overlapped
	const maxRetries = 100
	for i := range maxRetries {
		err := windows.LockFileEx(h, flags, 0, 1, 0, &overlapped)
		if err == nil {
			return nil
		}
		if err == windows.ERROR_LOCK_VIOLATION {
			if i < maxRetries-1 {
				time.Sleep(lockRetryInterval)
				continue
			}
			return err
		}
		return err
	}
	return windows.ERROR_LOCK_VIOLATION
}

// unlockFileWindows releases an exclusive lock previously taken by
// lockFileWindows.
func unlockFileWindows(f *os.File) error {
	h := windows.Handle(f.Fd())
	var overlapped windows.Overlapped
	return windows.UnlockFileEx(h, 0, 1, 0, &overlapped)
}
