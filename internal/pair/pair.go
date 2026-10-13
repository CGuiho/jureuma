// Package pair resolves a jureuma configuration pair (unencrypted file,
// encrypted file, schema) and reads, writes, encrypts, decrypts and compares
// its two sides.
//
// No error produced here contains a configuration value, plaintext, or key
// material: messages name files, JSON Pointers and fixed reasons only.
package pair

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/CGuiho/jureuma/internal/config"
	"github.com/CGuiho/jureuma/internal/doc"
	"github.com/CGuiho/jureuma/internal/jsonc"
	"github.com/CGuiho/jureuma/internal/xerr"
)

// DefaultName is the stem used when no pair is selected or configured.
const DefaultName = "config"

// Markers that make the state of a file obvious from its name.
const (
	EncryptedMarker   = ".encrypted."
	UnencryptedMarker = ".unencrypted."
)

// Schema sources, in precedence order.
const (
	SchemaFromFlag      = "flag"
	SchemaFromConfig    = "jureuma.yaml"
	SchemaFromEncrypted = "$jureuma.schema"
	SchemaFromDefault   = "default"
)

// Options selects a pair. Paths in EncryptedFile, UnencryptedFile and
// SchemaFile are relative to the current directory (or absolute); Root is the
// project root (absolute) that pair names and jureuma.yaml paths are relative to.
type Options struct {
	Root, Name, EncryptedFile, UnencryptedFile, SchemaFile string
	Config                                                 *config.Project
}

// Pair is a resolved pair. All paths are absolute.
type Pair struct {
	Name, EncryptedPath, UnencryptedPath, SchemaPath string
	UnencryptedIsJSONC                               bool

	// Root is the project root the pair was resolved against.
	Root string
	// SchemaSource names which precedence rule chose SchemaPath.
	SchemaSource string
	// ConfiguredSchema is the schema chosen by jureuma.yaml or $jureuma.schema
	// (sources 2-3), or "" when neither is set. Used to reject a disagreeing
	// --schema-file on mutating commands.
	ConfiguredSchema string
	// DefaultSchema is the source-4 schema (<stem>.schema.json or
	// .schema.jsonc beside the encrypted file) when that file exists, else "".
	// A --schema-file that disagrees with it is rejected like one that
	// disagrees with ConfiguredSchema.
	DefaultSchema string
}

var driveRe = regexp.MustCompile(`^[A-Za-z]:`)

