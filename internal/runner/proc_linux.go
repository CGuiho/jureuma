//go:build linux

package runner

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// processSnapshot lists the visible processes through /proc. A process whose
// executable link cannot be read (gone, or another user's without privileges)
// is skipped: its executable path is never guessed.
func processSnapshot() ([]Proc, error) {
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	out := make([]Proc, 0, len(ents))
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		base := filepath.Join("/proc", e.Name())
		exe, err := os.Readlink(filepath.Join(base, "exe"))
		if err != nil {
			continue
		}
		ppid, user := procStatus(base)
		out = append(out, Proc{PID: pid, PPID: ppid, User: user, Executable: exe})
	}
	return out, nil
}

// procStatus reads the parent PID and the real user ID from
// /proc/<pid>/status. Both are empty when the file is unreadable.
func procStatus(base string) (ppid int, user string) {
	data, err := os.ReadFile(filepath.Join(base, "status"))
	if err != nil {
		return 0, ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "PPid:"):
			ppid, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "PPid:")))
		case strings.HasPrefix(line, "Uid:"):
			if f := strings.Fields(strings.TrimPrefix(line, "Uid:")); len(f) > 0 {
				user = f[0]
			}
		}
	}
	return ppid, user
}

// currentUser names the calling process's real user the way procStatus does.
func currentUser() string { return strconv.Itoa(os.Getuid()) }

// processUser returns the real user ID of pid ("", error when unverifiable).
func processUser(pid int) (string, error) {
	_, user := procStatus(filepath.Join("/proc", strconv.Itoa(pid)))
	if user == "" {
		return "", os.ErrProcessDone
	}
	return user, nil
}
