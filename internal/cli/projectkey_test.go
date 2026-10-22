package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/CGuiho/zeze/internal/config"
	"github.com/CGuiho/zeze/internal/xerr"
)

// A pattern-valid, obviously synthetic key ID (public metadata). No key
// material appears anywhere in this file.
const boundKeyOther = "k_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

// initProject runs a non-interactive init and returns the key ID it created.
func initProject(t *testing.T, h *harness) string {
	t.Helper()
	h.expect(0, "", "init", "--agent-deny-rules", "yes")
	ids := storedKeyIDs(t, h)
	if len(ids) != 1 {
		t.Fatalf("want one stored key, have %v", ids)
	}
	return ids[0]
}

// storedKeyIDs lists the active stored key IDs in the harness home.
func storedKeyIDs(t *testing.T, h *harness) []string {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(h.home, ".guiho", "zeze", "keys", "k_*.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, f := range m {
		ids = append(ids, strings.TrimSuffix(filepath.Base(f), ".json"))
	}
	sort.Strings(ids)
	return ids
}

// pairKeyID reads the $zeze.keyId a pair file records.
func pairKeyID(t *testing.T, h *harness, file string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(h.proj, file))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Meta struct {
			KeyID string `json:"keyId"`
		} `json:"$zeze"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Meta.KeyID
}

// removeFile deletes a file inside the harness project.
func removeFile(t *testing.T, h *harness, name string) {
	t.Helper()
	if err := os.Remove(filepath.Join(h.proj, name)); err != nil {
		t.Fatal(err)
	}
}

func projectKey(t *testing.T, h *harness) string {
	t.Helper()
	p, _, err := config.LoadProject(h.proj)
	if err != nil {
		t.Fatal(err)
	}
	return p.Key
}

func projectFile(t *testing.T, h *harness) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(h.proj, config.ProjectFile))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A binding that disagrees with the recorded pair identity is a key error
// naming the binding and the file — on every pair-resolving command, never a
// silent re-encrypt.
func TestProjectKeyBindingDisagreementIsKeyError(t *testing.T) {
	h := newHarness(t)
	keyID := initProject(t, h)
	h.write("config.unencrypted.json", `{"db":{"url":"bound-value"}}`)
	h.expect(0, "", "encrypt", "--keep-both")
	// Bind a different key than the pair records.
	h.write("zeze.yaml", "key: "+boundKeyOther+"\npairs:\n  - name: config\n")
	for _, args := range [][]string{
		{"list"},
		{"set", "/x", "1"},
		{"unset", "/db/url"},
		{"status"},
		{"check"},
		{"schema", "infer"},
	} {
		code, out := h.run("", args...)
		if code != xerr.CodeKey {
			t.Fatalf("%v: exit %d, want %d\n%s", args, code, xerr.CodeKey, out)
		}
		for _, want := range []string{"zeze.yaml", boundKeyOther, keyID} {
			if !strings.Contains(out, want) {
				t.Fatalf("%v: message misses %q:\n%s", args, want, out)
			}
		}
	}
	// run maps every zeze-side failure to 125 with the same key message.
	code, out := h.run("", "run", "--", "true")
	if code != 125 || !strings.HasPrefix(out, "[ZEZE:KEY]") || !strings.Contains(out, boundKeyOther) {
		t.Fatalf("run: exit %d\n%s", code, out)
	}
	// Nothing was re-encrypted to the bound key.
	if got := pairKeyID(t, h, "config.encrypted.json"); got != keyID {
		t.Fatalf("pair re-keyed: %s", got)
	}
}

// A matching binding works, and a pair with no recorded identity — a
// headerless plaintext alone, or no files at all — is never called a
// disagreement.
func TestProjectKeyBindingAgreementAndNoIdentity(t *testing.T) {
	h := newHarness(t)
	keyID := initProject(t, h)
	h.write("config.unencrypted.json", `{"db":{"url":"agree-value"}}`)
	h.expect(0, "", "encrypt", "--keep-both")
	h.write("zeze.yaml", "# the project key\nkey: "+keyID+"\npairs:\n  - name: config\n")
	h.expect(0, "", "set", "/n", "--json", "1")
	h.expect(0, "", "list")
	// A headerless plaintext with no encrypted file records no identity: the
	// binding never treats that as a disagreement. (What follows is the
	// single-file repair pointer, not a key error.)
	removeFile(t, h, "config.encrypted.json")
	h.write("config.unencrypted.json", `{"db":{"url":"agree-value"}}`)
	h.write("zeze.yaml", "key: "+boundKeyOther+"\npairs:\n  - name: config\n")
	code, out := h.run("", "set", "/x", "--json", "1")
	if code != 1 || !strings.Contains(out, "run zeze init to repair") {
		t.Fatalf("headerless plaintext: exit %d\n%s", code, out)
	}
	if strings.Contains(out, "key bound") || strings.Contains(out, boundKeyOther) {
		t.Fatalf("a headerless plaintext must not be a binding disagreement:\n%s", out)
	}
	// No files at all: init creates the pair with the bound key and the
	// absence of an identity is not a disagreement either.
	removeFile(t, h, "config.unencrypted.json")
	h.write("zeze.yaml", "key: "+keyID+"\npairs:\n  - name: config\n")
	h.expect(0, "", "init", "--agent-deny-rules", "yes")
	if got := pairKeyID(t, h, "config.encrypted.json"); got != keyID {
		t.Fatalf("pair key %s, want the bound key %s", got, keyID)
	}
}

// When zeze.yaml binds a stored key, init creates a pair with that key and
// leaves the file — and its comments — byte-identical (an equal binding is
// never rewritten).
func TestProjectKeyBindingInitUsesBoundKey(t *testing.T) {
	h := newHarness(t)
	keyID := initProject(t, h)
	removeFile(t, h, "config.encrypted.json")
	removeFile(t, h, "config.unencrypted.json")
	yml := "# project configuration\n\nkey: " + keyID + "   # the pair key\n\npairs:\n  - name: config   # the only pair\n"
	h.write("zeze.yaml", yml)
	h.expect(0, "", "init", "--agent-deny-rules", "yes")
	if got := projectFile(t, h); got != yml {
		t.Fatalf("init rewrote %s:\n%s", config.ProjectFile, got)
	}
	if got := pairKeyID(t, h, "config.encrypted.json"); got != keyID {
		t.Fatalf("pair key %s, want the bound key %s", got, keyID)
	}
	if ids := storedKeyIDs(t, h); len(ids) != 1 {
		t.Fatalf("init created another key: %v", ids)
	}
}

// A pair with no recorded identity gets its key created or chosen; init then
// binds that key in zeze.yaml with a one-line splice that keeps every
// comment byte-identical.
func TestProjectKeyBindingInitWritesBinding(t *testing.T) {
	h := newHarness(t)
	const head = "# yaml-language-server: $schema=example.invalid\n# keep this comment\n\npairs:\n  - name: config   # the only pair\nrun:\n  format: json\n"
	h.write("zeze.yaml", head)
	h.expect(0, "", "init", "--agent-deny-rules", "yes")
	keyID := pairKeyID(t, h, "config.encrypted.json")
	got := projectFile(t, h)
	if !strings.HasPrefix(got, head) || !strings.HasSuffix(got, "key: "+keyID+"\n") {
		t.Fatalf("binding not spliced in without touching comments:\n%s", got)
	}
	if projectKey(t, h) != keyID {
		t.Fatalf("binding %q, want %s", projectKey(t, h), keyID)
	}
	// The same through an explicitly chosen stored key.
	removeFile(t, h, "config.encrypted.json")
	removeFile(t, h, "config.unencrypted.json")
	h.write("zeze.yaml", head)
	h.expect(0, "", "init", "--agent-deny-rules", "yes", "--key-id", keyID)
	got = projectFile(t, h)
	if !strings.HasPrefix(got, head) || !strings.HasSuffix(got, "key: "+keyID+"\n") {
		t.Fatalf("--key-id binding not spliced in:\n%s", got)
	}
	if ids := storedKeyIDs(t, h); len(ids) != 1 {
		t.Fatalf("--key-id created another key: %v", ids)
	}
}

// --key-id / --key-label that contradict the binding are a usage error.
func TestProjectKeyBindingFlagContradiction(t *testing.T) {
	h := newHarness(t)
	keyID := initProject(t, h)
	code, out := h.run("", "init", "--agent-deny-rules", "yes", "--key-id", boundKeyOther)
	if code != xerr.CodeUsage || !strings.Contains(out, "contradicts") {
		t.Fatalf("contradicting --key-id: exit %d\n%s", code, out)
	}
	// The same key as the binding is accepted.
	h.expect(0, "", "init", "--agent-deny-rules", "yes", "--key-id", keyID)
	// A stored key contradicting a binding that names an unstored key.
	h.write("zeze.yaml", "key: "+boundKeyOther+"\npairs:\n  - name: config\n")
	code, out = h.run("", "init", "--agent-deny-rules", "yes", "--key-id", keyID)
	if code != xerr.CodeUsage || !strings.Contains(out, config.ProjectFile) {
		t.Fatalf("contradicting stored key: exit %d\n%s", code, out)
	}
}

// A label or a malformed value in key: is a validation error (exit 4).
func TestProjectKeyBindingMalformedIsValidation(t *testing.T) {
	h := newHarness(t)
	initProject(t, h)
	for _, val := range []string{"my-label", "\"\"", "k_" + strings.Repeat("A", 32)} {
		h.write("zeze.yaml", "key: "+val+"\npairs:\n  - name: config\n")
		code, out := h.run("", "list")
		if code != xerr.CodeValidation {
			t.Fatalf("key: %s -> exit %d, want %d\n%s", val, code, xerr.CodeValidation, out)
		}
		if strings.Contains(out, val) {
			t.Fatalf("message leaks the rejected value:\n%s", out)
		}
	}
}

// key rotate updates an existing binding to the new key ID and the pair keeps
// running; an absent binding stays absent.
func TestProjectKeyBindingRotateUpdates(t *testing.T) {
	h := newHarness(t)
	keyID := initProject(t, h)
	h.write("config.unencrypted.json", `{"db":{"url":"rotate-binding-value"}}`)
	h.expect(0, "", "encrypt", "--keep-both")
	out := h.expect(0, "", "key", "rotate")
	if !strings.Contains(out, "updated the "+config.ProjectFile+" key binding") {
		t.Fatalf("rotate did not report the binding update:\n%s", out)
	}
	newKey := pairKeyID(t, h, "config.encrypted.json")
	if newKey == keyID {
		t.Fatal("rotate did not change the pair key")
	}
	if got := projectKey(t, h); got != newKey {
		t.Fatalf("binding %q, want the new key %s", got, newKey)
	}
	if strings.Contains(projectFile(t, h), keyID) {
		t.Fatalf("binding still names the old key:\n%s", projectFile(t, h))
	}
	h.expect(0, "", "set", "/n", "--json", "2")
	h.expect(0, "", "run", "--", "true")
	// Without a binding, rotation leaves zeze.yaml alone.
	removeFile(t, h, "config.unencrypted.json")
	h.expect(0, "", "decrypt", "--keep-both", "--dangerously-bypass-protections")
	h.write("zeze.yaml", "pairs:\n  - name: config\n")
	before := projectFile(t, h)
	h.expect(0, "", "key", "rotate")
	if got := projectFile(t, h); got != before {
		t.Fatalf("rotation added a binding that was absent:\n%s", got)
	}
}

// The ensure step never overwrites a binding that differs from the key it
// just created or chose: it is a key error instead.
func TestEnsureKeyBindingNeverOverwrites(t *testing.T) {
	h := newHarness(t)
	keyID := initProject(t, h)
	e, err := loadEnv()
	if err != nil {
		t.Fatal(err)
	}
	p, err := e.resolve(&pairFlags{})
	if err != nil {
		t.Fatal(err)
	}
	r := &initReport{cmd: nil}
	e.proj.Key = boundKeyOther
	err = ensureKeyBinding(r, e, p, keyID)
	if err == nil || xerr.CodeOf(err) != xerr.CodeKey {
		t.Fatalf("want exit %d, got %v", xerr.CodeKey, err)
	}
	if got := projectKey(t, h); got != keyID {
		t.Fatalf("binding overwritten: %q", got)
	}
}
