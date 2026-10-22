package pair

import (
	"errors"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/CGuiho/zeze/internal/crypt"
	"github.com/CGuiho/zeze/internal/doc"
	"github.com/CGuiho/zeze/internal/jsonc"
	"github.com/CGuiho/zeze/internal/paths"
	"github.com/CGuiho/zeze/internal/xerr"
)

// MetaKey is the reserved top-level key of both pair files.
const MetaKey = "$zeze"

// FormatVersion is the format generation this zeze writes. Parsing accepts
// 1 (legacy) and 2; every write renders FormatVersion, so the Format fields of
// Header and Meta record the generation that was read.
const FormatVersion = 2

// Encrypted is a parsed encrypted file. Body excludes $zeze.
type Encrypted struct {
	Meta Meta
	Body *doc.Value
}

// Meta is the $zeze block: the shared identity (format, publicKey, keyId,
// home) plus the schema reference, the sync baseline and the pending paths.
// Schema is the readable path; SchemaCopy is the schema document encrypted to
// the pair key ("" when no embedded copy is stored), and its presence turns
// the written schema reference into the { path, c } form.
type Meta struct {
	Format           int
	PublicKey, KeyID string
	Home             string
	Schema           string
	SchemaCopy       string
	Sync             map[string]SyncEntry
	Pending          []string
}

// SyncEntry is the per-leaf baseline: c tags the ciphertext, h the plaintext.
type SyncEntry struct{ C, H string }

// PublicKeyValue parses Meta.PublicKey.
func (m Meta) PublicKeyValue() (crypt.PublicKey, error) {
	return crypt.ParsePublicKey(m.PublicKey)
}

// NewEncrypted returns an empty encrypted file for pk.
func NewEncrypted(pk crypt.PublicKey, schemaRef string) *Encrypted {
	return &Encrypted{
		Meta: Meta{Format: FormatVersion, PublicKey: pk.String(), KeyID: crypt.KeyID(pk), Home: HomeRef, Schema: schemaRef, Sync: map[string]SyncEntry{}},
		Body: doc.NewObject(),
	}
}

var tagRe = regexp.MustCompile(`^[0-9a-f]{32}$`)

// ErrNotFound is wrapped when a pair file does not exist.
var ErrNotFound = fs.ErrNotExist

// ReadEncrypted parses and checks an encrypted file: format, keyId/publicKey
// consistency (exit 5 on mismatch), sync/pending shape, and that every leaf is
// a ciphertext.
func ReadEncrypted(path string) (*Encrypted, error) {
	name := filepath.Base(path)
	data, err := readLimited(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &xerr.Error{Code: xerr.CodeFailure, Kind: xerr.KindFailure, Msg: name + ": not found (run zeze encrypt if the configuration is decrypted, or zeze init)", Err: fs.ErrNotExist}
		}
		return nil, readErr(name, err)
	}
	return ParseEncrypted(name, data)
}

// ParseEncrypted parses encrypted-file content; name is used in error messages.
// A $zeze block with format 1 parses and records Format 1 (legacy: reads
// keep working); an unknown future format is rejected explicitly; a
// present-but-malformed block returns the typed legacy refusal.
func ParseEncrypted(name string, data []byte) (*Encrypted, error) {
	root, err := jsonc.Parse(data, true)
	if err != nil {
		return nil, xerr.Failure("%s: %v", name, err)
	}
	if root.Kind != doc.Object {
		return nil, xerr.Failure("%s: top level must be an object", name)
	}
	mv := root.Get(MetaKey)
	if mv == nil {
		return nil, xerr.Failure("%s: missing %s block", name, MetaKey)
	}
	meta, err := parseMeta(name, mv)
	if err != nil {
		return nil, err
	}
	e := &Encrypted{Meta: *meta}
	body := doc.NewObject()
	for _, k := range root.Keys {
		if k == MetaKey {
			continue
		}
		body.Put(k, root.M[k])
	}
	for _, l := range doc.Leaves(body) {
		if l.Value.Kind != doc.String || !crypt.IsCiphertext(l.Value.S) {
			return nil, xerr.Failure("%s: %s: value is not a zeze ciphertext", name, l.Pointer)
		}
	}
	e.Body = body
	return e, nil
}

