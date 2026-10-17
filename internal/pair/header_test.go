package pair

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CGuiho/jureuma/internal/crypt"
	"github.com/CGuiho/jureuma/internal/doc"
	"github.com/CGuiho/jureuma/internal/xerr"
)

// schemaFixture is a schema document (never configuration data) used as the
// embedded schema copy payload.
const schemaFixture = "{\n  \"$schema\": \"https://json-schema.org/draft/2020-12/schema\",\n  \"type\": \"object\"\n}\n"

func testHeader(t *testing.T) (*crypt.PrivateKey, *Header) {
	t.Helper()
	sk, err := crypt.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return &sk, &Header{Format: FormatVersion, PublicKey: sk.Public().String(), KeyID: crypt.KeyID(sk.Public()), Home: HomeRef}
}

func TestHeaderMarshalParseRoundTrip(t *testing.T) {
	_, h := testHeader(t)
	v := marshalHeader(h)
	if strings.Join(v.Keys, ",") != "format,publicKey,keyId,home" {
		t.Fatalf("identity field spellings/order: %v", v.Keys)
	}
	data := doc.Marshal(v)
	for _, want := range []string{`"format": 2,`, `"publicKey": "`, `"keyId": "`, `"home": "` + HomeRef + `"`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("header rendering lacks %q:\n%s", want, data)
		}
	}
	got, err := parseHeader("t", v)
	if err != nil {
		t.Fatal(err)
	}
	if *got != *h {
		t.Fatalf("round trip: got %+v want %+v", got, h)
	}
	// Format 1 is accepted and recorded: legacy reads keep working.
	one, _ := doc.NewNumber("1")
	v.Put("format", one)
	got, err = parseHeader("t", v)
	if err != nil || got.Format != 1 {
		t.Fatalf("format 1 header must parse as legacy: %v %+v", err, got)
	}
	// Malformed blocks return the typed legacy refusal; unknown generations
	// the explicit rejection; a contradicting keyId exits 5.
	cases := map[string]struct {
		block string
		is    error
		code  int
	}{
		"not an object":  {`1`, ErrLegacyHeader, xerr.CodeFailure},
		"missing format": {`{"publicKey":"` + h.PublicKey + `","keyId":"` + h.KeyID + `"}`, ErrLegacyHeader, xerr.CodeFailure},
		"unknown field":  {`{"format":2,"publicKey":"` + h.PublicKey + `","keyId":"` + h.KeyID + `","sync":{}}`, ErrLegacyHeader, xerr.CodeFailure},
		"bad home":       {`{"format":2,"publicKey":"` + h.PublicKey + `","keyId":"` + h.KeyID + `","home":7}`, ErrLegacyHeader, xerr.CodeFailure},
		"future format":  {`{"format":3,"publicKey":"` + h.PublicKey + `","keyId":"` + h.KeyID + `"}`, ErrUnsupportedFormat, xerr.CodeFailure},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseHeader("t", mustParse(t, tc.block))
			if err == nil || !errors.Is(err, tc.is) || xerr.CodeOf(err) != tc.code {
				t.Fatalf("want %v (exit %d), got %v", tc.is, tc.code, err)
			}
			if !strings.Contains(err.Error(), RepairHint) && tc.is == ErrLegacyHeader {
				t.Fatalf("legacy refusal must carry the repair pointer: %v", err)
			}
		})
	}
	sk2, _ := crypt.GenerateKey()
	mismatch := mustParse(t, `{"format":2,"publicKey":"`+h.PublicKey+`","keyId":"`+crypt.KeyID(sk2.Public())+`"}`)
	if _, err := parseHeader("t", mismatch); xerr.CodeOf(err) != xerr.CodeKey {
		t.Fatalf("keyId/publicKey mismatch: want exit 5, got %v", err)
	}
}

