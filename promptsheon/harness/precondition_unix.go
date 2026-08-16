//go:build !windows

package harness

import (
	"os/exec"
	"syscall"
)

// configureProcessGroup puts the precondition command into its
// own process group on POSIX systems so the runner can signal the
// entire subtree at once. The implementation uses
// syscall.SysProcAttr.Setpgid; on Linux this materialises as the
// setpgid(2) syscall the kernel requires for the negative-PID
// kill in killProcessGroup below.
func configureProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// killProcessGroup signals the entire process group rooted at the
// precondition command. The negative-PID argument is the
// documented POSIX way to target every process in a process group
// without enumerating children. SIGKILL is intentional: the
// caller is on a deadline (context.WithTimeout) and SIGTERM is
// not a guaranteed exit.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
