// Package paths resolves jureuma's platform-native locations and provides
// atomic file writes and CLI-owned locks.
package paths

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// CLIName is the canonical executable and home-directory name.
const CLIName = "jureuma"

// HomeOverrideEnv lets tests and sandboxes relocate the user home.
const HomeOverrideEnv = "JUREUMA_USER_HOME"

// UserHome returns the user's home directory.
func UserHome() (string, error) {
	if h := os.Getenv(HomeOverrideEnv); h != "" {
		return h, nil
	}
	return os.UserHomeDir()
}

// HomeRelocated reports whether HomeOverrideEnv points somewhere other than
// the user's home (HOME, or USERPROFILE on Windows). Protections such as the
// bypass refuse to run in that case: a single variable must not swap in an
// empty global configuration and a different audit log.
func HomeRelocated() bool {
	h := os.Getenv(HomeOverrideEnv)
	if h == "" {
		return false
	}
	real, err := os.UserHomeDir()
	if err != nil {
		return true
	}
	return !samePath(h, real)
}

func samePath(a, b string) bool {
	clean := func(p string) string {
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		if r, err := filepath.EvalSymlinks(p); err == nil {
			p = r
		}
		return filepath.Clean(p)
	}
	return clean(a) == clean(b)
}

// Home returns ~/.guiho/jureuma.
func Home() (string, error) {
	h, err := UserHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".guiho", CLIName), nil
}

// BinDir returns the shared ~/.guiho/bin.
func BinDir() (string, error) {
	h, err := UserHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".guiho", "bin"), nil
}

// TempDir returns the shared ~/.guiho/.temp.
func TempDir() (string, error) {
	h, err := UserHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".guiho", ".temp"), nil
}

// ProjectRoot returns the nearest ancestor of start containing jureuma.yaml,
// else the nearest containing .git, else start itself.
func ProjectRoot(start string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for _, marker := range []string{"jureuma.yaml", ".git"} {
		dir := abs
		for {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir, nil
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return abs, nil
}

// AtomicWrite writes data to a temporary file beside path and renames it into place.
func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, tempPattern(filepath.Base(path)))
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// TempMarker is the tag AtomicWrite puts in every temporary file name.
const TempMarker = ".tmp-"

// tempPattern returns the os.CreateTemp pattern for a temporary file beside
// base. When base carries a ".unencrypted." or ".encrypted." marker, the
// marker and extension are kept at the end (".config.tmp-123.unencrypted.json")
// so a temporary file left by a crash still matches the gitignore and agent
// deny-rule patterns for that marker.
func tempPattern(base string) string {
	for _, marker := range []string{".unencrypted.", ".encrypted."} {
		if i := strings.Index(base, marker); i > 0 {
			return "." + base[:i] + TempMarker + "*" + base[i:]
		}
	}
	return "." + base + TempMarker + "*"
}

// IsStaleTemp reports whether name looks like a temporary file AtomicWrite
// left behind (".<stem>.tmp-<n>.unencrypted.<ext>" or ".<base>.tmp-<n>").
func IsStaleTemp(name string) bool {
	return strings.HasPrefix(name, ".") && strings.Contains(name, TempMarker)
}

// ErrLocked is returned when another live process holds the lock.
var ErrLocked = errors.New("locked by another jureuma process")

// Lock takes an exclusive CLI-owned lock named by key under home/locks.
// A lock whose recorded process is gone is recovered. It retries for up to
// five seconds before returning ErrLocked.
func Lock(home, key string) (func(), error) {
	sum := sha256.Sum256([]byte(key))
	dir := filepath.Join(home, "locks")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, hex.EncodeToString(sum[:8])+".lock")
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(tokenBytes)
	content := fmt.Sprintf("%d %s\n", os.Getpid(), token)
	deadline := time.Now().Add(5 * time.Second)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, werr := f.WriteString(content)
			f.Close()
			if werr != nil {
				os.Remove(path)
				return nil, werr
			}
			return func() {
				if b, err := os.ReadFile(path); err == nil && string(b) == content {
					os.Remove(path)
				}
			}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		if stale(path) {
			os.Remove(path)
			continue
		}
		if time.Now().After(deadline) {
			return nil, ErrLocked
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func stale(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	fields := strings.Fields(string(b))
	if len(fields) != 2 {
		// Partially written; treat as stale only if old.
		if st, err := os.Stat(path); err == nil && time.Since(st.ModTime()) > time.Minute {
			return true
		}
		return false
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return true
	}
	return !ProcessAlive(pid)
}
