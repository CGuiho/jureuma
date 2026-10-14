package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/CGuiho/jureuma/internal/paths"
	"github.com/CGuiho/jureuma/internal/xerr"
	"github.com/spf13/cobra"
)

const aliasStateFile = "alias.json"

type aliasState struct {
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	Source    string `json:"source"`
	Installed string `json:"installed"`
}

func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func readAliasState(home string) (*aliasState, bool) {
	data, err := os.ReadFile(filepath.Join(home, "state", aliasStateFile))
	if err != nil {
		return nil, false
	}
	var st aliasState
	if json.Unmarshal(data, &st) != nil || st.Path == "" {
		return nil, false
	}
	return &st, true
}

func jujuPath() (string, error) {
	bin, err := paths.BinDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(bin, exeName("juju")), nil
}

// aliasSource returns the stable launcher when installed, else this executable.
func aliasSource() (string, error) {
	if bin, err := paths.BinDir(); err == nil {
		launcher := filepath.Join(bin, exeName("jureuma"))
		if st, err := os.Stat(launcher); err == nil && st.Mode().IsRegular() {
			return launcher, nil
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	return exe, nil
}

func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".juju.tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		_ = os.Remove(dst)
	}
	return os.Rename(tmpName, dst)
}

func (a *app) cmdEnableJuju() *cobra.Command {
	return &cobra.Command{
		Use:   "juju-enable",
		Short: "install the juju alias in ~/.guiho/bin",
		Long: `Installs ~/.guiho/bin/juju (juju.exe on Windows), a copy of the jureuma launcher (or of this
executable when no launcher is installed), so "juju <args>" behaves exactly like
"jureuma <args>". Refuses to overwrite a juju it did not install, and warns when another juju
appears earlier on PATH. Re-run it after upgrading to refresh the copy.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := paths.Home()
			if err != nil {
				return xerr.Failure("cannot resolve the jureuma home")
			}
			dst, err := jujuPath()
			if err != nil {
				return xerr.Failure("cannot resolve ~/.guiho/bin")
			}
			st, owned := readAliasState(home)
			if fi, err := os.Lstat(dst); err == nil {
				if !fi.Mode().IsRegular() {
					return xerr.Failure("%s exists and is not a regular file; refusing to overwrite it", dst)
				}
				sum, _ := fileSHA256(dst)
				if !owned || st.Path != dst || st.SHA256 != sum {
					return xerr.Failure("%s exists and was not installed by jureuma; refusing to overwrite it", dst)
				}
			} else if !errors.Is(err, fs.ErrNotExist) {
				return xerr.Failure("cannot inspect %s", dst)
			}
			src, err := aliasSource()
			if err != nil {
				return xerr.Failure("cannot locate the jureuma executable")
			}
			if err := copyExecutable(src, dst); err != nil {
				return xerr.Failure("cannot install %s", dst)
			}
			sum, err := fileSHA256(dst)
			if err != nil {
				return xerr.Failure("cannot verify %s", dst)
			}
			data, _ := json.MarshalIndent(aliasState{Path: dst, SHA256: sum, Source: src, Installed: time.Now().UTC().Format(time.RFC3339)}, "", "  ")
			if err := os.MkdirAll(filepath.Join(home, "state"), 0o700); err != nil {
				return xerr.Failure("cannot create the jureuma state directory")
			}
			if err := paths.AtomicWrite(filepath.Join(home, "state", aliasStateFile), append(data, '\n'), 0o600); err != nil {
				return xerr.Failure("cannot record the alias")
			}
			outf(cmd, "installed %s (alias of jureuma)\n", dst)
			if found, err := exec.LookPath("juju"); err == nil {
				if abs, _ := filepath.Abs(found); abs != dst {
					errf(cmd, "warning: another juju appears earlier on PATH: %s\n", found)
				}
			} else {
				errf(cmd, "note: %s is not on PATH; add ~/.guiho/bin to PATH to use juju\n", filepath.Dir(dst))
			}
			return nil
		},
	}
}

// removeAlias deletes the alias when jureuma installed it.
func removeAlias(home string, force bool) (string, error) {
	st, owned := readAliasState(home)
	dst, err := jujuPath()
	if err != nil {
		return "", xerr.Failure("cannot resolve ~/.guiho/bin")
	}
	if !owned {
		return "", xerr.Failure("no juju alias installed by jureuma")
	}
	// Only ever touch ~/.guiho/bin/juju: a path recorded elsewhere in the
	// state file is never followed.
	if !samePathLoose(st.Path, dst) && !force {
		return "", xerr.Failure("the recorded alias is not %s; refusing to remove it (use --force)", dst)
	}
	if fi, err := os.Lstat(dst); err == nil {
		sum, _ := fileSHA256(dst)
		if (!fi.Mode().IsRegular() || sum != st.SHA256) && !force {
			return "", xerr.Failure("%s changed since jureuma installed it; refusing to remove it (use --force)", dst)
		}
		if err := os.Remove(dst); err != nil {
			return "", xerr.Failure("cannot remove %s", dst)
		}
	}
	_ = os.Remove(filepath.Join(home, "state", aliasStateFile))
	return dst, nil
}

// aliasOwned reports whether ~/.guiho/bin/juju is the alias jureuma
// installed: recorded at that exact path, a regular file, same SHA-256.
func aliasOwned(home string) (dst string, recorded, owned bool) {
	st, ok := readAliasState(home)
	dst, err := jujuPath()
	if err != nil || !ok {
		return dst, ok, false
	}
	if !samePathLoose(st.Path, dst) {
		return dst, true, false
	}
	fi, err := os.Lstat(dst)
	if err != nil || !fi.Mode().IsRegular() {
		return dst, true, false
	}
	sum, err := fileSHA256(dst)
	return dst, true, err == nil && sum == st.SHA256
}

func samePathLoose(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func (a *app) cmdDisableJuju() *cobra.Command {
	var force bool
	c := &cobra.Command{
		Use:   "juju-disable",
		Short: "remove the juju alias installed by juju-enable",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := paths.Home()
			if err != nil {
				return xerr.Failure("cannot resolve the jureuma home")
			}
			dst, err := removeAlias(home, force)
			if err != nil {
				return err
			}
			outf(cmd, "removed %s\n", dst)
			return nil
		},
	}
	c.Flags().BoolVar(&force, "force", false, "remove the alias even if it changed since it was installed")
	return c
}
