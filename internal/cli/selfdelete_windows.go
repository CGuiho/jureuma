//go:build windows

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/CGuiho/zeze/internal/paths"
)

// releaseRunningImage lets uninstall remove the executable that is running
// it. Windows refuses to delete a running image but allows renaming it, so
// when the running executable is one of the paths uninstall removes (the
// stable launcher in ~/.guiho/bin or a payload under home), it is moved into
// a unique ~/.guiho/.temp/zeze-uninstall-* directory. The returned
// function schedules that directory's deletion once this process has exited.
func releaseRunningImage(home, launcher string) func() {
	exe, err := os.Executable()
	if err != nil {
		return func() {}
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	inside := strings.EqualFold(filepath.Clean(exe), filepath.Clean(launcher))
	if rel, err := filepath.Rel(home, exe); err == nil && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
		inside = true
	}
	if !inside {
		return func() {}
	}
	tempRoot, err := paths.TempDir()
	if err != nil {
		return func() {}
	}
	if err := os.MkdirAll(tempRoot, 0o755); err != nil {
		return func() {}
	}
	dir, err := os.MkdirTemp(tempRoot, "zeze-uninstall-")
	if err != nil {
		return func() {}
	}
	if err := os.Rename(exe, filepath.Join(dir, filepath.Base(exe))); err != nil {
		os.Remove(dir)
		return func() {}
	}
	return func() {
		c := exec.Command("cmd.exe", "/c", "ping 127.0.0.1 -n 3 >nul & rmdir /s /q \""+dir+"\"")
		detach(c)
		if c.Start() == nil {
			_ = c.Process.Release()
		}
	}
}
