//go:build windows

package cas

import (
	"os"
	"syscall"
	"unsafe"
)

// lockFileWindows and unlockFileWindows wrap LockFileEx/UnlockFileEx so the
// caller can take and release an exclusive lock on the lock file.
//
// LockFileEx requires an OVERLAPPED struct even for non-overlapped I/O; we
// pass a zero-valued one, which is the documented way to lock a region.
var _ unsafe.Pointer

func lockFileWindows(f *os.File, flags uint32) error {
	h := syscall.Handle(f.Fd())
	var overlapped syscall.Overlapped
	for {
		err := syscall.LockFileEx(h, flags, 0, 1, 0, &overlapped)
		if err == nil {
			return nil
		}
		if err == syscall.ERROR_LOCK_VIOLATION {
			// Would block; retry briefly. LockFileEx is not interruptible
			// by signals the way flock is, but the surrounding loop with a
			// tight retry is enough for our short-lived contention.
			continue
		}
		return err
	}
}

func unlockFileWindows(f *os.File) error {
	h := syscall.Handle(f.Fd())
	var overlapped syscall.Overlapped
	return syscall.UnlockFileEx(h, 0, 1, 0, &overlapped)
}