// parseMeta parses the $zeze block of an encrypted file. The identity
// fields are shared with the unencrypted header; schema accepts the readable
// path alone or the { path, c } form with the embedded schema copy in any
// format generation.
func parseMeta(name string, mv *doc.Value) (*Meta, error) {
	if mv.Kind != doc.Object {
		return nil, LegacyHeader(name)
	}
	bad := func(field string) error {
		return refuse(name+": "+MetaKey+"/"+field+" is invalid (legacy or malformed); "+RepairHint, ErrLegacyHeader)
	}
	e := &Meta{Sync: map[string]SyncEntry{}}
	for _, k := range mv.Keys {
		v := mv.M[k]
		switch k {
		case "format":
			n, err := parseGeneration(name, v)
			if err != nil {
				return nil, err
			}
			e.Format = n
		case "publicKey":
			if v.Kind != doc.String {
				return nil, bad(k)
			}
			e.PublicKey = v.S
		case "keyId":
			if v.Kind != doc.String || !crypt.ValidKeyID(v.S) {
				return nil, bad(k)
			}
			e.KeyID = v.S
		case "home":
			if v.Kind != doc.String {
				return nil, bad(k)
			}
			e.Home = v.S
		case "schema":
			switch v.Kind {
			case doc.String:
				e.Schema = v.S
			case doc.Object:
				p, c := v.Get("path"), v.Get("c")
				if p == nil || c == nil || p.Kind != doc.String || c.Kind != doc.String || !crypt.IsCiphertext(c.S) || len(v.Keys) != 2 {
					return nil, bad(k)
				}
				e.Schema, e.SchemaCopy = p.S, c.S
			default:
				return nil, bad(k)
			}
		case "sync":
			if v.Kind != doc.Object {
				return nil, bad(k)
			}
			for _, p := range v.Keys {
				ent := v.M[p]
				if _, err := doc.ParsePointer(p); err != nil || ent.Kind != doc.Object {
					return nil, bad(k)
				}
				c, h := ent.Get("c"), ent.Get("h")
				if c == nil || h == nil || c.Kind != doc.String || h.Kind != doc.String || !tagRe.MatchString(c.S) || !tagRe.MatchString(h.S) || len(ent.Keys) != 2 {
					return nil, bad(k)
				}
				e.Sync[p] = SyncEntry{C: c.S, H: h.S}
			}
		case "pending":
			if v.Kind != doc.Array {
				return nil, bad(k)
			}
			for _, x := range v.A {
				if x.Kind != doc.String {
					return nil, bad(k)
				}
				if _, err := doc.ParsePointer(x.S); err != nil {
					return nil, bad(k)
				}
				e.Pending = append(e.Pending, x.S)
			}
		default:
			return nil, refuse(name+": unknown field "+MetaKey+"/"+k+"; "+RepairHint, ErrLegacyHeader)
		}
	}
	if e.Format == 0 {
		return nil, bad("format")
	}
	if e.PublicKey == "" || e.KeyID == "" {
		return nil, bad("publicKey")
	}
	pk, err := crypt.ParsePublicKey(e.PublicKey)
	if err != nil {
		return nil, bad("publicKey")
	}
	if crypt.KeyID(pk) != e.KeyID {
		return nil, xerr.Key("%s: %s/keyId does not match %s/publicKey", name, MetaKey, MetaKey)
	}
	return e, nil
}

// MarshalEncrypted renders e with $zeze first and sorted sync/pending. The
// block always carries FormatVersion and the home reference; the schema
// reference is the plain path, or { path, c } when an embedded schema copy is
// present.
func MarshalEncrypted(e *Encrypted) []byte {
	m := doc.NewObject()
	putIdentity(m, e.Meta.Identity())
	if e.Meta.Schema != "" || e.Meta.SchemaCopy != "" {
		if e.Meta.SchemaCopy == "" {
			m.Put("schema", doc.NewString(e.Meta.Schema))
		} else {
			s := doc.NewObject()
			s.Put("path", doc.NewString(e.Meta.Schema))
			s.Put("c", doc.NewString(e.Meta.SchemaCopy))
			m.Put("schema", s)
		}
	}
	sync := doc.NewObject()
	keys := make([]string, 0, len(e.Meta.Sync))
	for k := range e.Meta.Sync {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		ent := doc.NewObject()
		ent.Put("c", doc.NewString(e.Meta.Sync[k].C))
		ent.Put("h", doc.NewString(e.Meta.Sync[k].H))
		sync.Put(k, ent)
	}
	m.Put("sync", sync)
	pending := doc.NewArray()
	for _, p := range SortedUnique(e.Meta.Pending) {
		pending.A = append(pending.A, doc.NewString(p))
	}
	m.Put("pending", pending)
	root := doc.NewObject()
	root.Put(MetaKey, m)
	if e.Body != nil {
		for _, k := range e.Body.Keys {
			if k == MetaKey {
				continue
			}
			root.Put(k, e.Body.M[k])
		}
	}
	return doc.Marshal(root)
}

