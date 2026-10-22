package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A pattern-valid, obviously synthetic key ID (public metadata). No key
// material appears anywhere in this file.
const boundKeyOther = "k_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

// init with a created or explicitly chosen key binds it in zeze.yaml with a
// one-line splice that keeps every comment byte-identical; an already-equal
// binding is never rewritten.
func TestProjectKeyBindingOnInit(t *testing.T) {
	p := newProject(t)
	const head = "# project configuration\n# keep this comment\n\npairs:\n  - name: config   # the only pair\n"
	p.write("zeze.yaml", head)
	p.init()
	keyID, _ := p.privateKey()
	got := p.read("zeze.yaml")
	if !strings.HasPrefix(got, head) || !strings.HasSuffix(got, "key: "+keyID+"\n") {
		t.Fatalf("binding not spliced in without touching comments:\n%s", got)
	}
	// Already equal: a second init leaves the file byte-identical.
	p.init()
	if again := p.read("zeze.yaml"); again != got {
		t.Fatalf("init rewrote an equal binding:\n%s", again)
	}
	// The same through an explicitly chosen stored key.
	p.remove("config.encrypted.json")
	p.remove("config.unencrypted.json")
	p.write("zeze.yaml", head)
	p.init("--key-id", keyID)
	got = p.read("zeze.yaml")
	if !strings.HasPrefix(got, head) || !strings.HasSuffix(got, "key: "+keyID+"\n") {
		t.Fatalf("--key-id binding not spliced in:\n%s", got)
	}
	if got := p.encMeta("config.encrypted.json")["keyId"]; got != keyID {
		t.Fatalf("pair key %v, want the bound key %s", got, keyID)
	}
	if n := len(p.keyFiles()); n != 1 {
		t.Fatalf("--key-id created another key: %d key files", n)
	}
}

// A binding that disagrees with the recorded pair identity fails loudly and
// never re-encrypts anything to the bound key.
func TestProjectKeyBindingMismatchFailsLoudly(t *testing.T) {
	p := newProject(t)
	p.setup(`{"svc":{"url":"mismatch-bind-value"}}`)
	keyID, _ := p.privateKey()
	p.write("zeze.yaml", "key: "+boundKeyOther+"\npairs:\n  - name: config\n")
	r := p.run("set", "/svc/url", "x")
	if r.code != 5 || !strings.Contains(r.stderr, boundKeyOther) || !strings.Contains(r.stderr, keyID) {
		t.Fatalf("mismatched binding: want exit 5 naming both keys\n%s", r)
	}
	// run maps the same key error to 125 with its kind prefix.
	r = p.run("run", "--", "true")
	if r.code != 125 || !strings.HasPrefix(r.stderr, "[ZEZE:KEY]") || !strings.Contains(r.stderr, boundKeyOther) {
		t.Fatalf("mismatched binding on run: want 125 [ZEZE:KEY]\n%s", r)
	}
	if got := p.encMeta("config.encrypted.json")["keyId"]; got != keyID {
		t.Fatalf("pair silently re-keyed: %v", got)
	}
	assertNo(t, "mismatch errors", r.out(), "mismatch-bind-value")
}

// A pair with no recorded identity is not a disagreement: a headerless
// plaintext alone keeps its single-file repair pointer, never a key error.
func TestProjectKeyBindingNoIdentityIsNotDisagreement(t *testing.T) {
	p := newProject(t)
	p.setup(`{"a":"identity-free-value"}`)
	p.remove("config.encrypted.json")
	p.write("config.unencrypted.json", `{"a":"identity-free-value"}`)
	p.write("zeze.yaml", "key: "+boundKeyOther+"\npairs:\n  - name: config\n")
	r := p.run("set", "/b", "--json", "1")
	if r.code != 1 || !strings.Contains(r.stderr, "run zeze init to repair") {
		t.Fatalf("headerless plaintext: want the repair pointer, exit 1\n%s", r)
	}
	if strings.Contains(r.out(), boundKeyOther) {
		t.Fatalf("a headerless plaintext must not be a binding disagreement:\n%s", r)
	}
}

