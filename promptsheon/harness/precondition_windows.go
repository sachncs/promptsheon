//go:build windows

package harness

import (
	"os/exec"
)

// configureProcessGroup is a no-op on Windows. Windows has no
// POSIX-style process groups; the closest equivalent is a Job
// Object, which we do not need here because exec.Cmd already
// creates the child in its own process handle and Go's
// runtime cancels the process when the context fires.
func configureProcessGroup(cmd *exec.Cmd) {
	// Intentionally empty: see file-level comment.
	_ = cmd
}

// killProcessGroup terminates the precondition command. We use
// Process.Kill (which on Windows is implemented as
// TerminateProcess) instead of the POSIX negative-PID group
// signal because the latter does not exist on Windows. The
// caller is on a deadline; SIGKILL is the correct analogue.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
