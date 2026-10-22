package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/CGuiho/zeze/internal/doc"
	"github.com/CGuiho/zeze/internal/jsonc"
)

// OpenCode deny rules (issue #11): created-file shape, merge preserving
// unrelated settings, comment refusal, state recording with exact removal,
// backward-compatible state files and the presence check.

func newDenyFixture(t *testing.T) (home, root string) {
	t.Helper()
	home, root = t.TempDir(), t.TempDir()
	t.Setenv("ZEZE_USER_HOME", home)
	return home, root
}

func readOpencode(t *testing.T, root string) *doc.Value {
	t.Helper()
	data, err := os.ReadFile(opencodeConfigPath(root))
	if err != nil {
		t.Fatalf("opencode settings: %v", err)
	}
	v, err := jsonc.Parse(data, false)
	if err != nil {
		t.Fatalf("opencode settings: %v", err)
	}
	return v
}

func TestOpencodeDenyRulesCreatedFileShape(t *testing.T) {
	home, root := newDenyFixture(t)
	if opencodeDenyRulesPresent(root) {
		t.Fatal("present without a settings file")
	}
	added, err := applyOpencodeDenyRules(home, root)
	if err != nil || added != len(opencodeDenyRules) {
		t.Fatalf("apply: added %d err %v", added, err)
	}
	data, err := os.ReadFile(filepath.Join(root, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "{\n  \"$schema\": \"https://opencode.ai/config.json\",\n  \"permissions\": [") {
		t.Fatalf("a created opencode.json must carry $schema first, then permissions:\n%s", data)
	}
	v := readOpencode(t, root)
	perm := v.Get("permissions")
	if perm == nil || perm.Kind != doc.Array || len(perm.A) != len(opencodeDenyRules) {
		t.Fatalf("permissions: %s", data)
	}
	for i, want := range opencodeDenyRules {
		if r, ok := ruleOf(perm.A[i]); !ok || r != want {
			t.Fatalf("rule %d = %+v, want %+v", i, r, want)
		}
	}
	if !opencodeDenyRulesPresent(root) {
		t.Fatal("presence check must pass after apply")
	}
	if n, err := applyOpencodeDenyRules(home, root); err != nil || n != 0 {
		t.Fatalf("reapply must add nothing: %d %v", n, err)
	}
}

func TestOpencodeDenyRulesMergePreservesSettings(t *testing.T) {
	home, root := newDenyFixture(t)
	if err := os.WriteFile(filepath.Join(root, "opencode.json"), []byte(
		`{"$schema": "https://opencode.ai/config.json", "model": "provider/model-x", "permissions": [{"action": "shell", "resource": "git push *", "effect": "deny"}]}`+"\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	added, err := applyOpencodeDenyRules(home, root)
	if err != nil || added != len(opencodeDenyRules) {
		t.Fatalf("apply: added %d err %v", added, err)
	}
	v := readOpencode(t, root)
	if m := v.Get("model"); m == nil || m.S != "provider/model-x" {
		t.Fatalf("unrelated key lost: %v", m)
	}
	if s := v.Get("$schema"); s == nil || s.S != "https://opencode.ai/config.json" {
		t.Fatalf("$schema lost: %v", s)
	}
	perm := v.Get("permissions")
	if perm == nil || len(perm.A) != len(opencodeDenyRules)+1 {
		t.Fatalf("permissions length: %v", perm)
	}
	// The existing entry survives and the deny rules are appended after it:
	// appended = last word at install time.
	if r, ok := ruleOf(perm.A[0]); !ok || r.Action != "shell" || r.Resource != "git push *" {
		t.Fatalf("existing entry changed: %+v", perm.A[0])
	}
	for i, want := range opencodeDenyRules {
		if r, ok := ruleOf(perm.A[i+1]); !ok || r != want {
			t.Fatalf("rule %d = %+v, want %+v", i+1, r, want)
		}
	}
}

func TestOpencodeDenyRulesCommentRefusal(t *testing.T) {
	home, root := newDenyFixture(t)
	if err := os.WriteFile(filepath.Join(root, "opencode.jsonc"), []byte("{\n  // a comment\n  \"model\": \"provider/model-x\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if opencodeConfigPath(root) != filepath.Join(root, "opencode.jsonc") {
		t.Fatal("opencode.jsonc must be the target when opencode.json is absent")
	}
	if opencodeDenyRulesPresent(root) {
		t.Fatal("a comment file must not report the rules present")
	}
	_, err := applyOpencodeDenyRules(home, root)
	if err == nil || !strings.Contains(err.Error(), "add the deny rules by hand") {
		t.Fatalf("comment file must be refused: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "opencode.json")); err == nil {
		t.Fatal("a refused apply must not create opencode.json")
	}
}

func TestOpencodeDenyRulesStateRecordingAndRemoval(t *testing.T) {
	home, root := newDenyFixture(t)
	// A hand-added identical rule is not zeze's and must survive removal.
	hand := ruleValue(opencodeDenyRules[0])
	other := ruleValue(opencodeDenyRule{Action: "shell", Resource: "git push *", Effect: "deny"})
	file := doc.NewObject()
	file.Put("$schema", doc.NewString("https://opencode.ai/config.json"))
	file.Put("permissions", doc.NewArray(hand, other))
	if err := os.WriteFile(filepath.Join(root, "opencode.json"), doc.Marshal(file), 0o644); err != nil {
		t.Fatal(err)
	}
	added, err := applyOpencodeDenyRules(home, root)
	if err != nil || added != len(opencodeDenyRules)-1 {
		t.Fatalf("apply: added %d err %v", added, err)
	}
	st := readDenyState(home)
	if rec := st.OpenCode[root]; len(rec) != len(opencodeDenyRules)-1 {
		t.Fatalf("state recorded %d rules, want only the added ones: %+v", len(rec), rec)
	}
	for _, r := range st.OpenCode[root] {
		if r == opencodeDenyRules[0] {
			t.Fatal("a pre-existing hand rule must not be recorded")
		}
	}
	removed, err := removeOpencodeDenyRules(home, root)
	if err != nil || removed != len(opencodeDenyRules)-1 {
		t.Fatalf("remove: removed %d err %v", removed, err)
	}
	perm := readOpencode(t, root).Get("permissions")
	if perm == nil || len(perm.A) != 2 {
		t.Fatalf("exactly the recorded rules must be removed: %v", perm)
	}
	if r, ok := ruleOf(perm.A[0]); !ok || r != opencodeDenyRules[0] {
		t.Fatalf("hand rule lost: %+v", perm.A[0])
	}
	if r, ok := ruleOf(perm.A[1]); !ok || r.Action != "shell" {
		t.Fatalf("unrelated rule lost: %+v", perm.A[1])
	}
	if rec := readDenyState(home).OpenCode[root]; len(rec) != 0 {
		t.Fatalf("state must drop the root: %+v", rec)
	}
	if n, err := removeOpencodeDenyRules(home, root); err != nil || n != 0 {
		t.Fatalf("second remove must be a no-op: %d %v", n, err)
	}
}

func TestDenyStateOldFileKeepsLoading(t *testing.T) {
	home, root := newDenyFixture(t)
	if err := os.MkdirAll(filepath.Join(home, "state"), 0o700); err != nil {
		t.Fatal(err)
	}
	old := `{"projects":{` + strconv.Quote(root) + `:["Read(**/*.unencrypted.json)"]}}` + "\n"
	if err := os.WriteFile(filepath.Join(home, "state", denyStateFile), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	st := readDenyState(home)
	if got := st.Projects[root]; len(got) != 1 || got[0] != "Read(**/*.unencrypted.json)" {
		t.Fatalf("the Claude projects map lost its meaning: %+v", st.Projects)
	}
	if st.OpenCode == nil || len(st.OpenCode) != 0 {
		t.Fatalf("the OpenCode map must start empty: %+v", st.OpenCode)
	}
	if _, err := applyOpencodeDenyRules(home, root); err != nil {
		t.Fatal(err)
	}
	st = readDenyState(home)
	if got := st.Projects[root]; len(got) != 1 || got[0] != "Read(**/*.unencrypted.json)" {
		t.Fatalf("the Claude recording was changed: %+v", st.Projects)
	}
	if rec := st.OpenCode[root]; len(rec) != len(opencodeDenyRules) {
		t.Fatalf("the OpenCode recording: %+v", rec)
	}
}

func TestOpencodeDenyRulesPresent(t *testing.T) {
	home, root := newDenyFixture(t)
	partial := doc.NewObject()
	partial.Put("permissions", doc.NewArray(ruleValue(opencodeDenyRules[0])))
	if err := os.WriteFile(filepath.Join(root, "opencode.json"), doc.Marshal(partial), 0o644); err != nil {
		t.Fatal(err)
	}
	if opencodeDenyRulesPresent(root) {
		t.Fatal("partial rules must not report present")
	}
	if _, err := applyOpencodeDenyRules(home, root); err != nil {
		t.Fatal(err)
	}
	if !opencodeDenyRulesPresent(root) {
		t.Fatal("all rules present must be detected")
	}
	// A permissions key of another kind is refused, never rewritten.
	bad := doc.NewObject()
	bad.Put("permissions", doc.NewObject())
	if err := os.WriteFile(filepath.Join(root, "opencode.json"), doc.Marshal(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if opencodeDenyRulesPresent(root) {
		t.Fatal("a non-array permissions must not report present")
	}
	if _, err := applyOpencodeDenyRules(home, root); err == nil || !strings.Contains(err.Error(), "permissions must be an array") {
		t.Fatalf("a non-array permissions must be refused: %v", err)
	}
}
