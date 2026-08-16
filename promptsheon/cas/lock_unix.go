//go:build unix

package cas

import (
	"os"
	"syscall"

	"github.com/sachncs/promptsheon/errf"
)

// flockAcquire takes an exclusive flock(2) on f. Blocks until the lock is
// acquired. The companion flockRelease releases the lock; both are no-ops
// when f is nil or closed.
func flockAcquire(f *os.File) error {
	if f == nil {
		return errf.Errorf("flock: nil file")
	}
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
		if err == nil {
			return nil
		}
		if err == syscall.EINTR {
			continue
		}
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			// LOCK_EX | LOCK_NB would return this; with plain LOCK_EX we
			// block, so this should be unreachable, but guard anyway.
			continue
		}
		return errf.Errorf("flock LOCK_EX: %w", err)
	}
}

// flockRelease releases the flock(2) previously taken on f. Safe to call
// multiple times.
func flockRelease(f *os.File) error {
	if f == nil {
		return nil
	}
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		if err == nil {
			return nil
		}
		if err == syscall.EINTR {
			continue
		}
		return errf.Errorf("flock LOCK_UN: %w", err)
	}
}