// ValidName reports whether name is a usable pair stem: relative, non-empty,
// no ".." segment.
func ValidName(name string) bool {
	if name == "" || strings.HasPrefix(name, "/") || strings.HasPrefix(name, `\`) || driveRe.MatchString(name) {
		return false
	}
	if strings.HasSuffix(name, "/") || strings.HasSuffix(name, `\`) {
		return false
	}
	for _, seg := range strings.FieldsFunc(name, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." || seg == "." {
			return false
		}
	}
	return true
}

func absFrom(base, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(base, filepath.FromSlash(p))
}

// Within reports whether path stays inside root, both lexically and after
// resolving symbolic links of the deepest existing ancestor of path.
func Within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return false
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return true // the root itself cannot be resolved; the lexical check holds
	}
	// Resolve the deepest existing ancestor (the path itself when it exists).
	dir, rest := path, ""
	for {
		if r, err := filepath.EvalSymlinks(dir); err == nil {
			full := filepath.Join(r, rest)
			rel, err := filepath.Rel(realRoot, full)
			return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return true
		}
		rest = filepath.Join(filepath.Base(dir), rest)
		dir = parent
	}
}

// CheckSchemaName enforces the schema naming rule: the file name ends with
// .schema.json or .schema.jsonc.
func CheckSchemaName(path string) error {
	base := strings.ToLower(filepath.Base(path))
	if strings.HasSuffix(base, ".schema.json") || strings.HasSuffix(base, ".schema.jsonc") {
		return nil
	}
	return xerr.Usage("%s: a schema file name must end with .schema.json or .schema.jsonc", filepath.Base(path))
}

// confined returns a usage error naming field when path (taken from a
// committed file, not the command line) leaves the project root.
func confined(root, path, field string) error {
	if !Within(root, path) {
		return xerr.Usage("%s points outside the project root; only command-line flags may name files outside it", field)
	}
	return nil
}

func cwdAbs(p string) (string, error) {
	a, err := filepath.Abs(p)
	if err != nil {
		return "", xerr.Usage("cannot resolve path %s", p)
	}
	return a, nil
}

// CheckEncryptedName enforces the .encrypted. marker and the .json extension.
func CheckEncryptedName(path string) error {
	base := filepath.Base(path)
	if !strings.Contains(base, EncryptedMarker) {
		return xerr.Usage("%s: an encrypted file name must contain %q", base, EncryptedMarker)
	}
	if strings.ToLower(filepath.Ext(base)) != ".json" {
		return xerr.Usage("%s: an encrypted file must use the .json extension", base)
	}
	return nil
}

// CheckUnencryptedName enforces the .unencrypted. marker and .json/.jsonc.
func CheckUnencryptedName(path string) error {
	base := filepath.Base(path)
	if !strings.Contains(base, UnencryptedMarker) {
		return xerr.Usage("%s: an unencrypted file name must contain %q", base, UnencryptedMarker)
	}
	switch strings.ToLower(filepath.Ext(base)) {
	case ".json", ".jsonc":
		return nil
	}
	return xerr.Usage("%s: an unencrypted file must use the .json or .jsonc extension", base)
}

// stemOf returns the encrypted path without its ".encrypted.json" suffix, or
// "" when the name does not end that way.
func stemOf(encPath string) string {
	const suf = ".encrypted.json"
	if strings.HasSuffix(strings.ToLower(encPath), suf) {
		return encPath[:len(encPath)-len(suf)]
	}
	return ""
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// Resolve applies the naming rules and the schema precedence.
func Resolve(o Options) (*Pair, error) {
	root := o.Root
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, xerr.Failure("cannot determine the current directory")
		}
		root = wd
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, xerr.Failure("cannot resolve the project root")
	}

	name := o.Name
	if name != "" && !ValidName(name) {
		return nil, xerr.Usage("--name %q: a pair name must be a relative path without '..'", name)
	}
	var pc *config.PairConfig
	if o.Config != nil {
		if name == "" && o.EncryptedFile == "" && o.UnencryptedFile == "" && len(o.Config.Pairs) > 0 {
			name = o.Config.Pairs[0].Name
		}
		for i := range o.Config.Pairs {
			if filepath.ToSlash(o.Config.Pairs[i].Name) == filepath.ToSlash(name) && name != "" {
				pc = &o.Config.Pairs[i]
				break
			}
		}
	}
	if name == "" && o.EncryptedFile == "" && o.UnencryptedFile == "" {
		name = DefaultName
	}

	p := &Pair{Root: root}

	// Encrypted side.
	switch {
	case o.EncryptedFile != "":
		if p.EncryptedPath, err = cwdAbs(o.EncryptedFile); err != nil {
			return nil, err
		}
	case pc != nil && pc.Encrypted != "":
		p.EncryptedPath = absFrom(root, pc.Encrypted)
		if err := confined(root, p.EncryptedPath, config.ProjectFile+" pairs[].encrypted"); err != nil {
			return nil, err
		}
	case name != "":
		p.EncryptedPath = absFrom(root, name+".encrypted.json")
		if err := confined(root, p.EncryptedPath, "the pair name"); err != nil {
			return nil, err
		}
	default:
		// Only --unencrypted-file given: derive the encrypted side from it.
		u, err := cwdAbs(o.UnencryptedFile)
		if err != nil {
			return nil, err
		}
		if err := CheckUnencryptedName(u); err != nil {
			return nil, err
		}
		dir, base := filepath.Split(u)
		i := strings.LastIndex(base, UnencryptedMarker)
		rest := base[i+len(UnencryptedMarker):]
		if strings.EqualFold(filepath.Ext(base), ".jsonc") {
			rest = rest[:len(rest)-len("jsonc")] + "json"
		}
		p.EncryptedPath = filepath.Join(dir, base[:i]+EncryptedMarker+rest)
	}
	if err := CheckEncryptedName(p.EncryptedPath); err != nil {
		return nil, err
	}
	stem := stemOf(p.EncryptedPath)

	// Name.
	switch {
	case name != "":
		p.Name = filepath.ToSlash(name)
	case stem != "":
		if rel, err := filepath.Rel(root, stem); err == nil && !strings.HasPrefix(rel, "..") {
			p.Name = filepath.ToSlash(rel)
		} else {
			p.Name = filepath.Base(stem)
		}
	default:
		p.Name = filepath.Base(p.EncryptedPath)
	}

	// Unencrypted side.
	switch {
	case o.UnencryptedFile != "":
		if p.UnencryptedPath, err = cwdAbs(o.UnencryptedFile); err != nil {
			return nil, err
		}
	case pc != nil && pc.Unencrypted != "":
		p.UnencryptedPath = absFrom(root, pc.Unencrypted)
		if err := confined(root, p.UnencryptedPath, config.ProjectFile+" pairs[].unencrypted"); err != nil {
			return nil, err
		}
	default:
		dir, base := filepath.Split(p.EncryptedPath)
		i := strings.LastIndex(base, EncryptedMarker)
		jsonPath := filepath.Join(dir, base[:i]+UnencryptedMarker+base[i+len(EncryptedMarker):])
		jsoncPath := strings.TrimSuffix(jsonPath, filepath.Ext(jsonPath)) + ".jsonc"
		hasJSON, hasJSONC := exists(jsonPath), exists(jsoncPath)
		if hasJSON && hasJSONC {
			return nil, xerr.Usage("both %s and %s exist; keep only one", filepath.Base(jsonPath), filepath.Base(jsoncPath))
		}
		p.UnencryptedPath = jsonPath
		if hasJSONC {
			p.UnencryptedPath = jsoncPath
		}
	}
	if err := CheckUnencryptedName(p.UnencryptedPath); err != nil {
		return nil, err
	}
	p.UnencryptedIsJSONC = strings.EqualFold(filepath.Ext(p.UnencryptedPath), ".jsonc")

	// Schema precedence.
	var configured, fromFile string
	if pc != nil && pc.Schema != "" {
		configured = absFrom(root, pc.Schema)
		if err := confined(root, configured, config.ProjectFile+" pairs[].schema"); err != nil {
			return nil, err
		}
		if err := CheckSchemaName(configured); err != nil {
			return nil, err
		}
	}
	if s := readMetaSchema(p.EncryptedPath); s != "" {
		fromFile = absFrom(filepath.Dir(p.EncryptedPath), s)
		if err := confined(root, fromFile, p.Rel(p.EncryptedPath)+" $jureuma.schema"); err != nil {
			return nil, err
		}
		if err := CheckSchemaName(fromFile); err != nil {
			return nil, err
		}
	}
	if configured != "" {
		p.ConfiguredSchema = configured
	} else {
		p.ConfiguredSchema = fromFile
	}
	defBase := stem
	if defBase == "" {
		defBase = strings.TrimSuffix(p.EncryptedPath, filepath.Ext(p.EncryptedPath))
	}
	switch {
	case exists(defBase + ".schema.json"):
		p.DefaultSchema = defBase + ".schema.json"
	case exists(defBase + ".schema.jsonc"):
		p.DefaultSchema = defBase + ".schema.jsonc"
	}
	switch {
	case o.SchemaFile != "":
		if p.SchemaPath, err = cwdAbs(o.SchemaFile); err != nil {
			return nil, err
		}
		p.SchemaSource = SchemaFromFlag
	case configured != "":
		p.SchemaPath, p.SchemaSource = configured, SchemaFromConfig
	case fromFile != "":
		p.SchemaPath, p.SchemaSource = fromFile, SchemaFromEncrypted
	default:
		base := stem
		if base == "" {
			base = strings.TrimSuffix(p.EncryptedPath, filepath.Ext(p.EncryptedPath))
		}
		p.SchemaPath = base + ".schema.json"
		if !exists(p.SchemaPath) && exists(base+".schema.jsonc") {
			p.SchemaPath = base + ".schema.jsonc"
		}
		p.SchemaSource = SchemaFromDefault
	}
	return p, nil
}

// CheckSchemaFlag returns a usage error when --schema-file disagrees with the
// schema configured in jureuma.yaml or $jureuma.schema or, when neither is
// set, with the default <stem>.schema.json(c) if that file exists. Mutating
// commands call it, so a write cannot be validated against a looser schema.
func (p *Pair) CheckSchemaFlag() error {
	if p.SchemaSource != SchemaFromFlag {
		return nil
	}
	want := p.ConfiguredSchema
	if want == "" {
		want = p.DefaultSchema
	}
	if want == "" {
		return nil
	}
	if !samePath(p.SchemaPath, want) {
		return xerr.Usage("--schema-file disagrees with the configured schema %s", p.Rel(want))
	}
	return nil
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if a == b {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}

// Rel returns path relative to the pair's root when possible, for messages.
func (p *Pair) Rel(path string) string {
	if rel, err := filepath.Rel(p.Root, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return path
}

// SchemaRef returns the schema path relative to the encrypted file's
// directory, in the "./x.schema.json" form stored in $jureuma.schema.
func (p *Pair) SchemaRef() string {
	rel, err := filepath.Rel(filepath.Dir(p.EncryptedPath), p.SchemaPath)
	if err != nil {
		return filepath.ToSlash(p.SchemaPath)
	}
	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, "../") {
		rel = "./" + rel
	}
	return rel
}

// readMetaSchema returns $jureuma.schema from an encrypted file, or "".
// Any problem is ignored here; ReadEncrypted reports it.
func readMetaSchema(path string) string {
	data, err := readLimited(path)
	if err != nil {
		return ""
	}
	v, err := jsonc.Parse(data, true)
	if err != nil {
		return ""
	}
	m := v.Get(MetaKey)
	if m == nil {
		return ""
	}
	s := m.Get("schema")
	if s == nil || s.Kind != doc.String {
		return ""
	}
	return s.S
}

// maxFileSize bounds every configuration file jureuma reads (and writes).
const maxFileSize = 32 << 20

// MaxPointerDepth bounds the number of tokens in a pointer that set accepts,
// far below jsonc.MaxDepth, so the indented output stays small.
const MaxPointerDepth = 64

var errTooLarge = errors.New("file too large")

func readLimited(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: not a regular file", filepath.Base(path))
	}
	data, err := io.ReadAll(io.LimitReader(f, maxFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxFileSize {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), errTooLarge)
	}
	return data, nil
}
