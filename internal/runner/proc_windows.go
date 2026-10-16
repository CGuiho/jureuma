//go:build windows

package runner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// winProc is one Win32_Process record as PowerShell reports it.
type winProc struct {
	PID        int64  `json:"ProcessId"`
	PPID       int64  `json:"ParentProcessId"`
	Executable string `json:"ExecutablePath"`
}

// processSnapshot lists the processes through PowerShell/CIM. Processes
// without an ExecutablePath (system processes) are skipped: their executable
// path is never guessed. Ownership is not resolved here; processUser verifies
// it for the few selected candidates.
func processSnapshot() ([]Proc, error) {
	script := `Get-CimInstance Win32_Process | Select-Object ProcessId,ParentProcessId,ExecutablePath | ConvertTo-Json -Compress`
	out, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return nil, err
	}
	var many []winProc
	if err := json.Unmarshal(out, &many); err != nil {
		var one winProc
		if err := json.Unmarshal(out, &one); err != nil {
			return nil, err
		}
		many = []winProc{one}
	}
	procs := make([]Proc, 0, len(many))
	for _, w := range many {
		if strings.TrimSpace(w.Executable) == "" {
			continue
		}
		procs = append(procs, Proc{
			PID:        int(w.PID),
			PPID:       int(w.PPID),
			Executable: w.Executable,
		})
	}
	return procs, nil
}

// currentUser names the calling process's user the way GetOwner does.
func currentUser() string { return strings.TrimSpace(os.Getenv("USERNAME")) }

// processUser returns the owner of pid ("", error when unverifiable).
func processUser(pid int) (string, error) {
	filter := fmt.Sprintf("ProcessId = %d", pid)
	script := fmt.Sprintf("$p=Get-CimInstance Win32_Process -Filter '%s'; if ($null -eq $p) { exit 1 }; $o=Invoke-CimMethod -InputObject $p -MethodName GetOwner; $o.User", filter)
	out, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return "", err
	}
	user := strings.TrimSpace(string(out))
	if user == "" {
		return "", errors.New("Windows process identity is unavailable")
	}
	return user, nil
}

// terminateGraceful asks the process to close (WM_CLOSE).
func terminateGraceful(pid int) error { return taskkill(pid, false) }

// terminateForceful kills the process.
func terminateForceful(pid int) error { return taskkill(pid, true) }

func taskkill(pid int, force bool) error {
	args := []string{"/PID", strconv.Itoa(pid)}
	if force {
		args = append(args, "/F")
	}
	if out, err := exec.Command("taskkill.exe", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("taskkill %d: %w (%s)", pid, err, strings.TrimSpace(string(out)))
	}
	return nil
}
