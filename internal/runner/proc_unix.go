//go:build !windows

package runner

import (
	"errors"
	"syscall"
)

// terminateGraceful asks the process to exit (SIGTERM).
func terminateGraceful(pid int) error {
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

// terminateForceful kills the process (SIGKILL).
func terminateForceful(pid int) error {
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}