// key rotate updates an existing binding to the new key ID and the pair keeps
// working; an absent binding stays absent.
func TestProjectKeyBindingFollowsRotation(t *testing.T) {
	p := newProject(t)
	p.setup(`{"svc":{"url":"rotate-bind-value"}}`)
	oldID, _ := p.privateKey()
	r := p.expect(0, "key", "rotate")
	if !strings.Contains(r.stdout, "updated the zeze.yaml key binding") {
		t.Fatalf("rotate did not report the binding update:\n%s", r)
	}
	newID := p.encMeta("config.encrypted.json")["keyId"].(string)
	if newID == oldID {
		t.Fatal("rotate did not change the pair key")
	}
	if yml := p.read("zeze.yaml"); !strings.Contains(yml, "key: "+newID) || strings.Contains(yml, oldID) {
		t.Fatalf("binding not updated to %s:\n%s", newID, yml)
	}
	p.expect(0, "set", "/n", "--json", "2")
	p.expect(0, "check")
}

// A label or a malformed value in key: is a validation error (exit 4).
func TestProjectKeyBindingMalformedRejected(t *testing.T) {
	p := newProject(t)
	p.setup(`{"a":"malformed-bind-value"}`)
	for _, val := range []string{"my-label", `""`, "k_" + strings.Repeat("A", 32), "[k_x]"} {
		p.write("zeze.yaml", "key: "+val+"\npairs:\n  - name: config\n")
		for _, args := range [][]string{{"set", "/a", "x"}, {"init", "--agent-deny-rules", "no"}, {"check"}} {
			r := p.run(args...)
			if r.code != 4 {
				t.Fatalf("key: %s %v: want exit 4\n%s", val, args, r)
			}
			assertNo(t, "malformed binding error", r.out(), "malformed-bind-value")
		}
	}
}

// With a binding, key rotate is project-wide: every configured pair that
// records the bound key is rotated to ONE new key, every recorded identity
// follows it, the binding is updated once, and the old key is retired with no
// ciphertext left under it.
func TestProjectKeyBindingRotateRotatesWholeProject(t *testing.T) {
	needSh(t)
	p := newProject(t)
	p.write("zeze.yaml", "pairs:\n  - name: config\n  - name: production\n")
	p.init()
	keyID, _ := p.privateKey()
	if n := len(p.keyFiles()); n != 1 {
		t.Fatalf("want one project key, have %d", n)
	}
	p.expect(0, "set", "/svc", "--json", `{"url":"rotate-project-config"}`)
	p.expect(0, "set", "--name", "production", "/mode", "rotate-project-prod")
	r := p.expect(0, "key", "rotate")
	for _, want := range []string{
		"rotated config.encrypted.json: " + keyID,
		"rotated production.encrypted.json: " + keyID,
		"project key rotated: " + keyID,
		"updated the zeze.yaml key binding: " + keyID,
	} {
		if !strings.Contains(r.stdout, want) {
			t.Fatalf("rotate output misses %q:\n%s", want, r.stdout)
		}
	}
	newID := p.encMeta("config.encrypted.json")["keyId"].(string)
	if newID == keyID {
		t.Fatal("rotate did not change the key")
	}
	if got := p.encMeta("production.encrypted.json")["keyId"]; got != newID {
		t.Fatalf("production pair key %v, want the same new key %s", got, newID)
	}
	// Every recorded identity followed the new key, including the headered
	// unencrypted siblings, and nothing references the old key any more.
	for _, f := range []string{"config.encrypted.json", "production.encrypted.json", "config.unencrypted.json", "production.unencrypted.json"} {
		if strings.Contains(p.read(f), keyID) {
			t.Fatalf("%s still references the old key:\n%s", f, p.read(f))
		}
	}
	for _, f := range []string{"config.unencrypted.json", "production.unencrypted.json"} {
		if !strings.Contains(p.read(f), `"keyId": "`+newID+`"`) {
			t.Fatalf("%s identity did not follow the rotation:\n%s", f, p.read(f))
		}
	}
	// The binding is updated exactly once and still parses.
	if yml := p.read("zeze.yaml"); strings.Count(yml, "key: ") != 1 || !strings.Contains(yml, "key: "+newID+"\n") {
		t.Fatalf("binding not updated once to %s:\n%s", newID, yml)
	}
	// The old key is retired (moved, not left active).
	if _, err := os.Stat(filepath.Join(p.jhome(), "keys", keyID+".json")); !os.IsNotExist(err) {
		t.Fatalf("the old key is still active: %v", err)
	}
	if _, err := os.Stat(filepath.Join(p.jhome(), "keys", "retired", keyID+".json")); err != nil {
		t.Fatalf("the old key was not retired: %v", err)
	}
	// run, check and set pass for both pairs.
	p.expect(0, "run", "--", "sh", "-c", `test -n "$ZEZE_CONFIG"`)
	p.expect(0, "check")
	p.expect(0, "set", "/n", "--json", "1")
	p.expect(0, "set", "--name", "production", "/n", "--json", "1")
	if got := p.revealConfig(); !strings.Contains(got, "rotate-project-config") {
		t.Fatalf("values lost in rotation: %s", got)
	}
	if got := p.revealConfig("--name", "production"); !strings.Contains(got, "rotate-project-prod") {
		t.Fatalf("production values lost in rotation: %s", got)
	}
}

