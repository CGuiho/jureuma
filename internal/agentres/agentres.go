// Package agentres holds jureuma's embedded agent artifacts (skills, prompts,
// and the AGENTS.md instruction block) and the helpers that install them.
//
// Nothing in this package writes to stdout or stderr, and no error message
// contains file contents.
package agentres

import (
	"embed"
	"io/fs"
	"path"
	"sort"
	"strings"
)

//go:embed all:assets
var assets embed.FS

// Artifact IDs bundled with this release.
const (
	MainSkillID       = "guiho-s-jureuma"
	MainPromptID      = "guiho-p-jureuma"
	InstallPromptID   = "guiho-p-jureuma-install"
	UninstallPromptID = "guiho-p-jureuma-uninstall"
)

// Skill is one bundled agent skill. Files maps slash-separated paths relative
// to the skill directory (for example "SKILL.md") to their contents.
type Skill struct {
	ID    string
	Files map[string][]byte
}

// Prompt is one bundled one-time agent prompt.
type Prompt struct {
	ID   string
	Body string
}

// Skills returns every bundled skill, sorted by ID. The main skill is
// guiho-s-jureuma.
func Skills() []Skill {
	entries, err := fs.ReadDir(assets, "assets/skills")
	if err != nil {
		panic("agentres: embedded skills missing")
	}
	var out []Skill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		root := path.Join("assets/skills", e.Name())
		s := Skill{ID: e.Name(), Files: map[string][]byte{}}
		err := fs.WalkDir(assets, root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || isDescriptor(p) {
				return nil
			}
			b, err := assets.ReadFile(p)
			if err != nil {
				return err
			}
			s.Files[strings.TrimPrefix(p, root+"/")] = b
			return nil
		})
		if err != nil {
			panic("agentres: embedded skill unreadable")
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// SkillByID returns the bundled skill with the given ID.
func SkillByID(id string) (Skill, bool) {
	for _, s := range Skills() {
		if s.ID == id {
			return s, true
		}
	}
	return Skill{}, false
}

// Prompts returns every bundled prompt, sorted by ID. The default prompt is
// guiho-p-jureuma; the install prompt is guiho-p-jureuma-install.
func Prompts() []Prompt {
	entries, err := fs.ReadDir(assets, "assets/prompts")
	if err != nil {
		panic("agentres: embedded prompts missing")
	}
	var out []Prompt
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") || isDescriptor(name) {
			continue
		}
		b, err := assets.ReadFile(path.Join("assets/prompts", name))
		if err != nil {
			panic("agentres: embedded prompt unreadable")
		}
		out = append(out, Prompt{ID: strings.TrimSuffix(name, ".md"), Body: string(b)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// PromptByID returns the bundled prompt with the given ID.
func PromptByID(id string) (Prompt, bool) {
	for _, p := range Prompts() {
		if p.ID == id {
			return p, true
		}
	}
	return Prompt{}, false
}

// Instruction returns the managed AGENTS.md block body, without markers,
// with LF line endings and exactly one trailing newline.
func Instruction() string {
	b, err := assets.ReadFile("assets/instruction.md")
	if err != nil {
		panic("agentres: embedded instruction missing")
	}
	return normalizeBody(string(b)) + "\n"
}

// isDescriptor reports whether p is an XDocs descriptor (*.xdocs.md), which
// documents the assets in the repository but is never a shipped artifact.
func isDescriptor(p string) bool { return strings.HasSuffix(p, ".xdocs.md") }