// WriteEncrypted writes e atomically ($zeze first, sorted sync/pending).
// It refuses to mutate a pair whose files carry old zeze state; the typed
// refusal points at zeze init, and RepairEncrypted is the write path init
// uses to replace such files.
func WriteEncrypted(path string, e *Encrypted) error {
	if err := CheckEncryptedName(path); err != nil {
		return err
	}
	if err := checkMutable(false, path); err != nil {
		return err
	}
	return writeEncrypted(path, e)
}

// RepairEncrypted writes e like WriteEncrypted but is the zeze init repair
// path: it may replace a pair file whose $zeze block is format 1 or
// malformed. An unknown future format is still refused.
func RepairEncrypted(path string, e *Encrypted) error {
	if err := CheckEncryptedName(path); err != nil {
		return err
	}
	if err := checkMutable(true, path); err != nil {
		return err
	}
	return writeEncrypted(path, e)
}

func writeEncrypted(path string, e *Encrypted) error {
	data := MarshalEncrypted(e)
	if len(data) > maxFileSize {
		return tooLarge(path)
	}
	if err := paths.AtomicWrite(path, data, 0o644); err != nil {
		return xerr.Failure("%s: cannot write file", filepath.Base(path))
	}
	return nil
}

// tooLarge refuses a write that zeze could not read back.
func tooLarge(path string) error {
	return xerr.Validation("%s: the result would be larger than the %d MiB zeze can read; nothing was written", filepath.Base(path), maxFileSize>>20)
}

// readErr maps a readLimited failure to a message without content.
func readErr(name string, err error) error {
	if errors.Is(err, errTooLarge) {
		return xerr.Failure("%s: file too large (zeze reads at most %d MiB)", name, maxFileSize>>20)
	}
	return xerr.Failure("%s: cannot read file", name)
}

// SortedUnique returns a sorted copy of s without duplicates.
func SortedUnique(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	j := 0
	for i, v := range out {
		if i > 0 && v == out[j-1] {
			continue
		}
		out[j] = v
		j++
	}
	return out[:j]
}

// ReadPlain parses an unencrypted file (JSONC when jsonc is true). The top
// level must be an object without the reserved $zeze key.
func ReadPlain(path string, jsonc_ bool) (*doc.Value, error) {
	name := filepath.Base(path)
	data, err := readLimited(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &xerr.Error{Code: xerr.CodeFailure, Kind: xerr.KindFailure, Msg: name + ": not found", Err: fs.ErrNotExist}
		}
		return nil, readErr(name, err)
	}
	v, err := jsonc.Parse(data, !jsonc_)
	if err != nil {
		return nil, xerr.Validation("%s: %v", name, err)
	}
	if v.Kind != doc.Object {
		return nil, xerr.Validation("%s: top level must be an object", name)
	}
	if v.Get(MetaKey) != nil {
		return nil, xerr.Validation("%s: the top-level key %s is reserved", name, MetaKey)
	}
	return v, nil
}

// WritePlain writes an unencrypted file with mode 0600, exactly the document
// it is given: no $zeze header or $schema pointer is injected and nothing
// in the document is stripped. It refuses to mutate a pair whose files carry
// old zeze state (RepairPlainFile is the zeze init repair path). A .json
// target is written as canonical JSON. A .jsonc target with existing content
// is updated in place with the comment-preserving editor (comments and
// formatting outside the edited spans survive; unparseable or unrepresentable
// content is refused, never rewritten); a missing .jsonc target is created as
// canonical JSON (valid JSON is valid JSONC). It refuses targets that git
// would not ignore (after adding the managed block) and targets tracked by
// git.
func WritePlain(path string, v *doc.Value) error {
	name := filepath.Base(path)
	if err := CheckUnencryptedName(path); err != nil {
		return err
	}
	if v.Kind != doc.Object || v.Get(MetaKey) != nil {
		return xerr.Failure("%s: refusing to write an invalid document", name)
	}
	return writePlainDoc(path, v, false)
}

// PlainFile is a parsed unencrypted file: its $zeze identity header, the
// top-level $schema pointer, and the body without either reserved key.
type PlainFile struct {
	// Header is nil when the file carries no $zeze block (a headerless
	// plaintext is legal and stays mutable).
	Header *Header
	// Schema is the top-level $schema pointer value, "" when absent.
	Schema string
	// Body never contains $zeze or $schema.
	Body *doc.Value
}

