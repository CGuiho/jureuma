package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Process inspection for the upgrade transaction (convention: Identify other
// running instances through a CLI-owned instance registry and operating-system
// process inspection). Before activation the transaction must terminate every
// other process owned by the current user whose verified executable path is
// the previous payload. Selection never matches by filename, never targets the
// calling process, and never targets an unrelated child command launched by
// the CLI (a descendant of the calling process or of a registered run
// instance).

// Proc is one operating-system process as observed by the platform adapter.
// Executable is the verified, symlink-resolved absolute path of the process
// image; it is empty when the platform could not verify it. User names the
// owner the way the platform names it; it is empty when unknown.
type Proc struct {
	PID        int
	PPID       int
	User       string
	Executable string
}

// SelectOldPayload returns the processes the upgrade transaction must
// terminate before activation: every process owned by user whose verified
// executable path is exactly oldPayload, except the calling process
// (selfPID), every descendant of it, and every descendant of a registered run
// instance (runPIDs). A run instance process itself is selected when it runs
// the old payload: its registered child command is not. A process whose
// executable path is not exactly oldPayload is never selected, whatever its
// file name is.
func SelectOldPayload(procs []Proc, oldPayload, user string, selfPID int, runPIDs []int) []Proc {
	target := normalizeExecutable(oldPayload)
	if target == "" {
		return nil
	}
	protect := append([]int{selfPID}, runPIDs...)
	var out []Proc
	for _, p := range procs {
		if p.PID <= 0 || p.PID == selfPID {
			continue
		}
		if normalizeExecutable(p.Executable) != target {
			continue
		}
		if p.User != "" && user != "" && p.User != user {
			continue
		}
		if isDescendant(procs, p.PID, protect) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// normalizeExecutable is the comparison form of an executable path: absolute,
// symlink-resolved where possible, and without the Linux "/proc/<pid>/exe"
// " (deleted)" marker. Windows paths compare case-insensitively.
func normalizeExecutable(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimSuffix(p, " (deleted)")
	if p == "" {
		return ""
	}
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		p = resolved
	}
	p = filepath.Clean(p)
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	return p
}

// isDescendant reports whether pid has any of roots among its strict
// ancestors. A process is never its own descendant, so a root process itself
// is not protected by its own entry.
func isDescendant(procs []Proc, pid int, roots []int) bool {
	if len(roots) == 0 {
		return false
	}
	parent := make(map[int]int, len(procs))
	for _, p := range procs {
		parent[p.PID] = p.PPID
	}
	seen := map[int]bool{}
	for cur := pid; ; {
		pp, ok := parent[cur]
		if !ok || pp <= 0 || seen[cur] {
			return false
		}
		seen[cur] = true
		for _, r := range roots {
			if pp == r {
				return true
			}
		}
		cur = pp
	}
}

// TerminateOldPayload terminates every other process owned by the current
// user whose verified executable path is oldPayload. It sends a graceful
// signal first, escalates after a short grace period, and verifies death
// before returning. It returns an error when a verified old payload process
// survives: the caller must then fail without activating the new release.
// Processes that cannot be inspected, or whose owner cannot be verified as
// the current user, are never terminated: they are not verified old payload
// processes.
func TerminateOldPayload(home, oldPayload string) error {
	if home == "" || oldPayload == "" {
		return nil
	}
	procs, err := processSnapshot()
	if err != nil {
		// Best-effort inspection: nothing was verified, so nothing is
		// terminated and the transaction may proceed.
		return nil
	}
	var runPIDs []int
	for _, i := range Instances(home) {
		runPIDs = append(runPIDs, i.PID)
	}
	user := currentUser()
	var targets []Proc
	for _, p := range SelectOldPayload(procs, oldPayload, user, os.Getpid(), runPIDs) {
		u, uerr := processUser(p.PID)
		if uerr != nil || (u != "" && user != "" && u != user) {
			continue
		}
		targets = append(targets, p)
	}
	if len(targets) == 0 {
		return nil
	}
	for _, p := range targets {
		_ = terminateGraceful(p.PID)
	}
	waitDeath(targets, 2*time.Second)
	var survivors []Proc
	for _, p := range targets {
		if processAlive(p.PID) {
			_ = terminateForceful(p.PID)
			survivors = append(survivors, p)
		}
	}
	waitDeath(survivors, 3*time.Second)
	var left []string
	for _, p := range targets {
		if processAlive(p.PID) {
			left = append(left, fmt.Sprintf("%d", p.PID))
		}
	}
	if len(left) > 0 {
		return fmt.Errorf("the previous jureuma payload process %s could not be terminated", strings.Join(left, ", "))
	}
	return nil
}

// waitDeath waits until every process is gone or the deadline passes.
func waitDeath(procs []Proc, d time.Duration) {
	deadline := time.Now().Add(d)
	for {
		alive := false
		for _, p := range procs {
			if processAlive(p.PID) {
				alive = true
				break
			}
		}
		if !alive || time.Now().After(deadline) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}