func TestSchemaReferenceShapes(t *testing.T) {
	sk, _ := crypt.GenerateKey()
	e := NewEncrypted(sk.Public(), "./config.schema.json")
	data := MarshalEncrypted(e)
	// Without an embedded copy the plain string stays the written form.
	if !strings.Contains(string(data), `"schema": "./config.schema.json"`) || strings.Contains(string(data), `"path"`) {
		t.Fatalf("plain-string schema reference expected:\n%s", data)
	}
	got, err := ParseEncrypted("t.encrypted.json", data)
	if err != nil || got.Meta.Schema != "./config.schema.json" || got.Meta.SchemaCopy != "" {
		t.Fatalf("string shape read: %v %+v", err, got.Meta)
	}
	if got.Meta.Home != HomeRef || got.Meta.Format != FormatVersion {
		t.Fatalf("identity not recorded: %+v", got.Meta)
	}
	// With an embedded copy the reference becomes { path, c }.
	if err := e.SetSchemaCopy([]byte(schemaFixture)); err != nil {
		t.Fatal(err)
	}
	data2 := MarshalEncrypted(e)
	if !bytes.Equal(MarshalEncrypted(e), data2) {
		t.Fatal("marshal must be deterministic")
	}
	if !strings.Contains(string(data2), `"path": "./config.schema.json"`) || !strings.Contains(string(data2), `"c": "jureuma:1:`) {
		t.Fatalf("{ path, c } shape expected:\n%s", data2)
	}
	got2, err := ParseEncrypted("t.encrypted.json", data2)
	if err != nil {
		t.Fatal(err)
	}
	text, err := got2.OpenSchemaCopy(SingleKey(sk))
	if err != nil || string(text) != schemaFixture {
		t.Fatalf("the schema envelope must decrypt back to the schema text: %v", err)
	}
	if _, err := got2.OpenSchemaCopy(SingleKey(mustKey(t))); xerr.CodeOf(err) != xerr.CodeKey {
		t.Fatalf("opening the copy without its key: want exit 5, got %v", err)
	}
	// The plain string stays readable in any format generation.
	legacy := strings.Replace(string(data), `"format": 2,`, `"format": 1,`, 1)
	got3, err := ParseEncrypted("t.encrypted.json", []byte(legacy))
	if err != nil || got3.Meta.Format != 1 || got3.Meta.Schema != "./config.schema.json" {
		t.Fatalf("format-1 string shape must parse: %v %+v", err, got3.Meta)
	}
	// A malformed schema block is refused.
	broken := strings.Replace(string(data2), `"path": "./config.schema.json"`, `"path": 3`, 1)
	if _, err := ParseEncrypted("t.encrypted.json", []byte(broken)); !errors.Is(err, ErrLegacyHeader) {
		t.Fatalf("malformed schema block: want the typed legacy refusal, got %v", err)
	}
}

func mustKey(t *testing.T) crypt.PrivateKey {
	t.Helper()
	sk, err := crypt.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return sk
}