// A pair recording a different key than the binding refuses the whole project
// rotation before anything is written.
func TestProjectKeyBindingRotateRefusesMixedKeys(t *testing.T) {
	p := newProject(t)
	p.write("zeze.yaml", "pairs:\n  - name: config\n")
	p.init()
	keyA, _ := p.privateKey()
	// A second pair gets its own key: with pairs that disagree, init binds
	// nothing.
	p.write("zeze.yaml", "pairs:\n  - name: config\n  - name: other\n")
	p.init()
	files, _ := filepath.Glob(filepath.Join(p.jhome(), "keys", "k_*.json"))
	if len(files) != 2 {
		t.Fatalf("want two keys (one per pair), have %v", files)
	}
	keyB := p.encMeta("other.encrypted.json")["keyId"].(string)
	if keyB == keyA {
		t.Fatal("the second pair should carry its own key")
	}
	p.write("zeze.yaml", "key: "+keyA+"\npairs:\n  - name: config\n  - name: other\n")
	before := map[string]string{
		"zeze.yaml":          p.read("zeze.yaml"),
		"config.encrypted.json": p.read("config.encrypted.json"),
		"other.encrypted.json":  p.read("other.encrypted.json"),
	}
	r := p.run("key", "rotate")
	if r.code != 5 {
		t.Fatalf("mixed keys: want exit 5\n%s", r)
	}
	for _, want := range []string{"other", keyA, keyB} {
		if !strings.Contains(r.stderr, want) {
			t.Fatalf("refusal misses %q:\n%s", want, r.stderr)
		}
	}
	for f, old := range before {
		if got := p.read(f); got != old {
			t.Fatalf("%s was written although the rotation was refused:\n%s", f, got)
		}
	}
	if n := len(p.keyFiles()); n != 2 {
		t.Fatalf("the refused rotation created a key: %d active keys", n)
	}
}

// status and check never turn a missing key ID into a key error: with no
// recorded identity, status reports the state and exits 0, and check keeps
// its exit-1 repair-pointer failure.
func TestStatusAndCheckWithoutRecordedKey(t *testing.T) {
	p := newProject(t)
	p.setup(`{"a":"no-identity-value"}`)
	p.remove("config.encrypted.json")
	p.write("config.unencrypted.json", `{"a":"no-identity-value"}`)
	r := p.expect(0, "status")
	if !strings.Contains(r.out(), "no $zeze identity") {
		t.Fatalf("status must report the missing identity:\n%s", r)
	}
	r = p.run("check")
	if r.code != 1 || !strings.Contains(r.out(), "run zeze init to repair") {
		t.Fatalf("check: want exit 1 with the repair pointer\n%s", r)
	}
	if strings.Contains(r.out(), "invalid key ID") {
		t.Fatalf("check must not look up an empty key ID:\n%s", r)
	}
}

// A configured pair that records no key has nothing to rotate: it is skipped
// with a note while the rest of the project rotates.
func TestProjectKeyBindingRotateSkipsIdentitylessPair(t *testing.T) {
	p := newProject(t)
	p.write("zeze.yaml", "pairs:\n  - name: config\n")
	p.init()
	keyID, _ := p.privateKey()
	p.write("zeze.yaml", "key: "+keyID+"\npairs:\n  - name: config\n  - name: empty\n")
	r := p.expect(0, "key", "rotate")
	if !strings.Contains(r.stderr, "note: pair empty records no key; skipped") {
		t.Fatalf("the identityless pair must be skipped with a note:\n%s", r)
	}
	newID := p.encMeta("config.encrypted.json")["keyId"].(string)
	if newID == keyID || !strings.Contains(p.read("zeze.yaml"), "key: "+newID) {
		t.Fatalf("the bound pair was not rotated and re-bound:\n%s", p.read("zeze.yaml"))
	}
}
