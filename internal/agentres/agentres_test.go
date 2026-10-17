package agentres

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const body = "## jureuma\n\nline one\nline two\n"

func block(eol string) string {
	return BeginMarker + eol + "## jureuma" + eol + eol + "line one" + eol + "line two" + eol + EndMarker
}

func TestApplyBlock(t *testing.T) {
	lf, crlf := block("\n"), block("\r\n")
	tests := []struct {
		name, in, want string
	}{
		{"empty", "", lf + "\n"},
		{"no trailing newline", "# A", "# A\n\n" + lf + "\n"},
		{"trailing newline", "# A\n", "# A\n\n" + lf + "\n"},
		{"trailing blank line", "# A\n\n", "# A\n\n" + lf + "\n"},
		{"crlf append", "# A\r\nb\r\n", "# A\r\nb\r\n\r\n" + crlf + "\r\n"},
		{"replace middle", "head\n" + BeginMarker + "\nold\n" + EndMarker + "\ntail\n", "head\n" + lf + "\ntail\n"},
		{"replace keeps outside bytes", "x  \n\n\t" + BeginMarker + "  \nold\n" + EndMarker + "\n\n  y", "x  \n\n" + lf + "\n\n  y"},
		{"replace at eof without newline", "a\n" + BeginMarker + "\nold\n" + EndMarker, "a\n" + lf},
		{"replace crlf", "h\r\n" + BeginMarker + "\r\nold\r\n" + EndMarker + "\r\nt\r\n", "h\r\n" + crlf + "\r\nt\r\n"},
		{"duplicates removed", "a\n" + BeginMarker + "\n1\n" + EndMarker + "\nb\n" + BeginMarker + "\n2\n" + EndMarker + "\nc\n", "a\n" + lf + "\nb\nc\n"},
		{"orphan begin kept", "a\n" + BeginMarker + "\nuser\n" + BeginMarker + "\nold\n" + EndMarker + "\nz\n", "a\n" + BeginMarker + "\nuser\n" + lf + "\nz\n"},
		{"orphan end kept", EndMarker + "\nx\n", EndMarker + "\nx\n\n" + lf + "\n"},
		{"marker inline is content", "see " + BeginMarker + " here\n", "see " + BeginMarker + " here\n\n" + lf + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := []byte(tt.in)
			orig := append([]byte(nil), in...)
			got := ApplyBlock(in, body)
			if string(got) != tt.want {
				t.Fatalf("ApplyBlock:\n got %q\nwant %q", got, tt.want)
			}
			if !bytes.Equal(in, orig) {
				t.Fatal("input modified")
			}
			if !HasCurrentBlock(got, body) {
				t.Fatal("HasCurrentBlock false after apply")
			}
			again := ApplyBlock(got, body)
			if !bytes.Equal(again, got) {
				t.Fatalf("not idempotent:\n%q\n%q", got, again)
			}
		})
	}
}

func TestBodyNormalization(t *testing.T) {
	a := ApplyBlock([]byte("x\n"), "\n\n## jureuma\r\n\r\nline one\r\nline two\r\n\r\n")
	b := ApplyBlock([]byte("x\n"), body)
	if !bytes.Equal(a, b) {
		t.Fatalf("bodies with different line endings render differently:\n%q\n%q", a, b)
	}
	if !HasCurrentBlock(a, "## jureuma\n\nline one\nline two") {
		t.Fatal("trailing newline should not matter")
	}
}

