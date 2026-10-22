package agentres

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// SkillDestinations returns the supported global agent-skill directories for
// userHome, in this order: ~/.claude/skills (only when ~/.claude exists),
// ~/.agents/skills (always), ~/.codex/skills (only when ~/.codex exists).
func SkillDestinations(userHome string) []string {
	var out []string
	if isDir(filepath.Join(userHome, ".claude")) {
		out = append(out, filepath.Join(userHome, ".claude", "skills"))
	}
	out = append(out, filepath.Join(userHome, ".agents", "skills"))
	if isDir(filepath.Join(userHome, ".codex")) {
		out = append(out, filepath.Join(userHome, ".codex", "skills"))
	}
	return out
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// validID reports whether id is usable as a single directory name.
func validID(id string) bool {
	if id == "" || id == "." || id == ".." || strings.HasPrefix(id, ".") {
		return false
	}
	return !strings.ContainsAny(id, `/\:`) && !strings.ContainsRune(id, 0)
}

// validRel reports whether rel is a clean, relative, slash-separated path
// that stays inside the skill directory.
func validRel(rel string) bool {
	if rel == "" || strings.ContainsAny(rel, "\\:") || strings.ContainsRune(rel, 0) {
		return false
	}
	if path.IsAbs(rel) || path.Clean(rel) != rel || rel == "." {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, "../")
}

func randSuffix() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprint(os.Getpid())
	}
	return hex.EncodeToString(b)
}

// InstallSkill writes s into dest/<s.ID>/, replacing any existing directory
// there. The new copy is built in a temporary sibling directory and swapped
// in by rename; on failure the previous copy is restored. An existing
// dest/<s.ID> that is not a real directory (a file or a symlink) is refused,
// because zeze does not own it.
func InstallSkill(s Skill, dest string) error {
	if !validID(s.ID) {
		return errors.New("invalid skill id")
	}
	for rel := range s.Files {
		if !validRel(rel) {
			return fmt.Errorf("skill %s: invalid file path", s.ID)
		}
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("create skill destination %s: %w", dest, err)
	}
	target := filepath.Join(dest, s.ID)
	fi, err := os.Lstat(target)
	exists := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect %s: %w", target, err)
	}
	if exists && (fi.Mode()&fs.ModeSymlink != 0 || !fi.IsDir()) {
		return fmt.Errorf("%s exists and is not a directory managed by zeze", target)
	}

	suffix := randSuffix()
	tmp := filepath.Join(dest, "."+s.ID+".zeze-tmp-"+suffix)
	if err := os.Mkdir(tmp, 0o755); err != nil {
		return fmt.Errorf("create temporary skill directory in %s: %w", dest, err)
	}
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			os.RemoveAll(tmp)
		}
	}()
	for rel, data := range s.Files {
		p := filepath.Join(tmp, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return fmt.Errorf("write skill %s: %w", s.ID, err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			return fmt.Errorf("write skill %s file %s: %w", s.ID, rel, err)
		}
	}

	var old string
	if exists {
		old = filepath.Join(dest, "."+s.ID+".zeze-old-"+suffix)
		if err := os.Rename(target, old); err != nil {
			return fmt.Errorf("replace %s: %w", target, err)
		}
	}
	if err := os.Rename(tmp, target); err != nil {
		if old != "" {
			_ = os.Rename(old, target)
		}
		return fmt.Errorf("install %s: %w", target, err)
	}
	cleanupTmp = false
	if old != "" {
		_ = os.RemoveAll(old)
	}
	return nil
}

// SkillInstalledCurrent reports whether dest/<s.ID>/ is a real directory
// containing exactly the files of s with identical contents.
func SkillInstalledCurrent(s Skill, dest string) bool {
	if !validID(s.ID) {
		return false
	}
	target := filepath.Join(dest, s.ID)
	fi, err := os.Lstat(target)
	if err != nil || !fi.IsDir() {
		return false
	}
	seen := 0
	err = filepath.WalkDir(target, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return errors.New("not regular")
		}
		rel, err := filepath.Rel(target, p)
		if err != nil {
			return err
		}
		want, ok := s.Files[filepath.ToSlash(rel)]
		if !ok {
			return errors.New("extra file")
		}
		got, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if !bytes.Equal(got, want) {
			return errors.New("content differs")
		}
		seen++
		return nil
	})
	return err == nil && seen == len(s.Files)
}
