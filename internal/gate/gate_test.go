package gate

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func newTestGate(t *testing.T, term bool, env map[string]string, answer string) (*Gate, *bytes.Buffer) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "tty")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	out := &bytes.Buffer{}
	return &Gate{
		Stdin:      f,
		Stderr:     f,
		Getenv:     envOf(env),
		Prompt:     strings.NewReader(answer),
		Out:        out,
		IsTerminal: func(*os.File) bool { return term },
		Word:       func() string { return "maple-17" },
	}, out
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name   string
		term   bool
		env    map[string]string
		extra  []string
		answer string
		reason string // "" means success
	}{
		{"ok", true, nil, nil, "maple-17\n", ""},
		{"ok trimmed", true, nil, nil, "  maple-17 \r\n", ""},
		{"ok no newline", true, nil, nil, "maple-17", ""},
		{"not terminal", false, nil, nil, "maple-17\n", ReasonNotTerminal},
		{"agent marker", true, map[string]string{"CLAUDECODE": "1"}, nil, "maple-17\n", ReasonAgent},
		{"empty marker ignored", true, map[string]string{"CLAUDECODE": ""}, nil, "maple-17\n", ""},
		{"extra marker", true, map[string]string{"MY_BOT": "yes"}, []string{"MY_BOT"}, "maple-17\n", ReasonAgent},
		{"wrong word", true, nil, nil, "maple-18\n", ReasonWrongAnswer},
		{"plain yes", true, nil, nil, "y\n", ReasonWrongAnswer},
		{"eof", true, nil, nil, "", ReasonNoAnswer},
		{"overlong", true, nil, nil, strings.Repeat("a", 1000) + "\n", ReasonWrongAnswer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, out := newTestGate(t, tt.term, tt.env, tt.answer)
			g.ExtraMarkers = tt.extra
			err := g.Check("reveal 1 value")
			if tt.reason == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got := out.String(); got != `type "maple-17" to reveal 1 value: ` {
					t.Fatalf("prompt %q", got)
				}
				return
			}
			if !errors.Is(err, ErrRefused) {
				t.Fatalf("want ErrRefused, got %v", err)
			}
			var re *RefusedError
			if !errors.As(err, &re) || re.Reason != tt.reason {
				t.Fatalf("reason: got %v want %q", err, tt.reason)
			}
			if strings.Contains(err.Error(), "maple") || strings.Contains(err.Error(), "yes") {
				t.Fatalf("error leaks input: %q", err.Error())
			}
			if tt.reason == ReasonNotTerminal || tt.reason == ReasonAgent {
				if out.Len() != 0 {
					t.Fatalf("prompt shown although an earlier check failed")
				}
			}
		})
	}
}

func TestCheckStopsAtNewline(t *testing.T) {
	g, _ := newTestGate(t, true, nil, "")
	r := strings.NewReader("maple-17\nrest")
	g.Prompt = r
	if err := g.Check("x"); err != nil {
		t.Fatal(err)
	}
	if r.Len() != 4 {
		t.Fatalf("consumed past newline, %d bytes left", r.Len())
	}
}

func TestCheckNilFiles(t *testing.T) {
	g := &Gate{IsTerminal: func(*os.File) bool { return true }, Getenv: envOf(nil)}
	if err := g.Check("x"); !errors.Is(err, ErrRefused) {
		t.Fatalf("got %v", err)
	}
	var ng *Gate
	if err := ng.Check("x"); !errors.Is(err, ErrRefused) {
		t.Fatalf("got %v", err)
	}
}

func TestCheckSanitizesAction(t *testing.T) {
	g, out := newTestGate(t, true, nil, "maple-17\n")
	if err := g.Check("reveal\n\x1b[2Jvalues"); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(out.String(), "\n\x1b") {
		t.Fatalf("control characters in prompt: %q", out.String())
	}
}

