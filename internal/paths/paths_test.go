package paths

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// lockFile is the lock path Lock derives for key under home.
func lockFile(home, key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(home, "locks", hex.EncodeToString(sum[:8])+".lock")
}

// deadPID returns the PID of a process that has exited and been reaped.
func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	if err := cmd.Run(); err != nil {
		t.Fatalf("helper process: %v", err)
	}
	if ProcessAlive(cmd.Process.Pid) {
		t.Fatal("the helper process is still alive")
	}
	return cmd.Process.Pid
}

func TestLock(t *testing.T) {
	t.Setenv(HomeOverrideEnv, t.TempDir())
	home := t.TempDir()
	unlock, err := Lock(home, "config")
	if err != nil {
		t.Fatal(err)
	}

	// A different key must not contend.
	unlockOther, err := Lock(home, "other")
	if err != nil {
		t.Fatalf("independent key: %v", err)
	}
	unlockOther()

	// The same key contends and returns ErrLocked after its retry window.
	if _, err := Lock(home, "config"); !errors.Is(err, ErrLocked) {
		t.Fatalf("contending Lock = %v, want ErrLocked", err)
	}

	// Unlock removes only the lock the caller owns.
	unlock()
	if _, err := os.Stat(lockFile(home, "config")); !os.IsNotExist(err) {
		t.Fatal("unlock must remove the owned lock")
	}
	unlock() // second unlock is a safe no-op

	unlock2, err := Lock(home, "config")
	if err != nil {
		t.Fatalf("relock after unlock: %v", err)
	}
	unlock2()
}

func TestLockUnlockChecksToken(t *testing.T) {
	t.Setenv(HomeOverrideEnv, t.TempDir())
	home := t.TempDir()
	unlock, err := Lock(home, "config")
	if err != nil {
		t.Fatal(err)
	}
	path := lockFile(home, "config")
	foreign := fmt.Sprintf("%d foreign-token\n", os.Getpid())
	if err := os.WriteFile(path, []byte(foreign), 0o600); err != nil {
		t.Fatal(err)
	}
	unlock()
	if b, err := os.ReadFile(path); err != nil || string(b) != foreign {
		t.Fatalf("unlock removed a lock it does not own: %q %v", b, err)
	}
}

func TestLockStaleRecovery(t *testing.T) {
	t.Setenv(HomeOverrideEnv, t.TempDir())
	home := t.TempDir()
	path := lockFile(home, "config")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		content string
		oldFile bool
	}{
		{"dead process", fmt.Sprintf("%d 0123456789abcdef\n", deadPID(t)), false},
		{"garbage pid", "not-a-pid 0123456789abcdef\n", false},
		{"partial write left behind", "12345\n", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(c.content), 0o600); err != nil {
				t.Fatal(err)
			}
			if c.oldFile {
				old := time.Now().Add(-2 * time.Minute)
				if err := os.Chtimes(path, old, old); err != nil {
					t.Fatal(err)
				}
			}
			unlock, err := Lock(home, "config")
			if err != nil {
				t.Fatalf("stale lock not recovered: %v", err)
			}
			unlock()
		})
	}
}

func TestProjectRoot(t *testing.T) {
	t.Setenv(HomeOverrideEnv, t.TempDir())
	base := t.TempDir()

	// b: only a .git marker.
	b := filepath.Join(base, "b")
	if err := os.MkdirAll(filepath.Join(b, "sub", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(b, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	// c: zeze.yaml at the root, a nearer .git below it.
	c := filepath.Join(base, "c")
	if err := os.MkdirAll(filepath.Join(c, "sub", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(c, "sub", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c, "zeze.yaml"), []byte("pairs: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// d: .git at the root, a nearer zeze.yaml below it.
	d := filepath.Join(base, "d")
	if err := os.MkdirAll(filepath.Join(d, "sub", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(d, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "sub", "zeze.yaml"), []byte("pairs: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// e: no markers at all.
	e := filepath.Join(base, "e", "deep")
	if err := os.MkdirAll(e, 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		start string
		want  string
	}{
		{"zeze.yaml wins over nearer .git", filepath.Join(c, "sub", "deep"), c},
		{"nearest zeze.yaml wins", filepath.Join(d, "sub", "deep"), filepath.Join(d, "sub")},
		{"falls back to .git", filepath.Join(b, "sub", "deep"), b},
		{"no marker returns start", e, e},
		{"root itself with marker", filepath.Join(d, "sub"), filepath.Join(d, "sub")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ProjectRoot(tc.start)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("ProjectRoot(%s) = %s, want %s", tc.start, got, tc.want)
			}
		})
	}
}

func TestTempNameKeepsMarker(t *testing.T) {
	dir := t.TempDir()
	for _, base := range []string{"config.unencrypted.json", "apps.api.unencrypted.jsonc", "config.encrypted.json"} {
		pat := tempPattern(base)
		f, err := os.CreateTemp(dir, pat)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(f.Name())
		f.Close()
		ext := filepath.Ext(base)
		ok, _ := filepath.Match("*.unencrypted"+ext, name)
		if base == "config.encrypted.json" {
			ok, _ = filepath.Match("*.encrypted.json", name)
		}
		if !ok || !IsStaleTemp(name) {
			t.Errorf("%s: temp name %s does not keep the marker and extension", base, name)
		}
	}
	if got := tempPattern("cache.json"); got != ".cache.json.tmp-*" {
		t.Errorf("plain name pattern = %s", got)
	}
}

func TestAtomicWriteLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.unencrypted.json")
	if err := AtomicWrite(p, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	ents, _ := os.ReadDir(dir)
	if len(ents) != 1 {
		t.Fatalf("left %d entries", len(ents))
	}
}

func TestHomeRelocated(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("USERPROFILE", h)
	t.Setenv(HomeOverrideEnv, "")
	if HomeRelocated() {
		t.Fatal("no override is not relocated")
	}
	t.Setenv(HomeOverrideEnv, h)
	if HomeRelocated() {
		t.Fatal("override equal to HOME is not relocated")
	}
	t.Setenv(HomeOverrideEnv, t.TempDir())
	if !HomeRelocated() {
		t.Fatal("override elsewhere is relocated")
	}
}