func TestStripFrontmatter(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"plain body", "## jureuma\n\nx\n", "## jureuma\n\nx\n"},
		{"closed", "---\nname: guiho-i-x\n---\n\n## jureuma\nx\n", "## jureuma\nx\n"},
		{"closed keeps body blank lines", "---\na: 1\n---\n\n\nbody\n\nmore\n", "body\n\nmore\n"},
		{"crlf", "---\r\nname: x\r\n---\r\n\r\nbody\r\n", "body\n"},
		{"nested metadata", "---\nname: x\nmetadata:\n  version: \"0.1.0\"\n---\nbody\n", "body\n"},
		{"unterminated", "---\nname: x\nbody\n", "---\nname: x\nbody\n"},
		{"frontmatter only", "---\nname: x\n---\n", ""},
		{"not frontmatter", "text\n---\nname: x\n---\nbody\n", "text\n---\nname: x\n---\nbody\n"},
		{"closing at eof without newline", "---\nname: x\n---", "---\nname: x\n---"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripFrontmatter(tt.in); got != tt.want {
				t.Fatalf("stripFrontmatter(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestApplyBlockStripsFrontmatter(t *testing.T) {
	with := "---\nname: guiho-i-x\nmetadata:\n  version: \"0.1.0\"\n---\n\n" + body
	got := ApplyBlock([]byte("a\n"), with)
	want := ApplyBlock([]byte("a\n"), body)
	if !bytes.Equal(got, want) {
		t.Fatalf("frontmatter body rendered differently:\n%q\n%q", got, want)
	}
	if bytes.Contains(got, []byte("---")) || bytes.Contains(got, []byte("metadata")) {
		t.Fatalf("applied block contains frontmatter:\n%q", got)
	}
	if !HasCurrentBlock(got, with) {
		t.Fatal("HasCurrentBlock must ignore a frontmatter body prefix")
	}
}

func TestRemoveBlock(t *testing.T) {
	tests := []struct {
		name, in, want string
		found          bool
	}{
		{"none", "a\nb\n", "a\nb\n", false},
		{"empty", "", "", false},
		{"middle", "head\n" + BeginMarker + "\nold\n" + EndMarker + "\ntail\n", "head\ntail\n", true},
		{"eof no newline", "a\n" + BeginMarker + "\nold\n" + EndMarker, "a\n", true},
		{"crlf middle", "h\r\n" + BeginMarker + "\r\nold\r\n" + EndMarker + "\r\nt\r\n", "h\r\nt\r\n", true},
		{"orphan begin kept", "a\n" + BeginMarker + "\nkeep\n", "a\n" + BeginMarker + "\nkeep\n", false},
		{"all duplicates", BeginMarker + "\n1\n" + EndMarker + "\nmid\n" + BeginMarker + "\n2\n" + EndMarker + "\n", "mid\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := RemoveBlock([]byte(tt.in))
			if found != tt.found || string(got) != tt.want {
				t.Fatalf("RemoveBlock = %q, %v; want %q, %v", got, found, tt.want, tt.found)
			}
			again, found2 := RemoveBlock(got)
			if found2 || !bytes.Equal(again, got) {
				t.Fatal("remove not idempotent")
			}
		})
	}
}

func TestApplyRemoveRoundTrip(t *testing.T) {
	for _, in := range []string{
		"",
		"# Title\n",
		"# Title\n\nSome text.\n",
		"# Title\r\n\r\nWindows text.\r\n",
		"   \n\t\n",
		"unicode: é ü 中文\n",
	} {
		applied := ApplyBlock([]byte(in), Instruction())
		if !bytes.HasPrefix(applied, []byte(in)) {
			t.Fatalf("apply changed bytes before the block for %q", in)
		}
		out, found := RemoveBlock(applied)
		if !found || string(out) != in {
			t.Fatalf("round trip %q -> %q", in, out)
		}
	}
}

func TestHasCurrentBlock(t *testing.T) {
	cur := ApplyBlock([]byte("a\n"), body)
	tests := []struct {
		name string
		in   []byte
		want bool
	}{
		{"current", cur, true},
		{"none", []byte("a\n"), false},
		{"other body", ApplyBlock([]byte("a\n"), "different"), false},
		{"duplicate", append(append([]byte{}, cur...), cur...), false},
		{"crlf current", ApplyBlock([]byte("a\r\n"), body), true},
		{"lf block in crlf file", []byte("a\r\n" + block("\n") + "\n"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasCurrentBlock(tt.in, body); got != tt.want {
				t.Fatalf("HasCurrentBlock = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEmbeddedArtifacts(t *testing.T) {
	skills := Skills()
	if len(skills) != 1 || skills[0].ID != MainSkillID {
		t.Fatalf("skills = %v", skills)
	}
	md := string(skills[0].Files["SKILL.md"])
	for _, want := range []string{
		"name: guiho-s-jureuma\n", "description: ",
		"## CLI Evolution and Feedback",
		"https://github.com/CGuiho/jureuma",
		"https://github.com/CGuiho/jureuma/issues/new",
		"`disabled`", "`always-ask`", "`always-proceed`",
		"jureuma upgrade check", "jureuma init", "jureuma --version",
		"## Writing the schema", "jureuma schema infer", "x-jureuma-secret",
		"--dangerously-bypass-protections", "*.unencrypted.*", "--value-stdin",
		"JUREUMA_CONFIG", "guiho-p-jureuma-install",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("SKILL.md missing %q", want)
		}
	}
	if !strings.HasPrefix(md, "---\nname: guiho-s-jureuma\n") {
		t.Error("SKILL.md frontmatter must start with name")
	}

	ids := map[string]string{}
	for _, p := range Prompts() {
		ids[p.ID] = p.Body
	}
	for _, id := range []string{MainPromptID, InstallPromptID, UninstallPromptID} {
		if strings.TrimSpace(ids[id]) == "" {
			t.Errorf("prompt %s missing", id)
		}
	}
	if len(ids) != 3 {
		t.Errorf("unexpected prompts %v", len(ids))
	}
	inst := ids[InstallPromptID]
	for _, want := range []string{
		"irm https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/install.ps1 | iex",
		"curl -fsSL https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/install.sh | sh",
		"--version", "--channel", "-Version", "-Channel", "package manager",
		"checksums.txt", "$HOME/.guiho/bin/jureuma", "chmod +x", "jureuma init",
		"https://github.com/CGuiho/jureuma/issues/new",
	} {
		if !strings.Contains(inst, want) {
			t.Errorf("install prompt missing %q", want)
		}
	}
	if _, ok := PromptByID(MainPromptID); !ok {
		t.Error("PromptByID")
	}
	if _, ok := SkillByID(MainSkillID); !ok {
		t.Error("SkillByID")
	}

	ins := Instruction()
	if strings.Contains(ins, BeginMarker) || strings.Contains(ins, EndMarker) {
		t.Error("instruction must not contain markers")
	}
	if strings.HasPrefix(ins, "---") || strings.Contains(ins, "name: guiho-i-jureuma") || strings.Contains(ins, "metadata:") {
		t.Error("instruction must not carry artifact frontmatter")
	}
	for _, want := range []string{"https://github.com/CGuiho/jureuma", "*.unencrypted.*", "guiho-s-jureuma", "guiho-p-jureuma", "--help-docs", "125"} {
		if !strings.Contains(ins, want) {
			t.Errorf("instruction missing %q", want)
		}
	}
	if strings.Contains(ins, "\r") || !strings.HasSuffix(ins, "\n") || strings.HasSuffix(ins, "\n\n") {
		t.Error("instruction not normalized")
	}
}

// fmFields parses the leading YAML frontmatter of an artifact into simple
// top-level scalars plus "metadata.version". It returns nil when the block is
// missing, unclosed, or carries no metadata version.
func fmFields(md string) map[string]string {
	lines := strings.Split(md, "\n")
	if len(lines) < 3 || lines[0] != "---" {
		return nil
	}
	out := map[string]string{}
	inMeta := false
	for _, line := range lines[1:] {
		switch {
		case line == "---":
			if out["metadata.version"] == "" {
				return nil
			}
			return out
		case line == "metadata:":
			inMeta = true
		case inMeta && strings.HasPrefix(line, `  version: "`):
			out["metadata.version"] = strings.TrimSuffix(strings.TrimPrefix(line, `  version: "`), `"`)
		case strings.HasPrefix(line, "version: "):
			out["version"] = strings.Trim(strings.TrimPrefix(line, "version: "), `"`)
		case line == "" || strings.HasPrefix(line, " "):
		default:
			if i := strings.Index(line, ": "); i > 0 {
				out[line[:i]] = line[i+2:]
			}
		}
	}
	return nil
}

var semverPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)

// TestArtifactMetadata pins GUIHO convention 0002: every shipped agent
// artifact carries name, description and a SemVer metadata.version; prompts
// and the instruction also carry purpose, created, version and owner.
func TestArtifactMetadata(t *testing.T) {
	check := func(path, id, owner, md string) {
		t.Helper()
		fm := fmFields(md)
		if fm == nil {
			t.Errorf("%s: missing or unclosed frontmatter or metadata.version", path)
			return
		}
		if fm["name"] != id {
			t.Errorf("%s: name = %q, want %q", path, fm["name"], id)
		}
		if strings.TrimSpace(fm["description"]) == "" {
			t.Errorf("%s: empty description", path)
		}
		if !semverPattern.MatchString(fm["metadata.version"]) {
			t.Errorf("%s: metadata.version = %q, not SemVer", path, fm["metadata.version"])
		}
		if owner == "" {
			return
		}
		for _, key := range []string{"purpose", "created", "version", "owner"} {
			if strings.TrimSpace(fm[key]) == "" {
				t.Errorf("%s: empty %s", path, key)
			}
		}
		if fm["owner"] != owner {
			t.Errorf("%s: owner = %q, want %q", path, fm["owner"], owner)
		}
		if fm["version"] != fm["metadata.version"] {
			t.Errorf("%s: version %q != metadata.version %q", path, fm["version"], fm["metadata.version"])
		}
	}
	s, ok := SkillByID(MainSkillID)
	if !ok {
		t.Fatal("main skill missing")
	}
	check("skills/guiho-s-jureuma/SKILL.md", MainSkillID, "", string(s.Files["SKILL.md"]))
	for _, p := range Prompts() {
		check("prompts/"+p.ID+".md", p.ID, "jureuma-prompts", p.Body)
	}
	raw, err := assets.ReadFile("assets/instruction.md")
	if err != nil {
		t.Fatal(err)
	}
	check("instruction.md", "guiho-i-jureuma", "jureuma", string(raw))
}

func TestSkillDestinations(t *testing.T) {
	home := t.TempDir()
	got := SkillDestinations(home)
	if len(got) != 1 || got[0] != filepath.Join(home, ".agents", "skills") {
		t.Fatalf("got %v", got)
	}
	os.Mkdir(filepath.Join(home, ".claude"), 0o755)
	os.Mkdir(filepath.Join(home, ".codex"), 0o755)
	got = SkillDestinations(home)
	want := []string{
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".agents", "skills"),
		filepath.Join(home, ".codex", "skills"),
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestInstallSkill(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "skills")
	s := Skill{ID: "guiho-s-jureuma", Files: map[string][]byte{"SKILL.md": []byte("v1"), "ref/a.md": []byte("a")}}
	if SkillInstalledCurrent(s, dest) {
		t.Fatal("current before install")
	}
	if err := InstallSkill(s, dest); err != nil {
		t.Fatal(err)
	}
	if !SkillInstalledCurrent(s, dest) {
		t.Fatal("not current after install")
	}
	// Stale extra file and changed content are replaced.
	os.WriteFile(filepath.Join(dest, s.ID, "stale.md"), []byte("x"), 0o644)
	if SkillInstalledCurrent(s, dest) {
		t.Fatal("extra file should make it not current")
	}
	s2 := Skill{ID: s.ID, Files: map[string][]byte{"SKILL.md": []byte("v2")}}
	if err := InstallSkill(s2, dest); err != nil {
		t.Fatal(err)
	}
	if !SkillInstalledCurrent(s2, dest) || SkillInstalledCurrent(s, dest) {
		t.Fatal("upgrade not applied")
	}
	if _, err := os.Stat(filepath.Join(dest, s.ID, "stale.md")); !os.IsNotExist(err) {
		t.Fatal("stale file survived")
	}
	entries, _ := os.ReadDir(dest)
	if len(entries) != 1 {
		t.Fatalf("leftover temp entries: %v", entries)
	}
	// Sibling skills are untouched.
	other := filepath.Join(dest, "other-skill")
	os.Mkdir(other, 0o755)
	os.WriteFile(filepath.Join(other, "SKILL.md"), []byte("o"), 0o644)
	if err := InstallSkill(s, dest); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(other, "SKILL.md")); string(b) != "o" {
		t.Fatal("sibling modified")
	}
	// Real embedded skill installs.
	main, _ := SkillByID(MainSkillID)
	if err := InstallSkill(main, dest); err != nil || !SkillInstalledCurrent(main, dest) {
		t.Fatalf("embedded skill: %v", err)
	}
}

func TestInstallSkillRejects(t *testing.T) {
	dest := t.TempDir()
	for _, s := range []Skill{
		{ID: "", Files: nil},
		{ID: "..", Files: nil},
		{ID: "a/b", Files: nil},
		{ID: ".hidden", Files: nil},
		{ID: "ok", Files: map[string][]byte{"../x": nil}},
		{ID: "ok", Files: map[string][]byte{"/abs": nil}},
		{ID: "ok", Files: map[string][]byte{"a/../b": nil}},
	} {
		if err := InstallSkill(s, dest); err == nil {
			t.Errorf("InstallSkill(%q) accepted", s.ID)
		}
	}
	// A non-directory or symlink at the target is not ours.
	os.WriteFile(filepath.Join(dest, "file-skill"), []byte("x"), 0o644)
	if err := InstallSkill(Skill{ID: "file-skill", Files: map[string][]byte{"SKILL.md": nil}}, dest); err == nil {
		t.Error("replaced a file")
	}
	real := t.TempDir()
	if err := os.Symlink(real, filepath.Join(dest, "link-skill")); err == nil {
		if err := InstallSkill(Skill{ID: "link-skill", Files: map[string][]byte{"SKILL.md": nil}}, dest); err == nil {
			t.Error("replaced a symlink")
		}
	}
}

func TestDescriptorsAreNotShipped(t *testing.T) {
	for _, s := range Skills() {
		for name := range s.Files {
			if strings.HasSuffix(name, ".xdocs.md") {
				t.Errorf("skill %s ships descriptor %s", s.ID, name)
			}
		}
	}
	for _, p := range Prompts() {
		if strings.HasSuffix(p.ID, ".xdocs") {
			t.Errorf("descriptor listed as prompt %s", p.ID)
		}
	}
}