func TestIsTerminal(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "f")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if IsTerminal(f) {
		t.Fatal("regular file reported as terminal")
	}
	if IsTerminal(nil) {
		t.Fatal("nil reported as terminal")
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if IsTerminal(r) {
		t.Fatal("pipe reported as terminal")
	}
	if dn, err := os.Open(os.DevNull); err == nil {
		defer dn.Close()
		// /dev/null is a character device; this documents that the check is
		// ModeCharDevice-based, matching the contract.
		_ = IsTerminal(dn)
	}
}

func TestDetectAgent(t *testing.T) {
	env := map[string]string{"CURSOR_AGENT": "1", "CLAUDECODE": "1", "X_EXTRA": "v"}
	m, ok := DetectAgent(envOf(env), []string{"X_EXTRA"})
	if !ok || m != "CLAUDECODE" {
		t.Fatalf("got %q %v", m, ok)
	}
	all := DetectAgents(envOf(env), []string{"X_EXTRA", "CLAUDECODE", ""})
	if strings.Join(all, ",") != "CLAUDECODE,CURSOR_AGENT,X_EXTRA" {
		t.Fatalf("got %v", all)
	}
	if _, ok := DetectAgent(envOf(nil), nil); ok {
		t.Fatal("found marker in empty env")
	}
	for _, m := range DefaultMarkers {
		got, ok := DetectAgent(envOf(map[string]string{m: "secret-value"}), nil)
		if !ok || got != m {
			t.Fatalf("marker %s not detected", m)
		}
	}
}

func TestRandomWord(t *testing.T) {
	re := regexp.MustCompile(`^[a-z]+-[0-9]{2}$`)
	seen := map[string]bool{}
	for range 200 {
		w := RandomWord()
		if !re.MatchString(w) {
			t.Fatalf("bad word %q", w)
		}
		seen[w] = true
	}
	if len(seen) < 50 {
		t.Fatalf("words not random enough: %d distinct", len(seen))
	}
	for _, w := range words {
		if !regexp.MustCompile(`^[a-z]+$`).MatchString(w) {
			t.Fatalf("bad list word %q", w)
		}
	}
}

func TestAudit(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".guiho", "zeze")
	t.Setenv("ZEZE_USER_HOME", t.TempDir())
	ts := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	if err := Audit(home, AuditEvent{Time: ts, Command: "get", Paths: []string{"config.encrypted.json"}, Outcome: "refused", Markers: []string{"CLAUDECODE"}}); err != nil {
		t.Fatal(err)
	}
	if err := Audit(home, AuditEvent{Command: "decrypt", Outcome: "bypass", Bypass: true}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, AuditFileName)
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode().Perm())
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var lines []map[string]any
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("bad json line: %v", err)
		}
		lines = append(lines, m)
	}
	if len(lines) != 2 {
		t.Fatalf("got %d lines", len(lines))
	}
	if lines[0]["time"] != "2026-10-13T12:00:00Z" || lines[0]["command"] != "get" || lines[0]["outcome"] != "refused" || lines[0]["bypass"] != false {
		t.Fatalf("line 0: %v", lines[0])
	}
	if lines[1]["bypass"] != true {
		t.Fatalf("line 1: %v", lines[1])
	}
	if _, ok := lines[1]["paths"].([]any); !ok {
		t.Fatalf("paths should be an array: %v", lines[1])
	}
}

func TestAuditTightensMode(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, AuditFileName)
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Audit(home, AuditEvent{Command: "get", Outcome: "ok"}); err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(path)
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode().Perm())
	}
}

func TestAuditRejectsSymlink(t *testing.T) {
	home := t.TempDir()
	target := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(home, AuditFileName)); err != nil {
		t.Skip("symlinks unsupported")
	}
	if err := Audit(home, AuditEvent{Command: "get"}); !errors.Is(err, ErrAuditUnsafe) {
		t.Fatalf("got %v", err)
	}
}

func TestAuditEmptyHome(t *testing.T) {
	if err := Audit("", AuditEvent{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestIsTerminalRejectsDevNull(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Skip("no null device")
	}
	defer f.Close()
	if IsTerminal(f) {
		t.Fatalf("IsTerminal(%s) = true, want false", os.DevNull)
	}
}
