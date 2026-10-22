//go:build windows

package main

import (
	"fmt"
	"os"
)

// delegate starts the payload attached to this console, wires the launcher's
// standard streams, waits for it and returns its exact exit code. The shell
// stays attached until the payload completes because Wait blocks.
func delegate(payload string, args []string) (int, error) {
	argv := append([]string{payload}, args...)
	attr := &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		Env:   os.Environ(),
	}
	process, err := os.StartProcess(payload, argv, attr)
	if err != nil {
		return 0, fmt.Errorf("cannot start %s: %w", payload, err)
	}
	state, err := process.Wait()
	if err != nil {
		return 1, fmt.Errorf("cannot wait for %s: %w", payload, err)
	}
	code := state.ExitCode()
	if code < 0 {
		// The payload was terminated without an exit code.
		code = 1
	}
	return code, nil
}