func TestPlainFileRoundTripAndOldWrapperCompatibility(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	gitInit(t, root)
	_, h := testHeader(t)
	p := filepath.Join(root, "config.unencrypted.json")
	body := mustParse(t, `{"zeta":1,"alpha":{"b":true},"esc":"`+sentinel+`"}`)
	if err := WritePlainFile(p, &PlainFile{Header: h, Schema: "./config.schema.json", Body: body}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	order := []string{`"$jureuma"`, `"format"`, `"publicKey"`, `"keyId"`, `"home"`, `"$schema"`, `"zeta"`, `"alpha"`, `"b"`, `"esc"`}
	last := -1
	for _, k := range order {
		i := strings.Index(string(data[last+1:]), k)
		if i < 0 {
			t.Fatalf("plaintext: key %s missing or out of order\n%s", k, data)
		}
		last += 1 + i
	}
	got, err := ReadPlainFile(p, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Header == nil || *got.Header != *h || got.Schema != "./config.schema.json" {
		t.Fatalf("identity round trip: %+v %q", got.Header, got.Schema)
	}
	if got.Body.Get(MetaKey) != nil || got.Body.Get(SchemaKey) != nil {
		t.Fatal("the body must exclude the reserved keys")
	}
	if !doc.Equal(got.Body, body) {
		t.Fatalf("body round trip: %s", doc.Marshal(got.Body))
	}
	// The old wrappers stay byte-compatible: the old reader keeps rejecting
	// the $jureuma header and keeps passing $schema through as a body key.
	if _, err := ReadPlain(p, false); err == nil || !strings.Contains(err.Error(), "is reserved") {
		t.Fatalf("old ReadPlain must keep rejecting $jureuma: %v", err)
	}
	p2 := filepath.Join(root, "other.unencrypted.json")
	given := mustParse(t, `{"$schema":"./config.schema.json","a":1}`)
	if err := WritePlain(p2, given); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(p2)
	if !strings.Contains(string(data2), `"$schema"`) || strings.Contains(string(data2), `"$jureuma"`) {
		t.Fatalf("old WritePlain must write exactly what it is given:\n%s", data2)
	}
	if got, err := ReadPlain(p2, false); err != nil || got.Get(SchemaKey) == nil {
		t.Fatalf("old ReadPlain must pass $schema through as a body key: %v", err)
	}
}

func TestPlainFileJSONCHeaderInsert(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	gitInit(t, root)
	_, h := testHeader(t)
	p := filepath.Join(root, "config.unencrypted.jsonc")
	orig := "// top\n{\n  \"a\": 1, // about a\n  \"b\": [2, 3,],\n}\n"
	if err := os.WriteFile(p, []byte(orig), 0o600); err != nil {
		t.Fatal(err)
	}
	f := &PlainFile{Header: h, Schema: "./config.schema.json", Body: mustParse(t, `{"a":1,"b":[2,3]}`)}
	if err := WritePlainFile(p, f); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	for _, want := range []string{"// top", "// about a", "[2, 3,]", `"$jureuma"`, `"$schema"`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("header/pointer insert must preserve comments and formatting:\n%s", data)
		}
	}
	got, err := ReadPlainFile(p, true)
	if err != nil || got.Header == nil || *got.Header != *h || got.Schema != "./config.schema.json" {
		t.Fatalf("full read after insert: %v %+v", err, got)
	}
	// A second write of the same metadata is a no-op on the text.
	before := string(data)
	if err := WritePlainFile(p, got); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(p); string(after) != before {
		t.Fatalf("rewrite changed the file:\ngot:\n%s\nwant:\n%s", after, before)
	}
}

func TestCheckIdentity(t *testing.T) {
	sk, h := testHeader(t)
	_ = sk
	plain := &PlainFile{Header: h, Schema: "./config.schema.json", Body: doc.NewObject()}
	enc := NewEncrypted(mustKey(t).Public(), "./config.schema.json")
	enc.Meta.PublicKey, enc.Meta.KeyID, enc.Meta.Home = h.PublicKey, h.KeyID, h.Home
	if w, err := CheckIdentity(plain, enc); err != nil || len(w) != 0 {
		t.Fatalf("matching identity: %v %v", w, err)
	}
	other := *enc
	other.Meta.Home = "/elsewhere"
	other.Meta.Schema = "./other.schema.json"
	w, err := CheckIdentity(plain, &other)
	if err != nil || len(w) != 2 {
		t.Fatalf("home and schema-path differences must warn only: %v %v", w, err)
	}
	other = *enc
	other.Meta.PublicKey = mustKey(t).Public().String()
	if _, err := CheckIdentity(plain, &other); xerr.CodeOf(err) != xerr.CodeKey {
		t.Fatalf("publicKey disagreement: want exit 5, got %v", err)
	}
	other = *enc
	other.Meta.KeyID = crypt.KeyID(mustKey(t).Public())
	if _, err := CheckIdentity(plain, &other); xerr.CodeOf(err) != xerr.CodeKey {
		t.Fatalf("keyId disagreement: want exit 5, got %v", err)
	}
	// Single-file states and headerless plaintexts are not identity failures.
	if w, err := CheckIdentity(&PlainFile{Body: doc.NewObject()}, enc); err != nil || len(w) != 0 {
		t.Fatalf("headerless plaintext: %v %v", w, err)
	}
	if w, err := CheckIdentity(plain, nil); err != nil || len(w) != 0 {
		t.Fatalf("only-unencrypted state: %v %v", w, err)
	}
}

func TestTypedRepairRefusals(t *testing.T) {
	err := MissingPlainIdentity("config.unencrypted.json")
	if !errors.Is(err, ErrMissingPlainIdentity) || xerr.CodeOf(err) != xerr.CodeFailure {
		t.Fatalf("typed refusal: %v", err)
	}
	if !strings.Contains(err.Error(), "run jureuma init to repair") {
		t.Fatalf("the repair pointer must be exact: %q", err)
	}
	for _, err := range []error{LegacyHeader("config.encrypted.json"), MissingPlainIdentity("x")} {
		if strings.Contains(err.Error(), sentinel) {
			t.Fatal("refusals must stay value-free")
		}
	}
	if !errors.Is(LegacyHeader("x"), ErrLegacyHeader) || !errors.Is(UnsupportedFormat("x"), ErrUnsupportedFormat) {
		t.Fatal("every refusal must be detectable by type")
	}
}

// legacyPlainHeader renders a hand-made format-1 $jureuma block for an
// unencrypted file.
func legacyPlainHeader(h *Header) string {
	return `{"format": 1, "publicKey": "` + h.PublicKey + `", "keyId": "` + h.KeyID + `", "home": "` + h.Home + `"}`
}

// writeLegacyEncrypted writes a hand-made legacy encrypted file: the same
// content this jureuma writes, with the $jureuma/format generation of old
// jureuma.
func writeLegacyEncrypted(t *testing.T, path string, e *Encrypted) {
	t.Helper()
	data := strings.Replace(string(MarshalEncrypted(e)), `"format": 2,`, `"format": 1,`, 1)
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// assertRepairRefusal checks the contract of every refusal: typed, value-free
// and carrying the exact init repair pointer.
func assertRepairRefusal(t *testing.T, err error) {
	t.Helper()
	if err == nil || !errors.Is(err, ErrLegacyHeader) {
		t.Fatalf("want the typed legacy refusal, got %v", err)
	}
	if !strings.Contains(err.Error(), "run jureuma init to repair") {
		t.Fatalf("refusal must point at init: %q", err)
	}
	if strings.Contains(err.Error(), sentinel) {
		t.Fatal("refusals must stay value-free")
	}
}

func TestLegacyPairRefusesMutations(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	gitInit(t, root)
	sk, h := testHeader(t)
	encPath := filepath.Join(root, "config.encrypted.json")
	plainPath := filepath.Join(root, "config.unencrypted.json")
	writeLegacyEncrypted(t, encPath, NewEncrypted(sk.Public(), "./config.schema.json"))
	legacyPlain := `{"$jureuma": ` + legacyPlainHeader(h) + `, "a": 1}`
	if err := os.WriteFile(plainPath, []byte(legacyPlain), 0o600); err != nil {
		t.Fatal(err)
	}

	// Reads keep working and record the legacy generation.
	enc, err := ReadEncrypted(encPath)
	if err != nil || enc.Meta.Format != 1 {
		t.Fatalf("format-1 read must keep working: %v %+v", err, enc.Meta)
	}
	f, err := ReadPlainFile(plainPath, false)
	if err != nil || f.Header == nil || f.Header.Format != 1 {
		t.Fatalf("legacy plaintext read: %v %+v", err, f.Header)
	}

	// Every mutating pair entry point refuses.
	assertRepairRefusal(t, WriteEncrypted(encPath, NewEncrypted(sk.Public(), "./config.schema.json")))
	assertRepairRefusal(t, WritePlainFile(plainPath, f))
	assertRepairRefusal(t, WritePlain(plainPath, doc.NewObject()))

	// The init repair path replaces both files with format 2.
	if err := RepairEncrypted(encPath, NewEncrypted(sk.Public(), "./config.schema.json")); err != nil {
		t.Fatal(err)
	}
	rep := &PlainFile{Header: h, Schema: "./config.schema.json", Body: mustParse(t, `{"a":1}`)}
	if err := RepairPlainFile(plainPath, rep); err != nil {
		t.Fatal(err)
	}
	enc2, err := ReadEncrypted(encPath)
	if err != nil || enc2.Meta.Format != FormatVersion || enc2.Meta.Home != HomeRef {
		t.Fatalf("repaired encrypted file: %v %+v", err, enc2.Meta)
	}
	f2, err := ReadPlainFile(plainPath, false)
	if err != nil || f2.Header == nil || f2.Header.Format != FormatVersion || f2.Schema != "./config.schema.json" {
		t.Fatalf("repaired plaintext: %v %+v %q", err, f2.Header, f2.Schema)
	}

	// After repair the ordinary mutation entry points work again.
	if err := WriteEncrypted(encPath, enc2); err != nil {
		t.Fatal(err)
	}
	if err := WritePlainFile(plainPath, f2); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyJSONCPlainRefusesMutations(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	gitInit(t, root)
	sk, h := testHeader(t)
	encPath := filepath.Join(root, "config.encrypted.json")
	plainPath := filepath.Join(root, "config.unencrypted.jsonc")
	writeLegacyEncrypted(t, encPath, NewEncrypted(sk.Public(), "./config.schema.json"))
	legacy := "// legacy note\n{\n  \"$jureuma\": " + legacyPlainHeader(h) + ", // the old header\n  \"a\": 1,\n}\n"
	if err := os.WriteFile(plainPath, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := ReadPlainFile(plainPath, true)
	if err != nil || f.Header == nil || f.Header.Format != 1 {
		t.Fatalf("legacy .jsonc read: %v %+v", err, f.Header)
	}
	assertRepairRefusal(t, WritePlainFile(plainPath, f))
	assertRepairRefusal(t, WritePlain(plainPath, mustParse(t, `{"a":2}`)))

	// The repair goes through the lossless writer: the comments survive.
	if err := RepairPlainFile(plainPath, &PlainFile{Header: h, Schema: "./config.schema.json", Body: mustParse(t, `{"a":1}`)}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(plainPath)
	for _, want := range []string{"// legacy note", "// the old header", `"a": 1`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("repair must preserve comments and formatting:\n%s", data)
		}
	}
	got, err := ReadPlainFile(plainPath, true)
	if err != nil || got.Header == nil || got.Header.Format != FormatVersion {
		t.Fatalf("repaired .jsonc: %v %+v", err, got.Header)
	}
}

func TestHeaderlessPlaintextStaysMutable(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	gitInit(t, root)
	sk, h := testHeader(t)
	encPath := filepath.Join(root, "config.encrypted.json")
	plainPath := filepath.Join(root, "config.unencrypted.json")
	enc := NewEncrypted(sk.Public(), "./config.schema.json")
	if err := WriteEncrypted(encPath, enc); err != nil {
		t.Fatal(err)
	}
	// A headerless plaintext is never a refusal trigger.
	if err := WritePlain(plainPath, mustParse(t, `{"a":1}`)); err != nil {
		t.Fatalf("headerless plaintext must stay mutable: %v", err)
	}
	if err := WritePlainFile(plainPath, &PlainFile{Body: mustParse(t, `{"a":2}`)}); err != nil {
		t.Fatalf("headerless full write must stay mutable: %v", err)
	}
	if err := WriteEncrypted(encPath, enc); err != nil {
		t.Fatalf("mutation beside a headerless plaintext must stay mutable: %v", err)
	}
	if err := RepairPlainFile(plainPath, &PlainFile{Header: h, Schema: "./config.schema.json", Body: mustParse(t, `{"a":3}`)}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPlainFile(plainPath, false)
	if err != nil || got.Header == nil || got.Header.Format != FormatVersion {
		t.Fatalf("header insert through repair: %v %+v", err, got.Header)
	}
}

func TestMalformedHeaderRefusesMutations(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	gitInit(t, root)
	sk, _ := testHeader(t)
	encPath := filepath.Join(root, "config.encrypted.json")
	plainPath := filepath.Join(root, "config.unencrypted.json")
	if err := WriteEncrypted(encPath, NewEncrypted(sk.Public(), "./config.schema.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plainPath, []byte(`{"$jureuma": 1, "a": "`+sentinel+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	assertRepairRefusal(t, WritePlain(plainPath, mustParse(t, `{"a":1}`)))
	// The refusal keys on either side: the encrypted file is clean, but its
	// plaintext sibling carries the malformed block.
	assertRepairRefusal(t, WriteEncrypted(encPath, NewEncrypted(sk.Public(), "./config.schema.json")))
}

func TestUnsupportedFormatIsNeverOverwritten(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	gitInit(t, root)
	sk, _ := testHeader(t)
	encPath := filepath.Join(root, "config.encrypted.json")
	data := strings.Replace(string(MarshalEncrypted(NewEncrypted(sk.Public(), "./config.schema.json"))), `"format": 2,`, `"format": 3,`, 1)
	if err := os.WriteFile(encPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, err := range []error{
		func() error { _, e := ReadEncrypted(encPath); return e }(),
		WriteEncrypted(encPath, NewEncrypted(sk.Public(), "./config.schema.json")),
		RepairEncrypted(encPath, NewEncrypted(sk.Public(), "./config.schema.json")),
	} {
		if !errors.Is(err, ErrUnsupportedFormat) {
			t.Fatalf("a future format must be rejected explicitly, got %v", err)
		}
	}
}
