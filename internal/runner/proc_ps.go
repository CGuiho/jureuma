//go:build !linux && !windows

package runner

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// processSnapshot lists the processes with ps (macOS and the other BSDs have
// no /proc). "comm" names the executable path there; a process whose line
// cannot be parsed is skipped, never guessed.
func processSnapshot() ([]Proc, error) {
	out, err := exec.Command("ps", "-axo", "pid=,ppid=,uid=,comm=").Output()
	if err != nil {
		return nil, err
	}
	var procs []Proc
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 4 {
			continue
		}
		pid, err := strconv.Atoi(f[0])
		if err != nil {
			continue
		}
		ppid, err := strconv.Atoi(f[1])
		if err != nil {
			continue
		}
		procs = append(procs, Proc{
			PID:        pid,
			PPID:       ppid,
			User:       f[2],
			Executable: strings.Join(f[3:], " "),
		})
	}
	return procs, nil
}

// currentUser names the calling process's real user the way ps does.
func currentUser() string { return strconv.Itoa(os.Getuid()) }

// processUser returns the real user ID of pid ("", error when unverifiable).
func processUser(pid int) (string, error) {
	out, err := exec.Command("ps", "-o", "uid=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return "", err
	}
	f := strings.Fields(string(out))
	if len(f) == 0 {
		return "", errors.New("process is gone")
	}
	return f[0], nil
}
