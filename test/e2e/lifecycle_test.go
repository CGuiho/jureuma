package e2e

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// snapshot hashes every regular file under root (skipping .git).
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.Type().IsRegular() {
			return nil
		}
		// The update cache, locks and run registry are disposable state.
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if strings.Contains(rel, "/locks/") || strings.Contains(rel, "/run/") || strings.HasSuffix(rel, "cache.json") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		out[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func diffSnapshots(a, b map[string]string) []string {
	var d []string
	for k, v := range a {
		if b[k] != v {
			d = append(d, k)
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			d = append(d, k+" (new)")
		}
	}
	sort.Strings(d)
	return d
}

// bodyWithoutMeta returns the body members of a top-level JSON document
// written by zeze: the reserved $zeze and $schema members are dropped,
// every other member is returned byte for byte. Assertions compare the body
// because decrypt and the mirror now write the pair's identity metadata
// beside it.
func bodyWithoutMeta(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return s
	}
	var out []string
	for _, m := range splitMembers(s[1 : len(s)-1]) {
		t := strings.TrimLeft(m, " \t\r\n")
		if strings.HasPrefix(t, `"$zeze"`) || strings.HasPrefix(t, `"$schema"`) {
			continue
		}
		out = append(out, m)
	}
	return "{" + strings.Join(out, ",") + "}"
}

