package pair

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/CGuiho/jureuma/internal/crypt"
	"github.com/CGuiho/jureuma/internal/doc"
	"github.com/CGuiho/jureuma/internal/jsonc"
	"github.com/CGuiho/jureuma/internal/paths"
	"github.com/CGuiho/jureuma/internal/xerr"
)

// SchemaKey is the reserved top-level key of the unencrypted file that points
// at the local schema, in the "./x.schema.json" form.
const SchemaKey = "$schema"

// HomeRef is the global directory reference stored in $jureuma/home, in
// conventional portable form. It is never a machine-specific absolute path.
const HomeRef = "~/.guiho/" + paths.CLIName + "/"

// SchemaCopyPointer is the JSON Pointer the embedded schema copy envelope is
// bound to; opening the copy needs exactly this pointer.
const SchemaCopyPointer = "/$jureuma/schema/c"

// RepairHint names the command that repairs pair files. Every refusal caused
// by old pair state ends with it.
const RepairHint = "run jureuma init to repair"

// Typed refusals. Messages built from them carry file names and fixed phrases
// only, never configuration values or key material.
var (
	// ErrLegacyHeader marks a pair file whose $jureuma block is old jureuma
	// state: format 1, or present but malformed. Reads keep working where
	// they can; mutations refuse and point at jureuma init.
	ErrLegacyHeader = errors.New("legacy " + MetaKey + " header")
	// ErrUnsupportedFormat marks a $jureuma block with a format generation
	// this jureuma does not know. Reads and writes reject it explicitly.
	ErrUnsupportedFormat = errors.New("unsupported " + MetaKey + " format")
	// ErrMissingPlainIdentity marks an unencrypted file without the
	// $jureuma identity a single-file flow needs to rebuild its sibling.
	ErrMissingPlainIdentity = errors.New("missing " + MetaKey + " identity")
)

// refusal is a typed pair refusal: an exit-1 error whose message is exactly
// what its constructor built (file name, class and fixed phrases, never a
// value) and whose class is detectable with errors.Is.
type refusal struct{ inner *xerr.Error }

func (r *refusal) Error() string { return r.inner.Msg }
func (r *refusal) Unwrap() error { return r.inner }

func refuse(msg string, class error) error {
	return &refusal{&xerr.Error{Code: xerr.CodeFailure, Kind: xerr.KindFailure, Msg: msg, Err: class}}
}

// LegacyHeader returns the typed, value-free refusal for a pair file whose
// $jureuma block is format 1 or malformed.
func LegacyHeader(file string) error {
	return refuse(file+": legacy "+MetaKey+" header (format 1 or malformed); "+RepairHint, ErrLegacyHeader)
}

// UnsupportedFormat returns the explicit rejection for a $jureuma format
// generation outside the ones this jureuma reads.
func UnsupportedFormat(file string) error {
	return refuse(file+": unsupported "+MetaKey+"/format (this jureuma writes format 2 and reads formats 1 and 2)", ErrUnsupportedFormat)
}

// MissingPlainIdentity returns the typed repair refusal for an unencrypted
// file with no $jureuma identity. Single-file identity paths return it when
// they cannot rebuild the sibling without the pair key.
func MissingPlainIdentity(file string) error {
	return refuse(file+": no "+MetaKey+" identity; "+RepairHint, ErrMissingPlainIdentity)
}

// Header is the pair identity both files carry under the reserved $jureuma
// key: the format generation, the pair public key, its key ID and the global
// directory reference. The unencrypted header is exactly this; the encrypted
// $jureuma block carries it plus the schema, sync and pending state.
type Header struct {
	Format           int
	PublicKey, KeyID string
	Home             string
}

// PublicKeyValue parses Header.PublicKey.
func (h Header) PublicKeyValue() (crypt.PublicKey, error) {
	return crypt.ParsePublicKey(h.PublicKey)
}

// Identity returns the identity part of the encrypted meta block.
func (m Meta) Identity() Header {
	return Header{Format: m.Format, PublicKey: m.PublicKey, KeyID: m.KeyID, Home: m.Home}
}