// ReadPlainFile parses an unencrypted file (JSONC when jsonc is true) with
// its identity metadata: the $zeze header and the top-level $schema
// pointer are returned separately and excluded from Body. A header with
// format 1 parses and records Format 1 (legacy: reads keep working); an
// unknown future format is rejected explicitly; a present-but-malformed
// header returns the typed legacy refusal and a keyId that contradicts
// publicKey fails with exit 5.
func ReadPlainFile(path string, jsonc_ bool) (*PlainFile, error) {
	name := filepath.Base(path)
	data, err := readLimited(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &xerr.Error{Code: xerr.CodeFailure, Kind: xerr.KindFailure, Msg: name + ": not found", Err: fs.ErrNotExist}
		}
		return nil, readErr(name, err)
	}
	v, err := jsonc.Parse(data, !jsonc_)
	if err != nil {
		return nil, xerr.Validation("%s: %v", name, err)
	}
	if v.Kind != doc.Object {
		return nil, xerr.Validation("%s: top level must be an object", name)
	}
	f := &PlainFile{Body: doc.NewObject()}
	if mv := v.Get(MetaKey); mv != nil {
		h, err := parseHeader(name, mv)
		if err != nil {
			return nil, err
		}
		f.Header = h
	}
	if sv := v.Get(SchemaKey); sv != nil {
		if sv.Kind != doc.String {
			return nil, xerr.Validation("%s: %s must be a string", name, SchemaKey)
		}
		f.Schema = sv.S
	}
	for _, k := range v.Keys {
		if k == MetaKey || k == SchemaKey {
			continue
		}
		f.Body.Put(k, v.M[k])
	}
	return f, nil
}

// WritePlainFile writes an unencrypted file from its identity metadata: the
// $zeze header (when set) first, then the $schema pointer (when set), then
// the body. Body must not contain the reserved keys. Like WritePlain it
// composes .jsonc targets through the comment-preserving editor and refuses
// targets git would not ignore; it refuses to mutate a pair whose files carry
// old zeze state, and RepairPlainFile is the write path zeze init uses
// to replace such files.
func WritePlainFile(path string, f *PlainFile) error {
	return writePlainFile(path, f, false)
}

// RepairPlainFile writes f like WritePlainFile but is the zeze init repair
// path: it may replace a pair file whose $zeze block is format 1 or
// malformed. An unknown future format is still refused.
func RepairPlainFile(path string, f *PlainFile) error {
	return writePlainFile(path, f, true)
}

// writePlainFile validates the metadata and composes the document for
// writePlainDoc. repair allows old zeze state through (see checkMutable).
func writePlainFile(path string, f *PlainFile, repair bool) error {
	name := filepath.Base(path)
	if err := CheckUnencryptedName(path); err != nil {
		return err
	}
	if f == nil || f.Body == nil || f.Body.Kind != doc.Object || f.Body.Get(MetaKey) != nil || f.Body.Get(SchemaKey) != nil {
		return xerr.Failure("%s: refusing to write an invalid document", name)
	}
	root := doc.NewObject()
	if f.Header != nil {
		root.Put(MetaKey, marshalHeader(f.Header))
	}
	if f.Schema != "" {
		root.Put(SchemaKey, doc.NewString(f.Schema))
	}
	for _, k := range f.Body.Keys {
		root.Put(k, f.Body.M[k])
	}
	return writePlainDoc(path, root, repair)
}

// writePlainDoc writes the composed document through the $zeze gate and
// the git ignore and tracking rules, then the file itself.
func writePlainDoc(path string, root *doc.Value, repair bool) error {
	name := filepath.Base(path)
	if err := checkMutable(repair, path); err != nil {
		return err
	}
	dir, err := paths.ProjectRoot(filepath.Dir(path))
	if err != nil {
		return xerr.Failure("%s: cannot resolve the project root", name)
	}
	if err := EnsureIgnored(dir, path); err != nil {
		return err
	}
	tracked, err := TrackedByGit(dir, path)
	if err != nil {
		return err
	}
	if tracked {
		return xerr.Failure("%s: refusing to write a file that is tracked by git (run git rm --cached %s)", name, name)
	}
	data, err := plainBytes(path, root)
	if err != nil {
		return err
	}
	defer clear(data)
	if len(data) > maxFileSize {
		return tooLarge(path)
	}
	if err := paths.AtomicWrite(path, data, 0o600); err != nil {
		return xerr.Failure("%s: cannot write file", name)
	}
	return nil
}

// plainBytes renders v for path: canonical JSON for .json targets and missing
// files, a comment-preserving merge for an existing .jsonc target.
func plainBytes(path string, v *doc.Value) ([]byte, error) {
	name := filepath.Base(path)
	if !strings.EqualFold(filepath.Ext(path), ".jsonc") {
		return doc.Marshal(v), nil
	}
	orig, err := readLimited(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return doc.Marshal(v), nil
		}
		return nil, readErr(name, err)
	}
	current, err := jsonc.Parse(orig, false)
	if err != nil {
		return nil, xerr.Validation("%s: %v (fix it by hand; zeze will not rewrite it)", name, err)
	}
	data, err := jsonc.Edit(orig, current, v)
	if err != nil {
		return nil, xerr.Failure("%s: %v; nothing was written", name, err)
	}
	return data, nil
}
