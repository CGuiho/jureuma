package pair

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CGuiho/jureuma/internal/config"
	"github.com/CGuiho/jureuma/internal/crypt"
	"github.com/CGuiho/jureuma/internal/doc"
	"github.com/CGuiho/jureuma/internal/jsonc"
	"github.com/CGuiho/jureuma/internal/xerr"
)

const sentinel = "SENTINEL-7f3a9c-value"

func mustParse(t *testing.T, s string) *doc.Value {
	t.Helper()
	v, err := jsonc.Parse([]byte(s), true)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return v
}

func touch(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestResolveNaming(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Project{Pairs: []config.PairConfig{
		{Name: "production", Schema: "./shared.schema.json"},
		{Name: "custom", Encrypted: "./x/custom.encrypted.json", Unencrypted: "./x/custom.unencrypted.jsonc"},
	}}
	tests := []struct {
		name                    string
		opts                    Options
		wantEnc, wantPlain, sch string
		wantSrc                 string
		wantCode                int
	}{
		{name: "default", opts: Options{Root: root}, wantEnc: "config.encrypted.json", wantPlain: "config.unencrypted.json", sch: "config.schema.json", wantSrc: SchemaFromDefault},
		{name: "first configured", opts: Options{Root: root, Config: cfg}, wantEnc: "production.encrypted.json", wantPlain: "production.unencrypted.json", sch: "shared.schema.json", wantSrc: SchemaFromConfig},
		{name: "nested stem", opts: Options{Root: root, Name: "apps/api/config"}, wantEnc: "apps/api/config.encrypted.json", wantPlain: "apps/api/config.unencrypted.json", sch: "apps/api/config.schema.json", wantSrc: SchemaFromDefault},
		{name: "configured paths", opts: Options{Root: root, Name: "custom", Config: cfg}, wantEnc: "x/custom.encrypted.json", wantPlain: "x/custom.unencrypted.jsonc", sch: "x/custom.schema.json", wantSrc: SchemaFromDefault},
		{name: "dotdot", opts: Options{Root: root, Name: "../x"}, wantCode: xerr.CodeUsage},
		{name: "absolute", opts: Options{Root: root, Name: "/etc/x"}, wantCode: xerr.CodeUsage},
		{name: "bad encrypted name", opts: Options{Root: root, EncryptedFile: filepath.Join(root, "config.json")}, wantCode: xerr.CodeUsage},
		{name: "bad unencrypted name", opts: Options{Root: root, UnencryptedFile: filepath.Join(root, "plain.json")}, wantCode: xerr.CodeUsage},
		{name: "derive from unencrypted", opts: Options{Root: root, UnencryptedFile: filepath.Join(root, "a.unencrypted.jsonc")}, wantEnc: "a.encrypted.json", wantPlain: "a.unencrypted.jsonc", sch: "a.schema.json", wantSrc: SchemaFromDefault},
		{name: "schema flag", opts: Options{Root: root, SchemaFile: filepath.Join(root, "s.schema.json")}, wantEnc: "config.encrypted.json", wantPlain: "config.unencrypted.json", sch: "s.schema.json", wantSrc: SchemaFromFlag},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := Resolve(tc.opts)
			if tc.wantCode != 0 {
				if err == nil || xerr.CodeOf(err) != tc.wantCode {
					t.Fatalf("want code %d, got %v", tc.wantCode, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			rel := func(x string) string { r, _ := filepath.Rel(root, x); return filepath.ToSlash(r) }
			if rel(p.EncryptedPath) != tc.wantEnc || rel(p.UnencryptedPath) != tc.wantPlain || rel(p.SchemaPath) != tc.sch || p.SchemaSource != tc.wantSrc {
				t.Fatalf("got %s %s %s %s", rel(p.EncryptedPath), rel(p.UnencryptedPath), rel(p.SchemaPath), p.SchemaSource)
			}
		})
	}
}

func TestResolveBothPlainVariants(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "config.unencrypted.json"))
	touch(t, filepath.Join(root, "config.unencrypted.jsonc"))
	if _, err := Resolve(Options{Root: root}); xerr.CodeOf(err) != xerr.CodeUsage {
		t.Fatalf("want usage error, got %v", err)
	}
	os.Remove(filepath.Join(root, "config.unencrypted.json"))
	p, err := Resolve(Options{Root: root})
	if err != nil || !p.UnencryptedIsJSONC {
		t.Fatalf("want jsonc pair, got %v %+v", err, p)
	}
}