// formatValue renders the format generation this jureuma writes. Reads accept
// format 1 (legacy) and 2; every write renders FormatVersion, so the Format
// fields of Header and Meta record the generation that was read.
func formatValue() *doc.Value {
	n, _ := doc.NewNumber(strconv.Itoa(FormatVersion))
	return n
}

// putIdentity writes the shared identity fields in their fixed order.
func putIdentity(m *doc.Value, h Header) {
	m.Put("format", formatValue())
	m.Put("publicKey", doc.NewString(h.PublicKey))
	m.Put("keyId", doc.NewString(h.KeyID))
	m.Put("home", doc.NewString(h.Home))
}

// parseGeneration reads $jureuma/format: "1" (legacy) or "2". Any other
// number is an unknown future generation; a missing or non-number field is a
// malformed header.
func parseGeneration(name string, v *doc.Value) (int, error) {
	if v == nil || v.Kind != doc.Number {
		return 0, LegacyHeader(name)
	}
	switch v.N {
	case "1":
		return 1, nil
	case "2":
		return FormatVersion, nil
	default:
		return 0, UnsupportedFormat(name)
	}
}

// parseHeader parses the $jureuma identity block of an unencrypted file. The
// block carries exactly format, publicKey, keyId and home; anything else is a
// present-but-malformed header. A keyId that contradicts publicKey is the
// key mismatch rule and fails with exit 5.
func parseHeader(name string, mv *doc.Value) (*Header, error) {
	if mv == nil || mv.Kind != doc.Object {
		return nil, LegacyHeader(name)
	}
	h := &Header{}
	for _, k := range mv.Keys {
		v := mv.M[k]
		switch k {
		case "format":
			n, err := parseGeneration(name, v)
			if err != nil {
				return nil, err
			}
			h.Format = n
		case "publicKey":
			if v.Kind != doc.String {
				return nil, LegacyHeader(name)
			}
			h.PublicKey = v.S
		case "keyId":
			if v.Kind != doc.String || !crypt.ValidKeyID(v.S) {
				return nil, LegacyHeader(name)
			}
			h.KeyID = v.S
		case "home":
			if v.Kind != doc.String {
				return nil, LegacyHeader(name)
			}
			h.Home = v.S
		default:
			return nil, LegacyHeader(name)
		}
	}
	if h.Format == 0 || h.PublicKey == "" || h.KeyID == "" {
		return nil, LegacyHeader(name)
	}
	pk, err := crypt.ParsePublicKey(h.PublicKey)
	if err != nil {
		return nil, LegacyHeader(name)
	}
	if crypt.KeyID(pk) != h.KeyID {
		return nil, xerr.Key("%s: %s/keyId does not match %s/publicKey", name, MetaKey, MetaKey)
	}
	return h, nil
}

// marshalHeader renders the $jureuma identity block of an unencrypted file.
// The result always carries FormatVersion; h.Format records the generation
// that was read.
func marshalHeader(h *Header) *doc.Value {
	m := doc.NewObject()
	putIdentity(m, *h)
	return m
}

// SetSchemaCopy stores the schema document encrypted to the pair key as a
// single envelope in $jureuma.schema, turning the schema reference into the
// { path, c } form. Encryption needs only the public key.
func (e *Encrypted) SetSchemaCopy(schemaText []byte) error {
	pk, err := e.Meta.PublicKeyValue()
	if err != nil {
		return xerr.Failure("%s/publicKey is invalid", MetaKey)
	}
	ct, err := crypt.EncryptValue(pk, SchemaCopyPointer, schemaText)
	if err != nil {
		return xerr.Failure("%s/schema: encryption failed", MetaKey)
	}
	e.Meta.SchemaCopy = ct
	return nil
}

// OpenSchemaCopy decrypts the embedded schema copy and returns the schema
// document text. keys resolves the key the copy was encrypted to, so a copy
// under a retired key can still be opened by a key holder.
func (e *Encrypted) OpenSchemaCopy(keys KeyLookup) ([]byte, error) {
	if e.Meta.SchemaCopy == "" {
		return nil, xerr.Failure("%s/schema: no embedded schema copy", MetaKey)
	}
	id, ok := crypt.CiphertextKeyID(e.Meta.SchemaCopy)
	if !ok {
		return nil, xerr.Failure("%s/schema: the embedded schema copy is invalid", MetaKey)
	}
	sk, err := keys(id)
	if err != nil {
		return nil, xerr.Key("%s/schema: private key %s not available", MetaKey, id)
	}
	text, err := crypt.DecryptValue(sk, SchemaCopyPointer, e.Meta.SchemaCopy)
	if err != nil {
		return nil, xerr.Key("%s/schema: decryption failed", MetaKey)
	}
	return text, nil
}

