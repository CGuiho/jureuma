package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/CGuiho/jureuma/internal/paths"
)

// isolateHome points JUREUMA_USER_HOME at a temporary home and returns the
// jureuma CLI home it implies. Tests never touch the real home.
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv(paths.HomeOverrideEnv, home)
	jhome := filepath.Join(home, ".guiho", paths.CLIName)
	if err := os.MkdirAll(filepath.Join(jhome, "versions"), 0o755); err != nil {
		t.Fatal(err)
	}
	return jhome
}

// installPayload creates versions/<version>/jureuma as a regular file.
func installPayload(t *testing.T, jhome, version string) string {
	t.Helper()
	path := filepath.Join(jhome, "versions", version, payloadName())
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("payload"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeCurrent(t *testing.T, jhome, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(jhome, currentFile), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// recordingStarter returns a starter that records every call and answers with
// the configured exit code and error per payload path.
func recordingStarter(code int, errFor map[string]error) (*[]string, starter) {
	var calls []string
	return &calls, func(payload string, args []string) (int, error) {
		calls = append(calls, payload+" "+strings.Join(args, " "))
		if err := errFor[payload]; err != nil {
			return 0, err
		}
		return code, nil
	}
}

func TestReadPointer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, currentFile)
	cases := []struct {
		name    string
		content string
		write   bool
		active  string
		prev    string
		errText string
	}{
		{name: "valid", write: true, content: `{"protocol":1,"active":"1.2.3","previous":"1.2.2"}`, active: "1.2.3", prev: "1.2.2"},
		{name: "valid with trailing newline", write: true, content: "{\"protocol\":1,\"active\":\"0.1.0\",\"previous\":null}\n", active: "0.1.0", prev: ""},
		{name: "valid prerelease and build", write: true, content: `{"protocol":1,"active":"1.2.3-beta.1+build.7","previous":null}`, active: "1.2.3-beta.1+build.7"},
		{name: "missing file", write: false, errText: "missing or unreadable"},
		{name: "empty file", write: true, content: "", errText: "malformed"},
		{name: "too large", write: true, content: `{"protocol":1,"active":"1.2.3","previous":null,` + strings.Repeat(`"pad":0,`, 700) + `"z":0}`, errText: "malformed"},
		{name: "malformed json", write: true, content: `{"protocol":1,`, errText: "malformed"},
		{name: "two documents", write: true, content: `{"protocol":1,"active":"1.2.3","previous":null}{}`, errText: "exactly one JSON document"},
		{name: "unknown field", write: true, content: `{"protocol":1,"active":"1.2.3","previous":null,"extra":1}`, errText: "unexpected fields"},
		{name: "missing protocol", write: true, content: `{"active":"1.2.3","previous":null}`, errText: "no protocol"},
		{name: "missing active", write: true, content: `{"protocol":1,"previous":null}`, errText: "no active version"},
		{name: "missing previous", write: true, content: `{"protocol":1,"active":"1.2.3"}`, errText: "no previous field"},
		{name: "wrong protocol", write: true, content: `{"protocol":2,"active":"1.2.3","previous":null}`, errText: "protocol must be 1"},
		{name: "active null", write: true, content: `{"protocol":1,"active":null,"previous":null}`, errText: "active version must be a string"},
		{name: "active number", write: true, content: `{"protocol":1,"active":7,"previous":null}`, errText: "active version must be a string"},
		{name: "active not semver", write: true, content: `{"protocol":1,"active":"next","previous":null}`, errText: "not a valid SemVer"},
		{name: "active traversal", write: true, content: `{"protocol":1,"active":"../../etc","previous":null}`, errText: "not a valid SemVer"},
		{name: "previous not semver", write: true, content: `{"protocol":1,"active":"1.2.3","previous":"../x"}`, errText: "previous version is not a valid SemVer"},
		{name: "previous number", write: true, content: `{"protocol":1,"active":"1.2.3","previous":3}`, errText: "previous version must be a string"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_ = os.Remove(path)
			if tc.write {
				if err := os.WriteFile(path, []byte(tc.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			p, err := readPointer(path)
			if tc.errText != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errText) {
					t.Fatalf("want error %q, got %v", tc.errText, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if p.Active != tc.active || p.Previous != tc.prev {
				t.Fatalf("got %+v, want active %q previous %q", p, tc.active, tc.prev)
			}
		})
	}
}

func TestValidateRelative(t *testing.T) {
	for _, bad := range []string{"", " ", ".", "..", "/abs", `\abs`, "a/../../b", `..\b`, "C:evil", "a:b", "a\x00b", "//srv/share"} {
		if err := validateRelative(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	for _, good := range []string{"1.2.3", "1.2.3-beta.1", "1.2.3+build.5", "a/b"} {
		if err := validateRelative(good); err != nil {
			t.Errorf("%q rejected: %v", good, err)
		}
	}
}

func TestResolvePayload(t *testing.T) {
	jhome := isolateHome(t)
	versions := filepath.Join(jhome, "versions")
	installed := installPayload(t, jhome, "1.2.3")

	got, err := resolvePayload(versions, "1.2.3")
	if err != nil || got != installed {
		t.Fatalf("got %q %v, want %q", got, err, installed)
	}
	if _, err := resolvePayload(versions, "9.9.9"); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("missing payload accepted: %v", err)
	}
	if _, err := resolvePayload(versions, "../1.2.3"); err == nil {
		t.Fatal("traversal accepted")
	}
	if err := os.MkdirAll(filepath.Join(versions, "2.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(versions, "2.0.0", payloadName()), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := resolvePayload(versions, "2.0.0"); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("directory payload accepted: %v", err)
	}
	if runtime.GOOS != "windows" {
		link := filepath.Join(versions, "3.0.0")
		if err := os.MkdirAll(link, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(installed, filepath.Join(link, payloadName())); err != nil {
			t.Fatal(err)
		}
		if _, err := resolvePayload(versions, "3.0.0"); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("symlink payload accepted: %v", err)
		}
	}
}

func TestRunDispatchesActivePayload(t *testing.T) {
	jhome := isolateHome(t)
	installPayload(t, jhome, "1.2.3")
	installPayload(t, jhome, "1.2.2")
	writeCurrent(t, jhome, "{\"protocol\":1,\"active\":\"1.2.3\",\"previous\":\"1.2.2\"}\n")

	calls, start := recordingStarter(42, nil)
	var errOut bytes.Buffer
	if code := run([]string{"--version"}, start, &errOut); code != 42 {
		t.Fatalf("exit code %d, want 42", code)
	}
	if len(*calls) != 1 || !strings.HasPrefix((*calls)[0], filepath.Join(jhome, "versions", "1.2.3")) || !strings.HasSuffix((*calls)[0], " --version") {
		t.Fatalf("calls %q", *calls)
	}
	if errOut.Len() != 0 {
		t.Fatalf("unexpected output: %s", errOut.String())
	}
}

func TestRunFallsBackToPrevious(t *testing.T) {
	jhome := isolateHome(t)
	previous := installPayload(t, jhome, "1.2.2")
	writeCurrent(t, jhome, "{\"protocol\":1,\"active\":\"1.2.3\",\"previous\":\"1.2.2\"}\n")

	// The active payload is missing entirely.
	calls, start := recordingStarter(7, nil)
	var errOut bytes.Buffer
	if code := run(nil, start, &errOut); code != 7 {
		t.Fatalf("exit code %d, want 7", code)
	}
	if len(*calls) != 1 || !strings.HasPrefix((*calls)[0], previous) {
		t.Fatalf("calls %q", *calls)
	}
	if !strings.Contains(errOut.String(), "running the fallback version 1.2.2") {
		t.Fatalf("missing fallback warning: %s", errOut.String())
	}

	// The active payload exists but cannot be started.
	active := installPayload(t, jhome, "1.2.3")
	calls, start = recordingStarter(3, map[string]error{active: os.ErrPermission})
	errOut.Reset()
	if code := run(nil, start, &errOut); code != 3 {
		t.Fatalf("exit code %d, want 3", code)
	}
	if len(*calls) != 2 {
		t.Fatalf("calls %q", *calls)
	}
	if !strings.Contains(errOut.String(), "running the fallback version 1.2.2") {
		t.Fatalf("missing fallback warning: %s", errOut.String())
	}
}

func TestRunFailsWhenBothPayloadsFail(t *testing.T) {
	jhome := isolateHome(t)
	writeCurrent(t, jhome, "{\"protocol\":1,\"active\":\"1.2.3\",\"previous\":\"1.2.2\"}\n")

	// Both payloads are missing.
	calls, start := recordingStarter(0, nil)
	var errOut bytes.Buffer
	if code := run(nil, start, &errOut); code == 0 {
		t.Fatal("exit code 0 with both payloads unavailable")
	}
	if len(*calls) != 0 {
		t.Fatalf("calls %q", *calls)
	}
	out := errOut.String()
	for _, want := range []string{"the active version 1.2.3", "the fallback version 1.2.2", "reinstall:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %q", want, out)
		}
	}

	// Both payloads exist but neither can start.
	active := installPayload(t, jhome, "1.2.3")
	previous := installPayload(t, jhome, "1.2.2")
	calls, start = recordingStarter(0, map[string]error{active: os.ErrPermission, previous: os.ErrPermission})
	errOut.Reset()
	if code := run(nil, start, &errOut); code == 0 {
		t.Fatal("exit code 0 when both payloads cannot start")
	}
	if len(*calls) != 2 {
		t.Fatalf("calls %q", *calls)
	}
	if !strings.Contains(errOut.String(), "cannot start") {
		t.Fatalf("missing start failure: %s", errOut.String())
	}
}

func TestRunWithoutPreviousFails(t *testing.T) {
	jhome := isolateHome(t)
	writeCurrent(t, jhome, "{\"protocol\":1,\"active\":\"1.2.3\",\"previous\":null}\n")
	calls, start := recordingStarter(0, nil)
	var errOut bytes.Buffer
	if code := run(nil, start, &errOut); code == 0 {
		t.Fatal("exit code 0 without any payload")
	}
	if len(*calls) != 0 {
		t.Fatalf("calls %q", *calls)
	}
	if !strings.Contains(errOut.String(), "reinstall:") {
		t.Fatalf("missing reinstall hint: %s", errOut.String())
	}
}

func TestRunRejectsInvalidPointer(t *testing.T) {
	jhome := isolateHome(t)
	installPayload(t, jhome, "1.2.3")
	writeCurrent(t, jhome, `{"protocol":1,"active":"/etc","previous":null}`)
	calls, start := recordingStarter(0, nil)
	var errOut bytes.Buffer
	if code := run(nil, start, &errOut); code == 0 {
		t.Fatal("exit code 0 with an invalid pointer")
	}
	if len(*calls) != 0 {
		t.Fatalf("calls %q", *calls)
	}
}

func TestDelegateMissingPayload(t *testing.T) {
	if _, err := delegate(filepath.Join(t.TempDir(), "no-such-payload"), nil); err == nil {
		t.Fatal("delegating to a missing payload must fail")
	}
}
