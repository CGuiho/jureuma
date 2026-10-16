//go:build linux

package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestProcessSnapshotFindsSelf(t *testing.T) {
	procs, err := processSnapshot()
	if err != nil {
		t.Fatalf("process snapshot: %v", err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range procs {
		if p.PID != os.Getpid() {
			continue
		}
		found = true
		if normalizeExecutable(p.Executable) != normalizeExecutable(self) {
			t.Fatalf("own executable = %q, want %q", p.Executable, self)
		}
		if p.User != currentUser() {
			t.Fatalf("own user = %q, want %q", p.User, currentUser())
		}
		if p.PPID <= 0 {
			t.Fatalf("own ppid = %d", p.PPID)
		}
	}
	if !found {
		t.Fatal("the running test process is missing from the snapshot")
	}
}

// TestHelperHold is the re-exec helper: a copy of this test binary started at
// a chosen path with RUNNER_TEST_HOLD=1 holds a process open at that exact
// executable path, the way a running jureuma payload does.
func TestHelperHold(t *testing.T) {
	if os.Getenv("RUNNER_TEST_HOLD") != "1" {
		t.Skip("helper for TestTerminateOldPayload*")
	}
	// A sleep loop, not a bare select: the Go deadlock detector kills a
	// process whose goroutines are all blocked with no timers.
	for {
		time.Sleep(time.Hour)
	}
}

// holdAt copies the test binary to exe and starts it there, holding a
// process open whose verified executable path is exactly exe. When detached
// is set the process is reparented to init (a foreign process); otherwise it
// stays a child of the test (a CLI-launched child command).
func holdAt(t *testing.T, exe string, detached bool) *exec.Cmd {
	t.Helper()
	me, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(me)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, data, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=TestHelperHold")
	cmd.Env = append(os.Environ(), "RUNNER_TEST_HOLD=1")
	if detached {
		sh, err := exec.LookPath("sh")
		if err != nil {
			t.Skip("no sh")
		}
		spawn := exec.Command(sh, "-c", `"`+exe+`" -test.run=TestHelperHold >/dev/null 2>&1 &`)
		spawn.Env = cmd.Env
		if out, err := spawn.CombinedOutput(); err != nil {
			t.Fatalf("start %s: %v %s", exe, err, out)
		}
		return nil
	}
	cmd.Stdout, cmd.Stderr = nil, nil
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return cmd
}

// matchingPID returns the PID of the live process running exactly exe.
func matchingPID(t *testing.T, exe string) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		procs, err := processSnapshot()
		if err == nil {
			for _, p := range SelectOldPayload(procs, exe, currentUser(), os.Getpid(), nil) {
				return p.PID
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no process runs %s", exe)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestTerminateOldPayload holds a process open at the previous payload path
// and verifies that TerminateOldPayload terminates it (graceful signal, death
// verification) while a same-named process at another path survives.
func TestTerminateOldPayload(t *testing.T) {
	home := t.TempDir()
	payload := filepath.Join(home, "versions", "0.1.0", "jureuma")
	// The bystander has the same file name at a different path: selection is
	// by verified executable path, never by file name.
	bystander := filepath.Join(t.TempDir(), "jureuma")

	holdAt(t, payload, true)
	target := matchingPID(t, payload)
	holdAt(t, bystander, true)
	foreign := matchingPID(t, bystander)

	if err := TerminateOldPayload(home, payload); err != nil {
		t.Fatalf("terminate: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for processAlive(target) {
		if time.Now().After(deadline) {
			t.Fatal("the old payload process is still alive after termination")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !processAlive(foreign) {
		t.Fatal("a same-named process at another path was terminated")
	}
	// Idempotent: a second call with nothing left to terminate succeeds.
	if err := TerminateOldPayload(home, payload); err != nil {
		t.Fatalf("second terminate: %v", err)
	}
	_ = syscall.Kill(foreign, syscall.SIGKILL) // clean up the bystander
}

func TestTerminateOldPayloadSparesOwnChildren(t *testing.T) {
	home := t.TempDir()
	payload := filepath.Join(home, "versions", "0.1.0", "jureuma")

	// A child of this process running the payload path stands in for a
	// CLI-launched child command and must be spared.
	child := holdAt(t, payload, false)
	defer func() {
		_ = child.Process.Kill()
		_, _ = child.Process.Wait()
	}()
	if err := TerminateOldPayload(home, payload); err != nil {
		t.Fatalf("terminate: %v", err)
	}
	if err := child.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("the CLI's own child was terminated: %v", err)
	}
}
