package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Two pattern-valid, obviously synthetic key IDs (public metadata, safe in
// tests). No real key material appears anywhere in this file.
const (
	bindKeyA = "k_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	bindKeyB = "k_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func writeProjectFile(t *testing.T, root, content string) string {
	t.Helper()
	path := filepath.Join(root, ProjectFile)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readProjectFile(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ProjectFile))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestBindProjectKeyAppendsOneLine(t *testing.T) {
	root := t.TempDir()
	const before = `# yaml-language-server: $schema=https://example.invalid/zeze.schema.json
# the project configuration

pairs:
  - name: config   # the main pair
run:
  format: json
`
	writeProjectFile(t, root, before)
	changed, err := BindProjectKey(root, bindKeyA)
	if err != nil || !changed {
		t.Fatalf("bind: changed=%v err=%v", changed, err)
	}
	got := readProjectFile(t, root)
	if !strings.HasPrefix(got, before) {
		t.Fatalf("everything before the appended line must stay byte-identical:\n%s", got)
	}
	if !strings.HasSuffix(got, "key: "+bindKeyA+"\n") {
		t.Fatalf("appended line:\n%s", got)
	}
	p, err := ParseProject([]byte(got))
	if err != nil || p.Key != bindKeyA {
		t.Fatalf("re-parse: %v %+v", err, p)
	}
}

func TestBindProjectKeyReplacesOnlyTheValue(t *testing.T) {
	root := t.TempDir()
	const before = `pairs: []

key: ` + bindKeyB + `   # the project key

agent:
  evolution:
    upgrade: always-ask
`
	writeProjectFile(t, root, before)
	changed, err := BindProjectKey(root, bindKeyA)
	if err != nil || !changed {
		t.Fatalf("bind: changed=%v err=%v", changed, err)
	}
	want := strings.Replace(before, "key: "+bindKeyB+"   #", "key: "+bindKeyA+"   #", 1)
	if got := readProjectFile(t, root); got != want {
		t.Fatalf("only the value span may change:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestBindProjectKeyAlreadyEqualWritesNothing(t *testing.T) {
	root := t.TempDir()
	// Quoted value and unusual spacing: an equal binding is never rewritten.
	const before = "pairs: []\nkey:    \"" + bindKeyA + "\"   # kept\n"
	writeProjectFile(t, root, before)
	changed, err := BindProjectKey(root, bindKeyA)
	if err != nil || changed {
		t.Fatalf("bind: changed=%v err=%v", changed, err)
	}
	if got := readProjectFile(t, root); got != before {
		t.Fatalf("file rewritten although already equal:\n%s", got)
	}
}

func TestBindProjectKeyRefusesWithoutWriting(t *testing.T) {
	cases := []struct{ name, in string }{
		{"unparsable", "run: [\n"},
		{"not a mapping", "- " + bindKeyA + "\n"},
		{"invalid document", "key: a-label-not-a-key-id\n"},
		{"quoted key", "\"key\": " + bindKeyA + "\n"},
		{"flow mapping", "{key: " + bindKeyA + ", pairs: []}\n"},
		{"aliased value", "pairs:\n  - name: &a config\nkey: *a\n"},
		{"value not on the key line", "key: \"k_aaa\n  aaaa\"\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := writeProjectFile(t, root, tc.in)
			if _, err := BindProjectKey(root, bindKeyB); err == nil {
				t.Fatal("want refusal")
			}
			if got := readProjectFile(t, root); got != tc.in {
				t.Fatalf("refused but the file changed:\n%s", got)
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBindProjectKeyMissingFile(t *testing.T) {
	root := t.TempDir()
	if _, err := BindProjectKey(root, bindKeyA); err == nil {
		t.Fatal("want refusal for a missing file")
	}
	if _, err := os.Stat(filepath.Join(root, ProjectFile)); !os.IsNotExist(err) {
		t.Fatal("a missing file must stay missing")
	}
}

func TestBindProjectKeyKeepsLineEndings(t *testing.T) {
	root := t.TempDir()
	before := "pairs: []\r\n# a windows-shaped file\r\n"
	writeProjectFile(t, root, before)
	if changed, err := BindProjectKey(root, bindKeyA); err != nil || !changed {
		t.Fatalf("bind: changed=%v err=%v", changed, err)
	}
	if got := readProjectFile(t, root); got != before+"key: "+bindKeyA+"\r\n" {
		t.Fatalf("line endings not preserved:\n%q", got)
	}
}

func TestBindProjectKeyRejectsInvalidKeyID(t *testing.T) {
	root := t.TempDir()
	before := "pairs: []\n"
	writeProjectFile(t, root, before)
	for _, id := range []string{"", "a-label", bindKeyA[:len(bindKeyA)-1], "k_" + strings.Repeat("A", 32)} {
		if _, err := BindProjectKey(root, id); err == nil {
			t.Errorf("%q: want refusal", id)
		}
	}
	if got := readProjectFile(t, root); got != before {
		t.Fatalf("file changed:\n%s", got)
	}
}

// The binding survives a SaveProject round trip, so a later whole-document
// save can never drop it silently.
func TestSaveProjectKeepsKeyBinding(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	p := &Project{Key: bindKeyA, Pairs: []PairConfig{{Name: "config"}}}
	if err := SaveProject(root, p, "0.1.0"); err != nil {
		t.Fatal(err)
	}
	data := readProjectFile(t, root)
	if !strings.Contains(data, "key: "+bindKeyA+"\n") {
		t.Fatalf("binding not written:\n%s", data)
	}
	got, _, err := LoadProject(root)
	if err != nil || got.Key != bindKeyA {
		t.Fatalf("round trip: %v %+v", err, got)
	}
}
