//go:build windows

package runner

import (
	"os"
	"strings"
)

// On Windows the console delivers Ctrl+C to the child directly; jureuma only
// needs to survive it until the child exits.
var forwardedSignals = []os.Signal{os.Interrupt}

func envKey(name string) string { return strings.ToUpper(name) }

func exitCode(ps *os.ProcessState) int {
	if ps == nil {
		return 1
	}
	return ps.ExitCode()
}