func TestSchemaPrecedenceAndFlagConflict(t *testing.T) {
	root := t.TempDir()
	sk, _ := crypt.GenerateKey()
	e := NewEncrypted(sk.Public(), "./meta.schema.json")
	if err := WriteEncrypted(filepath.Join(root, "config.encrypted.json"), e); err != nil {
		t.Fatal(err)
	}
	p, err := Resolve(Options{Root: root})
	if err != nil || filepath.Base(p.SchemaPath) != "meta.schema.json" || p.SchemaSource != SchemaFromEncrypted {
		t.Fatalf("got %v %+v", err, p)
	}
	cfg := &config.Project{Pairs: []config.PairConfig{{Name: "config", Schema: "cfg.schema.json"}}}
	p, _ = Resolve(Options{Root: root, Config: cfg})
	if filepath.Base(p.SchemaPath) != "cfg.schema.json" {
		t.Fatalf("config should win: %s", p.SchemaPath)
	}
	p, _ = Resolve(Options{Root: root, Config: cfg, SchemaFile: filepath.Join(root, "other.schema.json")})
	if xerr.CodeOf(p.CheckSchemaFlag()) != xerr.CodeUsage {
		t.Fatal("disagreeing --schema-file must be a usage error")
	}
	p, _ = Resolve(Options{Root: root, Config: cfg, SchemaFile: filepath.Join(root, "cfg.schema.json")})
	if p.CheckSchemaFlag() != nil {
		t.Fatal("agreeing --schema-file must pass")
	}
}

func TestEncryptedRoundTripAndChecks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.encrypted.json")
	sk, _ := crypt.GenerateKey()
	pk := sk.Public()
	plain := mustParse(t, `{"b":{"url":"`+sentinel+`","n":1.50},"a":[true,null,"x"],"empty":{}}`)
	body, err := EncryptDoc(pk, plain, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	e := &Encrypted{Meta: Meta{Format: FormatVersion, PublicKey: pk.String(), KeyID: crypt.KeyID(pk), Home: HomeRef, Sync: ComputeSync(sk, plain, body), Pending: []string{"/z", "/a/0", "/z"}}, Body: body}
	if err := WriteEncrypted(path, e); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), sentinel) {
		t.Fatal("plaintext leaked into the encrypted file")
	}
	if !strings.HasPrefix(string(data), "{\n  \"$jureuma\"") {
		t.Fatal("$jureuma must come first")
	}
	got, err := ReadEncrypted(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.Meta.Pending, ",") != "/a/0,/z" {
		t.Fatalf("pending not sorted/unique: %v", got.Meta.Pending)
	}
	dec, err := DecryptDoc(sk, got.Body)
	if err != nil || !doc.Equal(dec, plain) {
		t.Fatalf("round trip failed: %v", err)
	}
	if dec.Get("b").Get("n").N != "1.50" {
		t.Fatal("number text must be preserved")
	}
	// Reuse unchanged ciphertexts.
	body2, err := EncryptDoc(pk, plain, got, &sk)
	if err != nil || !doc.Equal(body2, got.Body) {
		t.Fatal("unchanged values must keep their ciphertext")
	}
	plain.Get("b").Put("n", doc.NewString("changed"))
	body3, _ := EncryptDoc(pk, plain, got, &sk)
	if body3.Get("b").Get("url").S != got.Body.Get("b").Get("url").S || body3.Get("b").Get("n").S == got.Body.Get("b").Get("n").S {
		t.Fatal("only the changed value must be re-encrypted")
	}
	// Relocated ciphertext fails (pointer bound into AAD).
	moved := got.Body.Clone()
	moved.Get("b").Put("n", doc.NewString(moved.Get("b").Get("url").S))
	if _, err := DecryptDoc(sk, moved); xerr.CodeOf(err) != xerr.CodeKey || strings.Contains(err.Error(), sentinel) {
		t.Fatalf("relocated ciphertext must fail with exit 5: %v", err)
	}
}

