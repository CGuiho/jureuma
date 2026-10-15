//go:build !windows

package main

import (
	"fmt"
	"os"
	"syscall"
)

// delegate replaces this process with payload (exec semantics): the payload
// inherits the original arguments, environment, working directory and
// standard streams, and its exit status becomes the launcher's. It returns
// only when the payload cannot be started.
func delegate(payload string, args []string) (int, error) {
	argv := append([]string{payload}, args...)
	if err := syscall.Exec(payload, argv, os.Environ()); err != nil {
		return 0, fmt.Errorf("cannot exec %s: %w", payload, err)
	}
	// Exec replaces the process on success; this line is unreachable.
	return 0, fmt.Errorf("cannot exec %s", payload)
}