// CheckIdentity cross-checks the identity both files carry. A publicKey or
// keyId disagreement is fatal with exit 5: one pair belongs to exactly one
// key. Differences in the home reference and the schema path are returned as
// warnings and never fail. Either side may be absent (single-file states) or
// carry no plaintext header (a headerless plaintext is legal).
func CheckIdentity(plain *PlainFile, enc *Encrypted) (warnings []string, err error) {
	if plain == nil || plain.Header == nil || enc == nil {
		return nil, nil
	}
	h, m := plain.Header, enc.Meta
	if h.PublicKey != m.PublicKey || h.KeyID != m.KeyID {
		return nil, xerr.Key("the two files disagree on the pair key (%s/publicKey, %s/keyId)", MetaKey, MetaKey)
	}
	if h.Home != m.Home {
		warnings = append(warnings, "the two files record different "+MetaKey+"/home values")
	}
	if plain.Schema != m.Schema {
		warnings = append(warnings, "the two files reference different schemas")
	}
	return warnings, nil
}

// checkMutable refuses a mutation through the pair files named by paths when
// the pair carries old jureuma state. The refusal keys on format state, never
// on plaintext header absence: a $jureuma block with format 1, or a
// present-but-malformed block, on either side is refused with the init repair
// pointer, while a headerless plaintext stays fully mutable. A $jureuma
// format outside the ones this jureuma reads is always refused, even by the
// repair path: jureuma never overwrites a file it cannot read. repair lets
// format 1 and malformed blocks through, because the jureuma init repair path
// is what replaces them.
func checkMutable(repair bool, paths ...string) error {
	for _, path := range paths {
		for _, p := range append([]string{path}, otherSide(path)...) {
			if err := checkFileState(p, repair); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkFileState classifies the $jureuma state of one pair file's content.
// Content jureuma cannot parse at all has no header it can see and keeps the
// overwrite behaviour it always had.
func checkFileState(path string, repair bool) error {
	name := filepath.Base(path)
	data, err := readLimited(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return readErr(name, err)
	}
	root, err := jsonc.Parse(data, false)
	if err != nil || root.Kind != doc.Object {
		return nil
	}
	mv := root.Get(MetaKey)
	if mv == nil {
		return nil
	}
	var perr error
	if strings.Contains(name, EncryptedMarker) {
		var m *Meta
		m, perr = parseMeta(name, mv)
		if perr == nil && m.Format != FormatVersion {
			perr = LegacyHeader(name)
		}
	} else {
		var h *Header
		h, perr = parseHeader(name, mv)
		if perr == nil && h.Format != FormatVersion {
			perr = LegacyHeader(name)
		}
	}
	switch {
	case perr == nil:
		return nil
	case errors.Is(perr, ErrUnsupportedFormat) || xerr.CodeOf(perr) == xerr.CodeKey:
		return perr
	case repair:
		return nil
	default:
		return LegacyHeader(name)
	}
}

// otherSide returns the pair file paths on the other side of path, whether or
// not they exist: the unencrypted siblings of an encrypted file, or the
// encrypted sibling of an unencrypted file.
func otherSide(path string) []string {
	base := filepath.Base(path)
	dir := filepath.Dir(path)
	if stem := stemOf(path); stem != "" {
		return []string{stem + UnencryptedMarker + "json", stem + UnencryptedMarker + "jsonc"}
	}
	i := strings.LastIndex(base, UnencryptedMarker)
	if i < 0 {
		return nil
	}
	rest := base[i+len(UnencryptedMarker):]
	if strings.EqualFold(filepath.Ext(base), ".jsonc") {
		rest = strings.TrimSuffix(rest, filepath.Ext(rest)) + "json"
	}
	return []string{filepath.Join(dir, base[:i]+EncryptedMarker+rest)}
}