func TestReadEncryptedRejects(t *testing.T) {
	dir := t.TempDir()
	sk, _ := crypt.GenerateKey()
	sk2, _ := crypt.GenerateKey()
	pk := sk.Public()
	good := NewEncrypted(pk, "")
	cases := map[string]struct {
		content string
		code    int
	}{
		"keyId mismatch": {strings.Replace(string(MarshalEncrypted(good)), crypt.KeyID(pk), crypt.KeyID(sk2.Public()), 1), xerr.CodeKey},
		"plaintext leaf": {strings.Replace(string(MarshalEncrypted(good)), "\"pending\": []\n  }", "\"pending\": []\n  },\n  \"x\": \""+sentinel+"\"", 1), xerr.CodeFailure},
		"no meta":        {`{"x": 1}`, xerr.CodeFailure},
		"syntax":         {`{"$jureuma": ` + sentinel, xerr.CodeFailure},
		"unknown meta":   {strings.Replace(string(MarshalEncrypted(good)), `"format": 2,`, `"format": 2, "extra": "`+sentinel+`",`, 1), xerr.CodeFailure},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(dir, strings.ReplaceAll(name, " ", "_")+".encrypted.json")
			os.WriteFile(p, []byte(tc.content), 0o644)
			_, err := ReadEncrypted(p)
			if err == nil || xerr.CodeOf(err) != tc.code {
				t.Fatalf("want code %d, got %v", tc.code, err)
			}
			if strings.Contains(err.Error(), sentinel) {
				t.Fatal("error leaked file content")
			}
		})
	}
}

func TestReadPlain(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.unencrypted.jsonc")
	os.WriteFile(p, []byte("{\n // c\n \"a\": \""+sentinel+"\",\n}"), 0o600)
	if _, err := ReadPlain(p, true); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPlain(p, false); err == nil || strings.Contains(err.Error(), sentinel) {
		t.Fatalf("strict read must fail without leaking: %v", err)
	}
	os.WriteFile(p, []byte(`{"$jureuma": 1}`), 0o600)
	if _, err := ReadPlain(p, true); err == nil {
		t.Fatal("$jureuma must be reserved")
	}
}

func gitAvailable() bool { _, err := exec.LookPath("git"); return err == nil }

func gitInit(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", "-q", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("git init failed: %v %s", err, out)
	}
}