// splitMembers splits the members of a JSON object body at top-level commas.
func splitMembers(s string) []string {
	var out []string
	depth, start := 0, 0
	inStr, esc := false, false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case esc:
			esc = false
		case inStr && c == '\\':
			esc = true
		case inStr && c == '"':
			inStr = false
		case inStr:
		case c == '"':
			inStr = true
		case c == '{' || c == '[':
			depth++
		case c == '}' || c == ']':
			depth--
		case c == ',' && depth == 0:
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

func TestInitCreatesPairAndIsIdempotent(t *testing.T) {
	p := newProject(t)
	p.init()
	for _, f := range []string{"config.encrypted.json", "config.unencrypted.json", "config.schema.json", ".gitignore", "zeze.yaml", "AGENTS.md"} {
		if !p.exists(f) {
			t.Fatalf("init did not create %s", f)
		}
	}
	if n := len(p.keyFiles()); n != 1 {
		t.Fatalf("init generated %d keys, want 1", n)
	}
	if fi, err := os.Stat(p.keyFiles()[0]); err != nil || (fi.Mode().Perm()&0o077) != 0 {
		t.Fatalf("key file mode is too wide: %v", fi.Mode())
	}
	if fi, _ := os.Stat(p.path("config.unencrypted.json")); fi.Mode().Perm()&0o077 != 0 {
		t.Fatalf("unencrypted file mode is too wide: %v", fi.Mode())
	}
	meta := p.encMeta("config.encrypted.json")
	if meta["keyId"] == "" || !strings.HasPrefix(meta["publicKey"].(string), "zeze-pk-1:") {
		t.Fatalf("bad $zeze block: %v", meta)
	}

	projBefore := snapshot(t, p.dir)
	homeBefore := snapshot(t, p.home)
	r := p.init()
	if strings.Contains(r.stdout, "generated") {
		t.Fatalf("second init generated a key:\n%s", r)
	}
	if d := diffSnapshots(projBefore, snapshot(t, p.dir)); len(d) > 0 {
		t.Fatalf("second init changed project files: %v", d)
	}
	homeAfter := snapshot(t, p.home)
	var changed []string
	for _, f := range diffSnapshots(homeBefore, homeAfter) {
		// Only the audit log may grow; keys, configuration, state and skills stay.
		if !strings.HasSuffix(f, "audit.log") && !strings.HasSuffix(f, "audit.log (new)") {
			changed = append(changed, f)
		}
	}
	if len(changed) > 0 {
		t.Fatalf("second init changed home files: %v", changed)
	}
	if n := strings.Count(p.read(".gitignore"), "# >>> zeze managed block >>>"); n != 1 {
		t.Fatalf(".gitignore has %d managed blocks", n)
	}

	// init on an existing encrypted file without its key never replaces the key.
	restore := p.hideKeys()
	before := p.read("config.encrypted.json")
	r = p.init()
	if !strings.Contains(r.out(), "private key not available") {
		t.Fatalf("init without the key should report it as informational:\n%s", r)
	}
	if p.read("config.encrypted.json") != before {
		t.Fatal("init modified $zeze without the key")
	}
	if _, err := os.Stat(filepath.Join(p.jhome(), "keys")); err == nil {
		if files := p.keyFiles(); len(files) > 0 {
			t.Fatalf("init generated a key for an existing encrypted file: %v", files)
		}
		os.RemoveAll(filepath.Join(p.jhome(), "keys"))
	}
	restore()
}

func TestEncryptDecryptRoundTripKeepsOrderAndNumberText(t *testing.T) {
	needSh(t)
	p := newProject(t)
	const plain = `{"zeta":1,"alpha":{"b":1.0,"a":1e2,"c":-0.50},"num":100,"big":12345678901234567890.000,"arr":[3,"x",true,null,{}],"empty":[],"esc":"aé\"b","last":"Zz9"}`
	p.setup(plain)

	enc := p.read("config.encrypted.json")
	for _, v := range []string{"12345678901234567890", "Zz9"} {
		if strings.Contains(enc, v) {
			t.Fatalf("encrypted file contains plaintext %q", v)
		}
	}
	// $zeze first, then keys in source order.
	order := []string{`"$zeze"`, `"zeta"`, `"alpha"`, `"b"`, `"a"`, `"c"`, `"num"`, `"big"`, `"arr"`, `"empty"`, `"esc"`, `"last"`}
	last := -1
	body := enc
	for _, k := range order {
		i := strings.Index(body[last+1:], k)
		if i < 0 {
			t.Fatalf("encrypted file: key %s missing or out of order\n%s", k, enc)
		}
		last += 1 + i
	}
	if !strings.Contains(enc, `"empty": []`) || !strings.Contains(enc, "{}") {
		t.Fatalf("empty containers must stay as {} and []:\n%s", enc)
	}

	// encrypt again with no change: ciphertexts are reused.
	r := p.expect(0, "encrypt", "--keep-both")
	if p.read("config.encrypted.json") != enc {
		t.Fatalf("re-encrypting unchanged values changed the file:\n%s", r)
	}

	// The child sees the exact document text.
	if got := p.revealConfig(); got != `{"zeta":1,"alpha":{"b":1.0,"a":1e2,"c":-0.50},"num":100,"big":12345678901234567890.000,"arr":[3,"x",true,null,{}],"empty":[],"esc":"aé\"b","last":"Zz9"}` {
		t.Fatalf("ZEZE_CONFIG = %s", got)
	}

	// decrypt recreates the plaintext with the same order and number text,
	// beside the pair's $zeze identity header and $schema pointer.
	p.remove("config.unencrypted.json")
	p.expect(0, "decrypt", "--keep-both", "--dangerously-bypass-protections")
	dec := p.read("config.unencrypted.json")
	want := `{"zeta":1,"alpha":{"b":1.0,"a":1e2,"c":-0.50},"num":100,"big":12345678901234567890.000,"arr":[3,"x",true,null,{}],"empty":[],"esc":"aé\"b","last":"Zz9"}`
	if stripWS(bodyWithoutMeta(dec)) != want && stripWS(bodyWithoutMeta(dec)) != stripWS(plain) {
		t.Fatalf("decrypted document differs:\n%s", dec)
	}
	for _, meta := range []string{`"$zeze"`, `"$schema": "./config.schema.json"`} {
		if !strings.Contains(dec, meta) {
			t.Fatalf("decrypted document lacks %s:\n%s", meta, dec)
		}
	}
	if fi, _ := os.Stat(p.path("config.unencrypted.json")); fi.Mode().Perm()&0o077 != 0 {
		t.Fatalf("decrypted file mode is too wide: %v", fi.Mode())
	}
	if o, _ := p.statusJSON(); o != "in-sync" {
		t.Fatalf("status after decrypt: %s", o)
	}
}

func TestJSONCInput(t *testing.T) {
	needSh(t)
	p := newProject(t)
	p.init()
	p.remove("config.unencrypted.json")
	p.write("config.unencrypted.jsonc", `// leading comment
{
  /* block
     comment */
  "db": {
    "url": "jsonc-url-value", // trailing comment
    "pool": 5,
  },
  "tags": ["a", "b",],
}
`)
	p.expect(0, "encrypt", "--keep-both")
	enc := p.read("config.encrypted.json")
	if strings.Contains(enc, "comment") || strings.Contains(enc, "jsonc-url-value") {
		t.Fatalf("comments or values reached the encrypted file:\n%s", enc)
	}
	if got := p.revealConfig(); got != `{"db":{"url":"jsonc-url-value","pool":5},"tags":["a","b"]}` {
		t.Fatalf("ZEZE_CONFIG = %s", got)
	}
	if o, _ := p.statusJSON(); o != "in-sync" {
		t.Fatalf("status: %s", o)
	}
	// zeze writes .jsonc in place: decrypt and set keep every comment
	// and the formatting outside the values they change. decrypt also writes
	// the pair's $zeze identity header and $schema pointer, inserted before
	// the closing brace like any other new member.
	jsoncBefore := p.read("config.unencrypted.jsonc")
	p.expect(0, "decrypt", "--keep-both", "--dangerously-bypass-protections")
	got := p.read("config.unencrypted.jsonc")
	for _, want := range []string{"// leading comment", "/* block\n     comment */", `"url": "jsonc-url-value", // trailing comment`, `"tags": ["a", "b",]`} {
		if !strings.Contains(got, want) {
			t.Fatalf("decrypt must reproduce the same document without touching comments:\ngot:\n%s\nwant:\n%s", got, jsoncBefore)
		}
	}
	for _, meta := range []string{`"$zeze"`, `"$schema"`} {
		if !strings.Contains(got, meta) {
			t.Fatalf("decrypt must write the identity header and $schema pointer into the .jsonc file:\n%s", got)
		}
	}
	p.expect(0, "set", "/db/url", "changed-by-set")
	got = p.read("config.unencrypted.jsonc")
	for _, want := range []string{"// leading comment", "/* block\n     comment */", `"url": "changed-by-set", // trailing comment`} {
		if !strings.Contains(got, want) {
			t.Fatalf("set must preserve comments and formatting:\n%s", got)
		}
	}
	if strings.Contains(got, "jsonc-url-value") {
		t.Fatalf("set must replace the old value:\n%s", got)
	}
	if p.exists("config.unencrypted.json") {
		t.Fatal("set created a .json plaintext beside the .jsonc one")
	}
	if o, _ := p.statusJSON(); o != "in-sync" {
		t.Fatalf("status after set on a .jsonc pair: %s", o)
	}
	p.expect(0, "check")
	// decrypt restores the encrypted values but keeps the comments of
	// surviving keys and every comment outside the written spans.
	p.write("config.unencrypted.jsonc", `// leading comment
{
  /* block
     comment */
  "db": {
    "url": "human-edit", // trailing comment
    "pool": 5,
  },
  "tags": ["a", "b",],
  // human note
}
`)
	p.expect(0, "decrypt", "--keep-both", "--force", "--dangerously-bypass-protections")
	got = p.read("config.unencrypted.jsonc")
	for _, want := range []string{"// leading comment", "/* block\n     comment */", "// trailing comment", "// human note", `"url": "changed-by-set"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("decrypt must preserve comments:\n%s", got)
		}
	}
	if strings.Contains(got, "human-edit") {
		t.Fatalf("decrypt must restore the encrypted value:\n%s", got)
	}
	if o, _ := p.statusJSON(); o != "in-sync" {
		t.Fatalf("status after decrypt on a .jsonc pair: %s", o)
	}
	// Both .json and .jsonc present is a usage error for every command.
	p.write("config.unencrypted.json", `{}`)
	for _, args := range [][]string{{"status"}, {"encrypt", "--keep-both"}, {"list"}} {
		if r := p.run(args...); r.code != 2 {
			t.Fatalf("both plaintext variants present: want exit 2\n%s", r)
		}
	}
	// Strict .json input rejects comments, with line/column only.
	p.remove("config.unencrypted.jsonc")
	p.write("config.unencrypted.json", "{\n  \"a\": \"strict-secret-text\", // no\n}\n")
	r := p.run("encrypt", "--keep-both")
	if r.code == 0 || !strings.Contains(r.stderr, "line 2") {
		t.Fatalf("comment in .json must fail with a position:\n%s", r)
	}
	assertNo(t, "encrypt parse error", r.out(), "strict-secret-text", "no\n")
}

func TestGitignoreInvariant(t *testing.T) {
	p := newProject(t)
	p.setup(`{"a":"gitignore-value-1"}`)
	for _, f := range []string{"config.unencrypted.json", "x.unencrypted.jsonc", "deep/dir/y.unencrypted.json", "prod.unencrypted.json"} {
		if !p.gitOK("check-ignore", "-q", f) {
			t.Fatalf("%s is not ignored by git", f)
		}
	}

	// Without the block, the next write restores it.
	p.write(".gitignore", "node_modules/\n")
	p.remove("config.unencrypted.json")
	p.expect(0, "decrypt", "--keep-both", "--dangerously-bypass-protections")
	if !strings.Contains(p.read(".gitignore"), "*.unencrypted.json") || !strings.Contains(p.read(".gitignore"), "node_modules/") {
		t.Fatalf("block not restored (or user lines lost):\n%s", p.read(".gitignore"))
	}
	if !p.gitOK("check-ignore", "-q", "config.unencrypted.json") {
		t.Fatal("plaintext not ignored after restore")
	}

	// A negation rule after the block makes the file not ignorable: refuse to write.
	p.write(".gitignore", p.read(".gitignore")+"!config.unencrypted.json\n")
	plainBefore := p.read("config.unencrypted.json")
	r0 := p.expect(0, "set", "/b", "mirror-blocked-value")
	if p.read("config.unencrypted.json") != plainBefore || !strings.Contains(r0.stderr, "warning") {
		t.Fatalf("set mirrored into a plaintext file git does not ignore:\n%s", r0)
	}
	assertNo(t, "set warning", r0.out(), "mirror-blocked-value", "gitignore-value-1")
	p.remove("config.unencrypted.json")
	r := p.run("decrypt", "--keep-both", "--dangerously-bypass-protections")
	if r.code != 1 || p.exists("config.unencrypted.json") || !strings.Contains(r.stderr, "git does not ignore") {
		t.Fatalf("decrypt wrote a plaintext file git would not ignore:\n%s", r)
	}
	assertNo(t, "refusal", r.out(), "gitignore-value-1")
	p.write(".gitignore", strings.Replace(p.read(".gitignore"), "!config.unencrypted.json\n", "", 1))

	// A negation in a nested .gitignore is caught too.
	p.write("sub/.gitignore", "!*.unencrypted.json\n")
	p.write("zeze.yaml", "pairs:\n  - name: config\n  - name: sub/app\n")
	r = p.run("init", "--agent-deny-rules", "no", "--agent-evolution", "always-ask")
	if r.code != 1 || !strings.Contains(r.stderr, "git does not ignore") {
		t.Fatalf("init must refuse a plaintext file git does not ignore:\n%s", r)
	}
	assertNo(t, "init refusal", r.out(), "gitignore-value-1")
	if p.exists("sub/app.unencrypted.json") {
		t.Fatalf("init wrote sub/app.unencrypted.json although git does not ignore it:\n%s", r)
	}
}

func TestNamedPairsAndFileFlags(t *testing.T) {
	needSh(t)
	p := newProject(t)
	p.write("zeze.yaml", "pairs:\n  - name: config\n  - name: production\n")
	p.init()
	if !p.exists("production.encrypted.json") || !p.exists("production.schema.json") || !p.exists("config.encrypted.json") {
		t.Fatal("init did not create every configured pair")
	}
	// One project, one key: the first pair's key becomes the zeze.yaml key
	// binding and every configured pair uses it.
	if n := len(p.keyFiles()); n != 1 {
		t.Fatalf("want one project key, have %d", n)
	}
	keyID, _ := p.privateKey()
	if got := p.encMeta("production.encrypted.json")["keyId"]; got != keyID {
		t.Fatalf("production pair key %v, want the project key %s", got, keyID)
	}
	if !strings.Contains(p.read("zeze.yaml"), "key: "+keyID) {
		t.Fatalf("init did not bind the project key:\n%s", p.read("zeze.yaml"))
	}
	p.expect(0, "set", "--name", "production", "/mode", "prod-mode")
	p.expect(0, "set", "/mode", "dev-mode")
	if got := p.revealConfig("--name", "production"); got != `{"mode":"prod-mode"}` {
		t.Fatalf("production pair: %s", got)
	}
	// Explicit file paths must carry the state marker.
	for _, args := range [][]string{
		{"status", "--encrypted-file", "secrets.json"},
		{"status", "--unencrypted-file", "plain.json"},
	} {
		if r := p.run(args...); r.code != 2 {
			t.Fatalf("names without .encrypted./.unencrypted. must be refused with exit 2\n%s", r)
		}
	}
	p.expect(0, "status", "--encrypted-file", "production.encrypted.json")
}

func TestSchemaFileDisagreementOnMutatingCommand(t *testing.T) {
	p := newProject(t)
	p.init()
	p.write("zeze.yaml", "pairs:\n  - name: config\n    schema: ./config.schema.json\n")
	p.write("loose.schema.json", `{"type":"object"}`)
	if r := p.run("set", "/a", "x", "--schema-file", "loose.schema.json"); r.code != 2 {
		t.Fatalf("--schema-file disagreeing with zeze.yaml on set: want exit 2\n%s", r)
	}
	p.expect(0, "status", "--schema-file", "loose.schema.json")
}

func TestJSONOutputIsParseable(t *testing.T) {
	p := newProject(t)
	p.setup(`{"a":{"b":"json-output-value"},"n":3}`)
	r := p.expect(0, "list", "--output", "json")
	var l map[string]any
	if err := json.Unmarshal([]byte(r.stdout), &l); err != nil {
		t.Fatalf("list --output json: %v\n%s", err, r)
	}
	assertNo(t, "list json", r.out(), "json-output-value")
	if !strings.Contains(r.stdout, `"A_B"`) || !strings.Contains(r.stdout, `"/n"`) {
		t.Fatalf("list json lacks paths or env names:\n%s", r)
	}
}

// metaSpans returns the exact text of the $zeze and $schema members of a
// zeze-written document, so tests can pin that pair metadata is preserved
// byte for byte.
func metaSpans(s string) string {
	s = strings.TrimSpace(s)
	var out []string
	for _, m := range splitMembers(s[1 : len(s)-1]) {
		if t := strings.TrimLeft(m, " \t\r\n"); strings.HasPrefix(t, `"$zeze"`) || strings.HasPrefix(t, `"$schema"`) {
			out = append(out, m)
		}
	}
	return strings.Join(out, ",")
}

// The only-unencrypted state is first class: set and unset build (or refresh)
// the encrypted sibling from the plaintext's $zeze identity header without
// the private key, and decrypt returns the pair to the single-file state.
// This is the W3 brief's manual proof walk-through.
func TestOnlyUnencryptedSetRebuildsSibling(t *testing.T) {
	needSh(t)
	p := newProject(t)
	p.init()
	p.write("config.unencrypted.json", `{"db":{"url":"first-value","pool":5},"name":"svc"}`)
	p.expect(0, "encrypt")
	if p.exists("config.unencrypted.json") {
		t.Fatal("encrypt must delete the unencrypted file")
	}
	p.expect(0, "decrypt", "--dangerously-bypass-protections")
	if p.exists("config.encrypted.json") {
		t.Fatal("decrypt must delete the encrypted file")
	}

	// Keyless rebuild: the plaintext header names the pair key.
	restore := p.hideKeys()
	p.expect(0, "set", "/db/host", "host-value")
	if !p.exists("config.encrypted.json") {
		t.Fatal("set must rebuild the encrypted sibling from the plaintext")
	}
	restore()
	if got := p.revealConfig(); got != `{"db":{"url":"first-value","pool":5,"host":"host-value"},"name":"svc"}` {
		t.Fatalf("rebuilt sibling: %s", got)
	}
	if !strings.Contains(p.read("config.unencrypted.json"), "host-value") {
		t.Fatal("set must mirror its value into the plaintext")
	}
	// A keyless rebuild writes no h tags: status never claims in-sync for it.
	restore = p.hideKeys()
	if o, _ := p.statusJSON(); o != "key-unavailable" {
		t.Fatalf("keyless rebuild: status is %s, want key-unavailable", o)
	}
	restore()
	if o, _ := p.statusJSON(); o != "in-sync" {
		t.Fatalf("keyed status after the rebuild: %s", o)
	}

	// unset works from the single-file state too.
	restore = p.hideKeys()
	p.expect(0, "unset", "/db/host")
	restore()
	if got := p.revealConfig(); got != `{"db":{"url":"first-value","pool":5},"name":"svc"}` {
		t.Fatalf("after keyless unset: %s", got)
	}

	// decrypt returns to the only-unencrypted state.
	p.expect(0, "decrypt", "--dangerously-bypass-protections")
	if p.exists("config.encrypted.json") {
		t.Fatal("decrypt must delete the encrypted file again")
	}
	if o, _ := p.statusJSON(); o != "encrypted-missing" {
		t.Fatalf("only-unencrypted status: %s", o)
	}
	r := p.expect(0, "check")
	assertNo(t, "check", r.out(), "first-value", "host-value")
}

// set and unset never touch pair metadata: the $zeze identity header and
// the $schema pointer survive byte-identically, in .json and .jsonc.
func TestSetUnsetKeepIdentityHeaderAndSchemaPointer(t *testing.T) {
	p := newProject(t)
	p.init()
	p.write("config.unencrypted.json", `{"a":"keep-1","b":"keep-2"}`)
	p.expect(0, "encrypt", "--keep-both")
	p.expect(0, "decrypt", "--keep-both", "--dangerously-bypass-protections")
	before := metaSpans(p.read("config.unencrypted.json"))
	if before == "" {
		t.Fatal("decrypt must write the identity header and $schema pointer")
	}
	p.expect(0, "set", "/a", "changed-1")
	p.expect(0, "unset", "/b")
	if got := metaSpans(p.read("config.unencrypted.json")); got != before {
		t.Fatalf("set/unset rewrote the pair metadata:\nbefore %q\nafter  %q", before, got)
	}

	// The same holds in a .jsonc plaintext, where the metadata spans are
	// spliced around the comments.
	p.remove("config.unencrypted.json")
	p.write("config.unencrypted.jsonc", `{"a":"keep-1", // human note
}`)
	p.expect(0, "decrypt", "--keep-both", "--force", "--dangerously-bypass-protections")
	before = metaSpans(p.read("config.unencrypted.jsonc"))
	p.expect(0, "set", "/a", "changed-2")
	if got := metaSpans(p.read("config.unencrypted.jsonc")); got != before {
		t.Fatalf("set rewrote the .jsonc pair metadata:\nbefore %q\nafter  %q", before, got)
	}
	if !strings.Contains(p.read("config.unencrypted.jsonc"), "// human note") {
		t.Fatal("set must keep the comments of the .jsonc plaintext")
	}
}

// init repairs a hand-made legacy pair: the format-1 $zeze block becomes a
// format-2 identity header (values, sync and pending kept), the schema copy is
// embedded, and the plaintext gains its identity header and $schema pointer —
// keylessly — while a second init changes nothing.
func TestInitRepairsLegacyPair(t *testing.T) {
	p := newProject(t)
	p.init()
	p.write("config.unencrypted.json", `{"legacy":"legacy-value"}`)
	p.expect(0, "encrypt", "--keep-both")

	// Hand-make the pair into the pre-format-2 shape: format 1, no home, the
	// plain-string schema path and no embedded copy.
	var m map[string]any
	if err := json.Unmarshal([]byte(p.read("config.encrypted.json")), &m); err != nil {
		t.Fatal(err)
	}
	meta := m["$zeze"].(map[string]any)
	m["$zeze"] = map[string]any{
		"format":    1,
		"publicKey": meta["publicKey"],
		"keyId":     meta["keyId"],
		"schema":    "./config.schema.json",
		"sync": map[string]any{
			"/legacy": map[string]any{"c": strings.Repeat("a", 32), "h": strings.Repeat("b", 32)},
		},
		"pending": []any{"/legacy"},
	}
	data, _ := json.MarshalIndent(m, "", "  ")
	p.write("config.encrypted.json", string(data))
	p.write("config.unencrypted.json", `{"legacy":"legacy-value"}`)

	// Reads keep working before repair; mutations point at init.
	p.expect(0, "status")
	p.expect(0, "list")
	r := p.run("set", "/x", "y")
	if r.code != 1 || !strings.Contains(r.stderr, "run zeze init to repair") {
		t.Fatalf("mutating a legacy pair must point at init:\n%s", r)
	}

	// The repair is keyless.
	restore := p.hideKeys()
	p.init()
	restore()

	meta = p.encMeta("config.encrypted.json")
	if meta["format"] != float64(2) || meta["home"] != "~/.guiho/zeze/" {
		t.Fatalf("legacy header not repaired: %v", meta)
	}
	sc, _ := meta["schema"].(map[string]any)
	if sc == nil || sc["path"] != "./config.schema.json" || sc["c"] == nil {
		t.Fatalf("repair must embed the schema copy: %v", meta["schema"])
	}
	sync, _ := meta["sync"].(map[string]any)
	if _, ok := sync["/legacy"]; !ok {
		t.Fatalf("repair must keep the sync state: %v", meta["sync"])
	}
	if pend, _ := meta["pending"].([]any); len(pend) != 1 || pend[0] != "/legacy" {
		t.Fatalf("repair must keep the pending state: %v", meta["pending"])
	}
	plain := p.read("config.unencrypted.json")
	for _, want := range []string{`"$zeze"`, `"$schema": "./config.schema.json"`, "legacy-value"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("repair must keep the body and add the identity metadata:\n%s", plain)
		}
	}

	// Byte-idempotent on an already-repaired pair.
	projBefore := snapshot(t, p.dir)
	p.init()
	if d := diffSnapshots(projBefore, snapshot(t, p.dir)); len(d) > 0 {
		t.Fatalf("second init changed project files: %v", d)
	}

	// Mutations work again.
	p.expect(0, "set", "/added", "added-value")
	if got := p.revealConfig(); got != `{"legacy":"legacy-value","added":"added-value"}` {
		t.Fatalf("after repair: %s", got)
	}
}

// init chooses the key for a pair with no recorded identity: --key-id and
// --key-label name a stored key without a terminal, anything else creates a
// new key labeled with the repository name.
func TestInitKeySelection(t *testing.T) {
	p := newProject(t)
	p.init()
	keyID, _ := p.privateKey()
	if n := len(p.keyFiles()); n != 1 {
		t.Fatalf("want one key after init, have %d", n)
	}
	// New keys carry the default repository-name label.
	data, err := os.ReadFile(p.keyFiles()[0])
	if err != nil {
		t.Fatal(err)
	}
	var kf struct {
		Label string `json:"label"`
	}
	if err := json.Unmarshal(data, &kf); err != nil {
		t.Fatal(err)
	}
	if kf.Label != filepath.Base(p.dir) {
		t.Fatalf("new key label %q, want the project directory name", kf.Label)
	}

	// --key-label selects the stored key that carries the label.
	p.write("zeze.yaml", "pairs:\n  - name: config\n  - name: other\n")
	p.expect(0, "init", "--key-label", kf.Label, "--agent-deny-rules", "no", "--agent-evolution", "always-ask")
	if n := len(p.keyFiles()); n != 1 {
		t.Fatalf("--key-label must not create a key: have %d", n)
	}
	if got := p.encMeta("other.encrypted.json")["keyId"]; got != keyID {
		t.Fatalf("--key-label selected %v", got)
	}

	// --key-id selects it by key ID (full or short).
	p.write("zeze.yaml", "pairs:\n  - name: config\n  - name: other\n  - name: third\n")
	p.expect(0, "init", "--key-id", keyID, "--agent-deny-rules", "no", "--agent-evolution", "always-ask")
	if n := len(p.keyFiles()); n != 1 {
		t.Fatalf("--key-id must not create a key: have %d", n)
	}
	if got := p.encMeta("third.encrypted.json")["keyId"]; got != keyID {
		t.Fatalf("--key-id selected %v", got)
	}

	// Default (non-TTY): a new key for a pair with no recorded identity.
	p.write("zeze.yaml", "pairs:\n  - name: config\n  - name: other\n  - name: third\n  - name: fourth\n")
	p.init()
	if n := len(p.keyFiles()); n != 2 {
		t.Fatalf("want one key per new pair, have %d", n)
	}
	if got := p.encMeta("fourth.encrypted.json")["keyId"]; got == keyID {
		t.Fatal("a pair with no recorded identity must get its own key by default")
	}

	// A label several keys carry is ambiguous; --key-id resolves it.
	p.write("zeze.yaml", "pairs:\n  - name: config\n  - name: other\n  - name: third\n  - name: fourth\n  - name: fifth\n")
	r := p.run("init", "--key-label", kf.Label, "--agent-deny-rules", "no", "--agent-evolution", "always-ask")
	if r.code != 2 || !strings.Contains(r.stderr, "--key-id") {
		t.Fatalf("an ambiguous label must be refused with a --key-id hint:\n%s", r)
	}

	// An unknown key is a key error, not a silent new key.
	r = p.run("init", "--key-id", "k_"+strings.Repeat("0", 32), "--agent-deny-rules", "no", "--agent-evolution", "always-ask")
	if r.code != 5 {
		t.Fatalf("unknown --key-id: want exit 5\n%s", r)
	}
	if p.exists("fifth.encrypted.json") || p.exists("fifth.schema.json") {
		t.Fatal("the refused init wrote pair files")
	}
}

// The $schema pointer and the embedded schema copy survive every flow: they
// are never listed or injected as values, encrypt embeds the copy, decrypt
// restores a missing schema from it and never clobbers an existing one.
func TestSchemaPointerAndEmbeddedCopy(t *testing.T) {
	needSh(t)
	p := newProject(t)
	p.init()
	// A hand-made plaintext in the editor shape: it carries its $schema
	// pointer, which is reserved metadata, never a value.
	p.write("config.unencrypted.json", `{"$schema":"./config.schema.json","db":{"url":"pointer-value"}}`)
	p.expect(0, "encrypt", "--keep-both")
	meta := p.encMeta("config.encrypted.json")
	sc, _ := meta["schema"].(map[string]any)
	if sc == nil || sc["path"] != "./config.schema.json" || sc["c"] == nil {
		t.Fatalf("encrypt must embed the schema copy: %v", meta["schema"])
	}
	if r := p.expect(0, "list", "--output", "json"); strings.Contains(r.stdout, "$schema") {
		t.Fatalf("list must not show the $schema pointer:\n%s", r)
	}
	if got := p.revealConfig(); got != `{"db":{"url":"pointer-value"}}` {
		t.Fatalf("the $schema pointer must not be injected: %s", got)
	}

	// decrypt restores a missing schema from the embedded copy ...
	p.remove("config.schema.json")
	p.expect(0, "decrypt", "--keep-both", "--dangerously-bypass-protections")
	if !p.exists("config.schema.json") {
		t.Fatal("decrypt must restore the schema from the embedded copy")
	}
	if !strings.Contains(p.read("config.schema.json"), "draft/2020-12") {
		t.Fatalf("restored schema: %s", p.read("config.schema.json"))
	}
	// ... and never clobbers an existing schema, warning when it differs.
	p.write("config.schema.json", `{"type":"object","x-local":"kept"}`)
	r := p.expect(0, "decrypt", "--keep-both", "--force", "--dangerously-bypass-protections")
	if got := p.read("config.schema.json"); got != `{"type":"object","x-local":"kept"}` {
		t.Fatalf("decrypt must keep the local schema: %s", got)
	}
	if !strings.Contains(r.stderr, "differs from the embedded schema copy") {
		t.Fatalf("drift between the copy and the local schema must be reported:\n%s", r)
	}

	// init repairs a missing $schema pointer.
	p.write("config.unencrypted.json", `{"db":{"url":"pointer-value"}}`)
	p.init()
	if !strings.Contains(p.read("config.unencrypted.json"), `"$schema": "./config.schema.json"`) {
		t.Fatalf("init must repair the $schema pointer:\n%s", p.read("config.unencrypted.json"))
	}
}

// schema infer declares the optional $schema string property in generated
// schemas and refreshes the copy embedded in the encrypted file.
func TestSchemaInferSchemaPropertyAndCopyRefresh(t *testing.T) {
	p := newProject(t)
	p.setup(`{"port":8080}`)
	p.expect(0, "schema", "infer")
	raw := p.read("config.schema.json")
	var sch map[string]any
	if err := json.Unmarshal([]byte(raw), &sch); err != nil {
		t.Fatal(err)
	}
	props, _ := sch["properties"].(map[string]any)
	sp, _ := props["$schema"].(map[string]any)
	if sp == nil || sp["type"] != "string" {
		t.Fatalf("the generated schema must declare the optional $schema property: %s", raw)
	}
	if req, _ := sch["required"].([]any); len(req) == 0 {
		t.Fatalf("the generated schema must keep required for real keys: %s", raw)
	} else {
		for _, x := range req {
			if x == "$schema" {
				t.Fatalf("$schema must stay optional: %s", raw)
			}
		}
	}
	// The embedded copy is refreshed to the schema just written: restoring it
	// yields exactly that document.
	p.remove("config.schema.json")
	p.expect(0, "decrypt", "--keep-both", "--dangerously-bypass-protections")
	if got := p.read("config.schema.json"); got != raw {
		t.Fatalf("restored schema differs:\ngot:\n%s\nwant:\n%s", got, raw)
	}
}

// The mirror keeps protecting unsaved human edits in the single-file state:
// a value the plaintext holds and zeze cannot prove unchanged is never
// replaced, and the path stays pending.
func TestSetMirrorStillProtectsUnsavedEdits(t *testing.T) {
	needSh(t)
	p := newProject(t)
	p.init()
	p.write("config.unencrypted.json", `{"a":"human-value","b":"other"}`)
	p.expect(0, "encrypt", "--keep-both")
	p.expect(0, "decrypt", "--keep-both", "--dangerously-bypass-protections")

	// The human edits /a after decrypt: an unsaved edit.
	dec := p.read("config.unencrypted.json")
	p.write("config.unencrypted.json", strings.Replace(dec, `"human-value"`, `"human-edited"`, 1))

	// Keyless: a value at the target that cannot be checked blocks the mirror.
	restore := p.hideKeys()
	r := p.expect(0, "set", "/a", "agent-value")
	restore()
	if !strings.Contains(r.stderr, "cannot be checked") {
		t.Fatalf("keyless set must not replace an unchecked plaintext value:\n%s", r)
	}
	if !strings.Contains(p.read("config.unencrypted.json"), "human-edited") {
		t.Fatal("the human edit must survive the keyless set")
	}

	// Keyed: an edit that differs from the last saved state blocks it too.
	p.write("config.unencrypted.json", strings.Replace(p.read("config.unencrypted.json"), `"human-edited"`, `"human-edit-2"`, 1))
	r = p.expect(0, "set", "/a", "agent-value-2")
	if !strings.Contains(r.stderr, "unsaved edit") {
		t.Fatalf("keyed set must not replace an unsaved human edit:\n%s", r)
	}
	if !strings.Contains(p.read("config.unencrypted.json"), "human-edit-2") {
		t.Fatal("the human edit must survive the keyed set")
	}
	if pend, _ := p.encMeta("config.encrypted.json")["pending"].([]any); len(pend) != 1 || pend[0] != "/a" {
		t.Fatalf("a skipped mirror leaves the path pending: %v", pend)
	}

	// A set at a path the human did not touch mirrors normally.
	p.expect(0, "set", "/c", "agent-c")
	if !strings.Contains(p.read("config.unencrypted.json"), "agent-c") {
		t.Fatal("set must mirror into the plaintext when nothing blocks it")
	}
}

// status and check cover every pair state with value-free output: the
// single-file states, both files present (the riskiest state) and legacy
// zeze state.
func TestFourStatesReporting(t *testing.T) {
	p := newProject(t)
	p.init()
	p.write("config.unencrypted.json", `{"a":"four-states-value"}`)
	p.expect(0, "encrypt")
	p.expect(0, "decrypt", "--dangerously-bypass-protections")

	// only-unencrypted
	if o, _ := p.statusJSON(); o != "encrypted-missing" {
		t.Fatalf("only-unencrypted status: %s", o)
	}
	assertNo(t, "status", p.expect(0, "status").out(), "four-states-value")
	assertNo(t, "check", p.expect(0, "check").out(), "four-states-value")

	// both present (the riskiest state): warned about on both commands.
	p.expect(0, "set", "/b", "second-value")
	r := p.expect(0, "status")
	if !strings.Contains(r.stderr, "both files exist") {
		t.Fatalf("status must warn when both files exist:\n%s", r)
	}
	r = p.expect(0, "check")
	if !strings.Contains(r.stdout, "both files exist") {
		t.Fatalf("check must warn when both files exist:\n%s", r)
	}

	// only-encrypted
	p.remove("config.unencrypted.json")
	if o, _ := p.statusJSON(); o != "unencrypted-missing" {
		t.Fatalf("only-encrypted status: %s", o)
	}
	assertNo(t, "check", p.expect(0, "check").out(), "four-states-value", "second-value")

	// legacy zeze state: reported with the repair pointer everywhere.
	var m map[string]any
	if err := json.Unmarshal([]byte(p.read("config.encrypted.json")), &m); err != nil {
		t.Fatal(err)
	}
	meta := m["$zeze"].(map[string]any)
	meta["format"] = 1
	delete(meta, "home")
	data, _ := json.MarshalIndent(m, "", "  ")
	p.write("config.encrypted.json", string(data))
	r = p.expect(0, "status")
	if !strings.Contains(r.stderr, "run zeze init to repair") {
		t.Fatalf("status must report legacy state with the repair pointer:\n%s", r)
	}
	r = p.run("check")
	if r.code != 1 || !strings.Contains(r.stdout, "run zeze init to repair") {
		t.Fatalf("check must fail on legacy state with the repair pointer:\n%s", r)
	}
	p.init()
	r = p.expect(0, "status")
	if strings.Contains(r.stderr, "run zeze init to repair") {
		t.Fatalf("init must clear the legacy state:\n%s", r)
	}
}
