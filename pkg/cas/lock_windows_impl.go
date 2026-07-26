//go:build windows

package cas

import (
	"os"
	"syscall"
	"time"
)

// lockFileWindows and unlockFileWindows wrap LockFileEx/UnlockFileEx so the
// caller can take and release an exclusive lock on the lock file.
//
// LockFileEx requires an OVERLAPPED struct even for non-overlapped I/O; we
// pass a zero-valued one, which is the documented way to lock a region.

const lockRetryInterval = 10 * time.Millisecond

func lockFileWindows(f *os.File, flags uint32) error {
	h := syscall.Handle(f.Fd())
	var overlapped syscall.Overlapped
	const maxRetries = 100
	for i := range maxRetries {
		err := syscall.LockFileEx(h, flags, 0, 1, 0, &overlapped)
		if err == nil {
			return nil
		}
		if err == syscall.ERROR_LOCK_VIOLATION {
			if i < maxRetries-1 {
				time.Sleep(lockRetryInterval)
				continue
			}
			return err
		}
		return err
	}
	return syscall.ERROR_LOCK_VIOLATION
}

func unlockFileWindows(f *os.File) error {
	h := syscall.Handle(f.Fd())
	var overlapped syscall.Overlapped
	return syscall.UnlockFileEx(h, 0, 1, 0, &overlapped)
}