func TestGitignoreAndWritePlain(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	gitInit(t, root)
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules\n"), 0o644)
	p := filepath.Join(root, "config.unencrypted.json")
	if err := WritePlain(p, mustParse(t, `{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if !HasGitignoreBlock(root) {
		t.Fatal("block must be added")
	}
	data, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if !strings.HasPrefix(string(data), "node_modules\n\n# >>> jureuma") {
		t.Fatalf("existing lines must be kept: %q", data)
	}
	if changed, _ := EnsureGitignoreBlock(root); changed {
		t.Fatal("block must be idempotent")
	}
	if st, _ := os.Stat(p); st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", st.Mode().Perm())
	}
	// A negation rule defeats the block: refuse.
	os.WriteFile(filepath.Join(root, ".gitignore"), append(data, []byte("!other.unencrypted.json\n")...), 0o644)
	if err := WritePlain(filepath.Join(root, "other.unencrypted.json"), doc.NewObject()); err == nil {
		t.Fatal("must refuse a file git does not ignore")
	}
	// A .jsonc target is written like any other plaintext target.
	if err := WritePlain(filepath.Join(root, "x.unencrypted.jsonc"), doc.NewObject()); err != nil {
		t.Fatalf("must write .jsonc: %v", err)
	}
}

func TestWritePlainJSONC(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	gitInit(t, root)
	p := filepath.Join(root, "config.unencrypted.jsonc")

	// A missing .jsonc target is created as canonical JSON (valid JSONC).
	if err := WritePlain(p, mustParse(t, `{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(p); string(data) != "{\n  \"a\": 1\n}\n" {
		t.Fatalf("created: %q", data)
	}
	if st, _ := os.Stat(p); st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", st.Mode().Perm())
	}

	// An existing .jsonc target is merged in place: comments and formatting
	// outside the edited spans survive byte for byte.
	orig := "// header\n{\n  \"a\": 1, // about a\n  \"b\": [2, 3,],\n}\n"
	if err := os.WriteFile(p, []byte(orig), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WritePlain(p, mustParse(t, `{"a":9,"b":[2,3],"c":4}`)); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	want := "// header\n{\n  \"a\": 9, // about a\n  \"b\": [2, 3,],\n  \"c\": 4\n}\n"
	if string(data) != want {
		t.Fatalf("merge:\ngot:\n%q\nwant:\n%q", data, want)
	}

	// An unparseable current file is refused and left untouched.
	bad := "{ not json"
	if err := os.WriteFile(p, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WritePlain(p, mustParse(t, `{"a":1}`)); err == nil {
		t.Fatal("must refuse an unparseable .jsonc")
	}
	if data, _ := os.ReadFile(p); string(data) != bad {
		t.Fatalf("unparseable file was touched: %q", data)
	}

	// A change the editor cannot represent is refused and left untouched.
	arr := "{\"l\": [1, 3]}\n"
	if err := os.WriteFile(p, []byte(arr), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WritePlain(p, mustParse(t, `{"l":[1,2,3]}`)); err == nil {
		t.Fatal("must refuse an unrepresentable change")
	}
	if data, _ := os.ReadFile(p); string(data) != arr {
		t.Fatalf("unrepresentable file was touched: %q", data)
	}
}

func TestTrackedByGit(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	gitInit(t, root)
	p := filepath.Join(root, "config.unencrypted.json")
	os.WriteFile(p, []byte("{}"), 0o600)
	if tr, _ := TrackedByGit(root, p); tr {
		t.Fatal("untracked file reported tracked")
	}
	cmd := exec.Command("git", "-C", root, "add", "-f", "config.unencrypted.json")
	if err := cmd.Run(); err != nil {
		t.Skip(err)
	}
	if tr, _ := TrackedByGit(root, p); !tr {
		t.Fatal("tracked file not detected")
	}
	if err := WritePlain(p, doc.NewObject()); err == nil {
		t.Fatal("must refuse to write a tracked file")
	}
}

func TestStatusStates(t *testing.T) {
	sk, _ := crypt.GenerateKey()
	pk := sk.Public()
	base := mustParse(t, `{"a":"one","b":"two","c":"three","d":"four"}`)
	body, _ := EncryptDoc(pk, base, nil, nil)
	e := &Encrypted{Meta: Meta{Format: FormatVersion, PublicKey: pk.String(), KeyID: crypt.KeyID(pk), Home: HomeRef, Sync: ComputeSync(sk, base, body)}, Body: body}
	p := &Pair{Name: "config"}

	if r := Status(p, e, base, &sk); r.Overall != StateInSync {
		t.Fatalf("want in-sync, got %+v", r)
	}
	plain := base.Clone()
	plain.Put("a", doc.NewString("ONE"))       // unencrypted-newer
	plain.Put("c", doc.NewString("plain-new")) // conflict (enc changed too)
	enc := &Encrypted{Meta: e.Meta, Body: body.Clone()}
	bv, _ := EncryptAt(pk, "/b", doc.NewString("TWO"))
	enc.Body.Put("b", bv) // encrypted-newer
	cv, _ := EncryptAt(pk, "/c", doc.NewString("enc-new"))
	enc.Body.Put("c", cv)
	dv, _ := EncryptAt(pk, "/d", doc.NewString("four")) // re-encrypted, same value on both sides after a set
	enc.Body.Put("d", dv)
	plain.Put("d", doc.NewString("four"))
	enc.Meta.Pending = []string{"/d"}
	r := Status(p, enc, plain, &sk)
	want := map[string]string{"/a": StateUnencryptedNewer, "/b": StateEncryptedNewer, "/c": StateConflict, "/d": StateEncryptedNewer}
	for _, ps := range r.Paths {
		if want[ps.Pointer] != ps.State {
			t.Errorf("%s: want %s got %s", ps.Pointer, want[ps.Pointer], ps.State)
		}
	}
	if r.Overall != StateConflict {
		t.Fatalf("overall %s", r.Overall)
	}
	if r := Status(p, enc, nil, &sk); r.Overall != StateUnencryptedMissing {
		t.Fatalf("got %s", r.Overall)
	}
	if r := Status(p, enc, plain, nil); r.Overall != StateKeyUnavailable {
		t.Fatalf("got %s", r.Overall)
	}
	// Both sides changed to the same value: in-sync.
	plain.Put("b", doc.NewString("TWO"))
	plain.Put("c", doc.NewString("enc-new"))
	plain.Put("a", doc.NewString("one"))
	r = Status(p, enc, plain, &sk)
	for _, ps := range r.Paths {
		if ps.Pointer == "/c" && ps.State != StateInSync {
			t.Fatalf("/c equal on both sides must be in-sync, got %s", ps.State)
		}
	}
}

func TestConcreteTokens(t *testing.T) {
	v := mustParse(t, `{"a":[1,2],"o":{}}`)
	got := ConcreteTokens(v, []string{"a", "-"})
	if strings.Join(got, "/") != "a/2" {
		t.Fatal(got)
	}
	got = ConcreteTokens(v, []string{"o", "new", "-"})
	if strings.Join(got, "/") != "o/new/-" {
		t.Fatal(got)
	}
}

func TestResolveConfinesCommittedPaths(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	for _, pc := range []config.PairConfig{
		{Name: "config", Schema: "../../evil.schema.json"},
		{Name: "config", Schema: filepath.Join(outside, "evil.schema.json")},
		{Name: "config", Schema: "./victim.txt"},
		{Name: "config", Encrypted: "../x.encrypted.json"},
		{Name: "config", Unencrypted: "../x.unencrypted.json"},
	} {
		cfg := &config.Project{Pairs: []config.PairConfig{pc}}
		if _, err := Resolve(Options{Root: root, Config: cfg}); xerr.CodeOf(err) != xerr.CodeUsage {
			t.Errorf("%+v: want usage error, got %v", pc, err)
		}
	}
	// $jureuma.schema from the committed encrypted file.
	sk, _ := crypt.GenerateKey()
	for _, ref := range []string{"../home/victim.txt", "../evil.schema.json", "./notaschema.json"} {
		if err := WriteEncrypted(filepath.Join(root, "config.encrypted.json"), NewEncrypted(sk.Public(), ref)); err != nil {
			t.Fatal(err)
		}
		if _, err := Resolve(Options{Root: root}); xerr.CodeOf(err) != xerr.CodeUsage {
			t.Errorf("$jureuma.schema %s: want usage error, got %v", ref, err)
		}
	}
	// A symlinked directory that leaves the root is refused too.
	if err := os.Symlink(outside, filepath.Join(root, "link")); err == nil {
		cfg := &config.Project{Pairs: []config.PairConfig{{Name: "config", Schema: "link/x.schema.json"}}}
		os.Remove(filepath.Join(root, "config.encrypted.json"))
		if _, err := Resolve(Options{Root: root, Config: cfg}); xerr.CodeOf(err) != xerr.CodeUsage {
			t.Errorf("symlinked schema dir: want usage error, got %v", err)
		}
	}
	// Explicit flags may still point outside.
	if _, err := Resolve(Options{Root: root, SchemaFile: filepath.Join(outside, "s.schema.json")}); err != nil {
		t.Fatalf("flag outside root refused: %v", err)
	}
}

func TestCheckSchemaFlagAgainstDefaultSchema(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "config.schema.json"))
	p, err := Resolve(Options{Root: root, SchemaFile: filepath.Join(root, "loose.schema.json")})
	if err != nil {
		t.Fatal(err)
	}
	if xerr.CodeOf(p.CheckSchemaFlag()) != xerr.CodeUsage {
		t.Fatal("a --schema-file that disagrees with the existing default schema must be refused")
	}
	p, _ = Resolve(Options{Root: root, SchemaFile: filepath.Join(root, "config.schema.json")})
	if p.CheckSchemaFlag() != nil {
		t.Fatal("agreeing --schema-file must pass")
	}
}

func TestWriteRefusesFilesJureumaCannotRead(t *testing.T) {
	root := t.TempDir()
	sk, _ := crypt.GenerateKey()
	e := NewEncrypted(sk.Public(), "")
	big := strings.Repeat("x", maxFileSize)
	e.Body.Put("a", doc.NewString(big))
	path := filepath.Join(root, "config.encrypted.json")
	if err := WriteEncrypted(path, e); xerr.CodeOf(err) != xerr.CodeValidation {
		t.Fatalf("oversized encrypted write: want exit 4, got %v", err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("an oversized file was written")
	}
	// A file that is too large is reported as such.
	if err := os.WriteFile(path, []byte(`{"x":"`+big+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadEncrypted(path); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("want a file-too-large error, got %v", err)
	}
}

func TestAppendWithoutArrayAndEmptyContainers(t *testing.T) {
	root := mustParse(t, `{"arr":[1],"obj":{"-":"k"}}`)
	for ptr, want := range map[string]bool{"/arr/-": false, "/arr/-/x": false, "/arr/-/-": true, "/missing/-": true, "/obj/-": false, "/arr/0/-": true} {
		toks, _ := doc.ParsePointer(ptr)
		if got := AppendWithoutArray(root, toks); got != want {
			t.Errorf("%s: got %v want %v", ptr, got, want)
		}
	}
	plain := mustParse(t, `{"a":"x","features":{},"hosts":[],"arr":[]}`)
	body := mustParse(t, `{"a":"ct","arr":[]}`)
	got := EmptyContainersOnlyIn(plain, body)
	if strings.Join(got, ",") != "/features,/hosts" {
		t.Fatalf("got %v", got)
	}
}
