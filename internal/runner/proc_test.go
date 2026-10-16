package runner

import (
	"runtime"
	"testing"
)

// The payload path every selected process must run exactly.
const testPayload = "/home/u/.guiho/jureuma/versions/0.1.0/jureuma"

func TestSelectOldPayload(t *testing.T) {
	procs := []Proc{
		{PID: 1, PPID: 0, User: "1000", Executable: "/sbin/init"},
		{PID: 100, PPID: 1, User: "1000", Executable: testPayload},                  // the upgrading process itself
		{PID: 101, PPID: 100, User: "1000", Executable: testPayload},                // its own child
		{PID: 102, PPID: 101, User: "1000", Executable: testPayload},                // its grandchild
		{PID: 200, PPID: 1, User: "1000", Executable: testPayload},                  // a registered run instance
		{PID: 201, PPID: 200, User: "1000", Executable: testPayload},                // its child command (a jureuma run -- jureuma ...)
		{PID: 300, PPID: 1, User: "1000", Executable: testPayload},                  // an unrelated old payload process
		{PID: 301, PPID: 1, User: "1000", Executable: testPayload + " (deleted)"},   // deleted but still running
		{PID: 400, PPID: 1, User: "1000", Executable: "/usr/local/bin/jureuma"},     // same file name, different path
		{PID: 401, PPID: 1, User: "1000", Executable: "/home/u/.guiho/bin/jureuma"}, // the launcher, not the payload
		{PID: 500, PPID: 1, User: "0", Executable: testPayload},                     // another user's process
		{PID: 600, PPID: 1, User: "", Executable: testPayload},                      // owner unknown
		{PID: 700, PPID: 1, User: "1000", Executable: ""},                           // executable unverifiable
	}
	got := SelectOldPayload(procs, testPayload, "1000", 100, []int{200})
	// The run instance itself runs the old payload and is selected; only its
	// child command is spared.
	want := []int{200, 300, 301, 600}
	if len(got) != len(want) {
		t.Fatalf("selected %d processes, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].PID != w {
			t.Fatalf("selected %+v, want pids %v", got, want)
		}
	}
}

func TestSelectOldPayloadOrderAndEmpty(t *testing.T) {
	if got := SelectOldPayload(nil, testPayload, "1000", 1, nil); len(got) != 0 {
		t.Fatalf("no processes must select nothing, got %+v", got)
	}
	// An empty or unresolvable payload path selects nothing.
	for _, p := range []Proc{{PID: 9, PPID: 1, User: "1000", Executable: testPayload}} {
		if got := SelectOldPayload([]Proc{p}, "", "1000", 1, nil); len(got) != 0 {
			t.Fatalf("an empty target path must select nothing, got %+v", got)
		}
	}
	// The run instance itself is selected; only its descendants are spared.
	procs := []Proc{
		{PID: 200, PPID: 1, User: "1000", Executable: testPayload},
		{PID: 201, PPID: 200, User: "1000", Executable: testPayload},
		{PID: 202, PPID: 201, User: "1000", Executable: testPayload},
	}
	got := SelectOldPayload(procs, testPayload, "1000", 5, []int{200})
	if len(got) != 1 || got[0].PID != 200 {
		t.Fatalf("only the run instance itself must be selected, got %+v", got)
	}
}

func TestSelectOldPayloadMatchesResolvedPaths(t *testing.T) {
	// The comparison form resolves symlinks and is case-insensitive only on
	// Windows; a different path with the same file name never matches.
	a := Proc{PID: 10, PPID: 1, User: "u", Executable: testPayload}
	for _, exe := range []string{
		"/home/u/.guiho/jureuma/versions/0.2.0/jureuma",
		"/home/u/jureuma",
		"jureuma",
		"/home/u/.guiho/jureuma/versions/0.1.0/jureuma-x",
	} {
		p := a
		p.Executable = exe
		if got := SelectOldPayload([]Proc{p}, testPayload, "u", 1, nil); len(got) != 0 {
			t.Fatalf("%q must not match %q", exe, testPayload)
		}
	}
}

func TestIsDescendant(t *testing.T) {
	procs := []Proc{
		{PID: 1, PPID: 0},
		{PID: 2, PPID: 1},
		{PID: 3, PPID: 2},
		{PID: 4, PPID: 3}, // closes a cycle below
		{PID: 5, PPID: 9}, // parent not listed
	}
	cases := []struct {
		pid   int
		roots []int
		want  bool
	}{
		{3, []int{1}, true},
		{3, []int{2}, true},
		{3, []int{3}, false}, // a process is not its own descendant
		{3, []int{4}, false},
		{4, []int{1}, true},
		{5, []int{1}, false},
		{1, []int{2}, false},
		{3, nil, false},
	}
	for _, c := range cases {
		if got := isDescendant(procs, c.pid, c.roots); got != c.want {
			t.Errorf("isDescendant(%d, %v) = %v, want %v", c.pid, c.roots, got, c.want)
		}
	}
}

func TestNormalizeExecutable(t *testing.T) {
	if got := normalizeExecutable(""); got != "" {
		t.Fatalf("empty path must stay empty, got %q", got)
	}
	if got := normalizeExecutable("  " + testPayload + " (deleted)  "); got == "" {
		t.Fatal("a deleted marker path must still normalize")
	}
	if runtime.GOOS != "windows" {
		if normalizeExecutable(testPayload) == normalizeExecutable("/home/u/jureuma") {
			t.Fatal("different paths must not normalize equally")
		}
	}
}
