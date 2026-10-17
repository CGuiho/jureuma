package e2e

import (
	"strings"
	"testing"
)

// A pattern-valid, obviously synthetic key ID (public metadata). No key
// material appears anywhere in this file.
const boundKeyOther = "k_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

// init with a created or explicitly chosen key binds it in jureuma.yaml with a
// one-line splice that keeps every comment byte-identical; an already-equal
// binding is never rewritten.
func TestProjectKeyBindingOnInit(t *testing.T) {
	p := newProject(t)
	const head = "# project configuration\n# keep this comment\n\npairs:\n  - name: config   # the only pair\n"
	p.write("jureuma.yaml", head)
	p.init()
	keyID, _ := p.privateKey()
	got := p.read("jureuma.yaml")
	if !strings.HasPrefix(got, head) || !strings.HasSuffix(got, "key: "+keyID+"\n") {
		t.Fatalf("binding not spliced in without touching comments:\n%s", got)
	}
	// Already equal: a second init leaves the file byte-identical.
	p.init()
	if again := p.read("jureuma.yaml"); again != got {
		t.Fatalf("init rewrote an equal binding:\n%s", again)
	}
	// The same through an explicitly chosen stored key.
	p.remove("config.encrypted.json")
	p.remove("config.unencrypted.json")
	p.write("jureuma.yaml", head)
	p.init("--key-id", keyID)
	got = p.read("jureuma.yaml")
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
	p.write("jureuma.yaml", "key: "+boundKeyOther+"\npairs:\n  - name: config\n")
	r := p.run("set", "/svc/url", "x")
	if r.code != 5 || !strings.Contains(r.stderr, boundKeyOther) || !strings.Contains(r.stderr, keyID) {
		t.Fatalf("mismatched binding: want exit 5 naming both keys\n%s", r)
	}
	// run maps the same key error to 125 with its kind prefix.
	r = p.run("run", "--", "true")
	if r.code != 125 || !strings.HasPrefix(r.stderr, "[JUREUMA:KEY]") || !strings.Contains(r.stderr, boundKeyOther) {
		t.Fatalf("mismatched binding on run: want 125 [JUREUMA:KEY]\n%s", r)
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
	p.write("jureuma.yaml", "key: "+boundKeyOther+"\npairs:\n  - name: config\n")
	r := p.run("set", "/b", "--json", "1")
	if r.code != 1 || !strings.Contains(r.stderr, "run jureuma init to repair") {
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
	if !strings.Contains(r.stdout, "updated the jureuma.yaml key binding") {
		t.Fatalf("rotate did not report the binding update:\n%s", r)
	}
	newID := p.encMeta("config.encrypted.json")["keyId"].(string)
	if newID == oldID {
		t.Fatal("rotate did not change the pair key")
	}
	if yml := p.read("jureuma.yaml"); !strings.Contains(yml, "key: "+newID) || strings.Contains(yml, oldID) {
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
		p.write("jureuma.yaml", "key: "+val+"\npairs:\n  - name: config\n")
		for _, args := range [][]string{{"set", "/a", "x"}, {"init", "--agent-deny-rules", "no"}, {"check"}} {
			r := p.run(args...)
			if r.code != 4 {
				t.Fatalf("key: %s %v: want exit 4\n%s", val, args, r)
			}
			assertNo(t, "malformed binding error", r.out(), "malformed-bind-value")
		}
	}
}
